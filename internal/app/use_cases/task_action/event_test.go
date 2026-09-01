package task_action

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
)

// recordingPublisher запоминает опубликованные события.
type recordingPublisher struct {
	events []TaskEvent
}

func (p *recordingPublisher) PublishTask(_ context.Context, event TaskEvent) {
	p.events = append(p.events, event)
}

// recordingNotifier запоминает отправленные уведомления.
type recordingNotifier struct {
	notices []AssignmentNotice
	reworks []ReworkNotice
}

func (n *recordingNotifier) NotifyAssignment(_ context.Context, notice AssignmentNotice) {
	n.notices = append(n.notices, notice)
}

func (n *recordingNotifier) NotifyRework(_ context.Context, notice ReworkNotice) {
	n.reworks = append(n.reworks, notice)
}

// stubStaff отдаёт сотрудника по идентификатору.
type stubStaff struct {
	member *role.Staff
	err    error
}

func (s *stubStaff) GetByID(_ context.Context, id uint) (*role.Staff, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.member != nil {
		return s.member, nil
	}

	member := &role.Staff{FullName: "Тестовый сотрудник"}
	member.ID = id
	return member, nil
}

// Заглушки по умолчанию для тестов, которым уведомления не важны.
var (
	staffStub = &stubStaff{}
	notifier  Notifier
)

// stubTasks — заглушка домена для проверки публикации событий.
type stubTasks struct {
	stored  *task.Task
	created *task.Task
	err     error
}

func (s *stubTasks) Create(_ context.Context, _ role.Actor, _ task.CreateTaskParams) (*task.Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.created, nil
}

func (s *stubTasks) GetByID(_ context.Context, _ uint) (*task.Task, error) {
	if s.stored == nil {
		return nil, task.ErrTaskNotFound(1)
	}
	copied := *s.stored
	return &copied, nil
}

func (s *stubTasks) Assign(_ context.Context, _ role.Actor, _, assigneeID uint) (*task.Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	updated := *s.stored
	updated.AssigneeID = &assigneeID
	return &updated, nil
}

func (s *stubTasks) Unassign(_ context.Context, _ role.Actor, _ uint) (*task.Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	updated := *s.stored
	updated.AssigneeID = nil
	return &updated, nil
}

func (s *stubTasks) Delete(_ context.Context, _ role.Actor, _ uint) error {
	return s.err
}

func TestCreatePublishesEvent(t *testing.T) {
	created := &task.Task{Title: "New task", Status: task.NewStatus, Type: task.BugType}
	created.ID = 10

	publisher := &recordingPublisher{}
	handler := NewCreateTaskHandler(&stubTasks{created: created}, staffStub, publisher, notifier, zap.NewNop())

	_, err := handler.Handle(context.Background(), CreateTaskCommand{
		Title: "New task",
		Type:  "bug",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("published %d events, want 1", len(publisher.events))
	}
	if publisher.events[0].Name != EventTaskCreated {
		t.Fatalf("event = %q, want %q", publisher.events[0].Name, EventTaskCreated)
	}
	if publisher.events[0].Task.ID != 10 {
		t.Fatalf("event task id = %d, want 10", publisher.events[0].Task.ID)
	}
}

func TestCreateDoesNotPublishOnValidationError(t *testing.T) {
	publisher := &recordingPublisher{}
	handler := NewCreateTaskHandler(&stubTasks{}, staffStub, publisher, notifier, zap.NewNop())

	_, err := handler.Handle(context.Background(), CreateTaskCommand{Title: "", Type: "bug"})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if len(publisher.events) != 0 {
		t.Fatalf("published %d events, want none on failure", len(publisher.events))
	}
}

func TestAssignPublishesPreviousAssignee(t *testing.T) {
	previous := uint(1)
	stored := &task.Task{Status: task.NewStatus, AssigneeID: &previous}
	stored.ID = 5

	publisher := &recordingPublisher{}
	handler := NewAssignTaskHandler(&stubTasks{stored: stored}, staffStub, publisher, notifier, zap.NewNop())

	_, err := handler.Handle(context.Background(), AssignTaskCommand{TaskID: 5, AssigneeID: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("published %d events, want 1", len(publisher.events))
	}
	event := publisher.events[0]
	if event.Name != EventTaskAssigned {
		t.Fatalf("event = %q, want %q", event.Name, EventTaskAssigned)
	}
	if event.PreviousAssigneeID == nil || *event.PreviousAssigneeID != 1 {
		t.Fatalf("previous assignee = %v, want 1", event.PreviousAssigneeID)
	}
	if event.Task.AssigneeID == nil || *event.Task.AssigneeID != 2 {
		t.Fatalf("new assignee = %v, want 2", event.Task.AssigneeID)
	}
}

func TestUnassignPublishesPreviousAssignee(t *testing.T) {
	previous := uint(4)
	stored := &task.Task{Status: task.NewStatus, AssigneeID: &previous}
	stored.ID = 6

	publisher := &recordingPublisher{}
	handler := NewUnassignTaskHandler(&stubTasks{stored: stored}, publisher, zap.NewNop())

	_, err := handler.Handle(context.Background(), UnassignTaskCommand{TaskID: 6})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := publisher.events[0]
	if event.Name != EventTaskUnassigned {
		t.Fatalf("event = %q, want %q", event.Name, EventTaskUnassigned)
	}
	if event.PreviousAssigneeID == nil || *event.PreviousAssigneeID != 4 {
		t.Fatalf("previous assignee = %v, want 4", event.PreviousAssigneeID)
	}
}

func TestDeletePublishesSnapshotTakenBeforeRemoval(t *testing.T) {
	stored := &task.Task{Title: "Doomed", Status: task.NewStatus, Type: task.FixType}
	stored.ID = 9

	publisher := &recordingPublisher{}
	handler := NewDeleteTaskHandler(&stubTasks{stored: stored}, publisher, zap.NewNop())

	if err := handler.Handle(context.Background(), DeleteTaskCommand{TaskID: 9}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("published %d events, want 1", len(publisher.events))
	}
	event := publisher.events[0]
	if event.Name != EventTaskDeleted {
		t.Fatalf("event = %q, want %q", event.Name, EventTaskDeleted)
	}
	if event.Task.Title != "Doomed" {
		t.Fatalf("snapshot title = %q, want %q", event.Task.Title, "Doomed")
	}
}

func TestNilPublisherIsTolerated(t *testing.T) {
	created := &task.Task{Title: "New task", Status: task.NewStatus, Type: task.BugType}
	created.ID = 11

	handler := NewCreateTaskHandler(&stubTasks{created: created}, staffStub, nil, nil, zap.NewNop())

	// Push и уведомления отключены — операция всё равно должна выполниться.
	if _, err := handler.Handle(context.Background(), CreateTaskCommand{
		Title: "New task",
		Type:  "bug",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssignNotifiesNewAssignee(t *testing.T) {
	previous := uint(1)
	stored := &task.Task{Title: "Починить фильтр", Status: task.NewStatus, AssigneeID: &previous}
	stored.ID = 5

	notices := &recordingNotifier{}
	handler := NewAssignTaskHandler(
		&stubTasks{stored: stored}, staffStub, &recordingPublisher{}, notices, zap.NewNop(),
	)

	if _, err := handler.Handle(
		context.Background(), AssignTaskCommand{TaskID: 5, AssigneeID: 2},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notices.notices) != 1 {
		t.Fatalf("отправлено %d уведомлений, ожидали 1", len(notices.notices))
	}

	notice := notices.notices[0]
	if notice.Recipient.ID != 2 {
		t.Fatalf("получатель = %d, ожидали 2", notice.Recipient.ID)
	}
	if notice.Task.ID != 5 {
		t.Fatalf("задача = %d, ожидали 5", notice.Task.ID)
	}
	// В письме должен быть новый исполнитель, а не прежний.
	if notice.Task.AssigneeID == nil || *notice.Task.AssigneeID != 2 {
		t.Fatalf("исполнитель в уведомлении = %v, ожидали 2", notice.Task.AssigneeID)
	}
}

func TestCreateNotifiesOnlyWhenAssigneeSet(t *testing.T) {
	unassigned := &task.Task{Title: "Ничья", Status: task.NewStatus, Type: task.BugType}
	unassigned.ID = 12

	notices := &recordingNotifier{}
	handler := NewCreateTaskHandler(
		&stubTasks{created: unassigned}, staffStub, &recordingPublisher{}, notices, zap.NewNop(),
	)

	if _, err := handler.Handle(
		context.Background(), CreateTaskCommand{Title: "Ничья", Type: "bug"},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notices.notices) != 0 {
		t.Fatalf("уведомление ушло без исполнителя: %d", len(notices.notices))
	}

	// Та же задача, но с исполнителем — письмо должно уйти.
	assignee := uint(3)
	assigned := &task.Task{Title: "На исполнителя", Status: task.NewStatus, AssigneeID: &assignee}
	assigned.ID = 13

	withAssignee := NewCreateTaskHandler(
		&stubTasks{created: assigned}, staffStub, &recordingPublisher{}, notices, zap.NewNop(),
	)

	if _, err := withAssignee.Handle(
		context.Background(), CreateTaskCommand{Title: "На исполнителя", Type: "bug"},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notices.notices) != 1 {
		t.Fatalf("отправлено %d уведомлений, ожидали 1", len(notices.notices))
	}
	if notices.notices[0].Recipient.ID != 3 {
		t.Fatalf("получатель = %d, ожидали 3", notices.notices[0].Recipient.ID)
	}
}

func TestNotifyFailureDoesNotBreakAssignment(t *testing.T) {
	stored := &task.Task{Title: "Задача", Status: task.NewStatus}
	stored.ID = 7

	// Сотрудник не найден — уведомить некого, но назначение состояться должно.
	failing := &stubStaff{err: role.ErrStaffNotFound(2)}
	notices := &recordingNotifier{}

	handler := NewAssignTaskHandler(
		&stubTasks{stored: stored}, failing, &recordingPublisher{}, notices, zap.NewNop(),
	)

	result, err := handler.Handle(
		context.Background(), AssignTaskCommand{TaskID: 7, AssigneeID: 2},
	)
	if err != nil {
		t.Fatalf("назначение сорвалось из-за уведомления: %v", err)
	}
	if result.AssigneeID == nil || *result.AssigneeID != 2 {
		t.Fatalf("исполнитель = %v, ожидали 2", result.AssigneeID)
	}
	if len(notices.notices) != 0 {
		t.Fatalf("уведомление ушло, хотя сотрудник не найден: %d", len(notices.notices))
	}
}
