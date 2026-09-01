package task_action

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
)

// stubReworker возвращает задачу в том виде, в каком её отдал бы домен.
type stubReworker struct {
	result *task.Task
	err    error
}

func (s *stubReworker) SendToRework(
	_ context.Context, _ role.Actor, _ uint, _ task.ReworkParams,
) (*task.Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func reworkedTask(assignee *uint, author *uint) *task.Task {
	now := time.Now()
	obj := &task.Task{
		Status:     task.WorkingStatus,
		AssigneeID: assignee,
		ReworkNote: "Поправить обработку пустого списка",
		ReworkByID: author,
		ReworkAt:   &now,
	}
	obj.ID = 19
	return obj
}

func uintPtr(value uint) *uint { return &value }

func TestSendToReworkPublishesEvent(t *testing.T) {
	events := &recordingPublisher{}
	handler := NewSendToReworkHandler(
		&stubReworker{result: reworkedTask(uintPtr(2), uintPtr(3))},
		&stubStaff{}, events, &recordingNotifier{}, zap.NewNop(),
	)

	result, err := handler.Handle(context.Background(), SendToReworkCommand{
		TaskID: 19, Note: "Поправить обработку пустого списка",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ReworkNote == "" {
		t.Fatal("result must carry the rework note")
	}
	if len(events.events) != 1 || events.events[0].Name != EventTaskReworked {
		t.Fatalf("expected one %q event, got %+v", EventTaskReworked, events.events)
	}
}

func TestSendToReworkRequiresNote(t *testing.T) {
	handler := NewSendToReworkHandler(
		&stubReworker{result: reworkedTask(uintPtr(2), uintPtr(3))},
		&stubStaff{}, &recordingPublisher{}, &recordingNotifier{}, zap.NewNop(),
	)

	if _, err := handler.Handle(
		context.Background(), SendToReworkCommand{TaskID: 19},
	); err == nil {
		t.Fatal("empty note must be rejected")
	}
}

func TestSendToReworkNotifiesAssignee(t *testing.T) {
	notices := &recordingNotifier{}
	handler := NewSendToReworkHandler(
		&stubReworker{result: reworkedTask(uintPtr(2), uintPtr(3))},
		&stubStaff{}, &recordingPublisher{}, notices, zap.NewNop(),
	)

	if _, err := handler.Handle(
		context.Background(), SendToReworkCommand{TaskID: 19, Note: "Доделать"},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notices.reworks) != 1 {
		t.Fatalf("expected one rework notice, got %d", len(notices.reworks))
	}
	if notices.reworks[0].Note == "" {
		t.Fatal("notice must carry the note")
	}
}

func TestSendToReworkSkipsNoticeForSelf(t *testing.T) {
	notices := &recordingNotifier{}
	// Вернул задачу тот же, кто её исполняет — письмо самому себе не нужно.
	handler := NewSendToReworkHandler(
		&stubReworker{result: reworkedTask(uintPtr(2), uintPtr(2))},
		&stubStaff{}, &recordingPublisher{}, notices, zap.NewNop(),
	)

	if _, err := handler.Handle(
		context.Background(), SendToReworkCommand{TaskID: 19, Note: "Доделать"},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notices.reworks) != 0 {
		t.Fatal("must not notify the author of the rework")
	}
}

func TestSendToReworkWithoutAssigneeSendsNoNotice(t *testing.T) {
	notices := &recordingNotifier{}
	handler := NewSendToReworkHandler(
		&stubReworker{result: reworkedTask(nil, uintPtr(3))},
		&stubStaff{}, &recordingPublisher{}, notices, zap.NewNop(),
	)

	if _, err := handler.Handle(
		context.Background(), SendToReworkCommand{TaskID: 19, Note: "Доделать"},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notices.reworks) != 0 {
		t.Fatal("task without an assignee has nobody to notify")
	}
}

// Регрессия: ReworkByID без значения ронял обработчик по nil pointer.
func TestSendToReworkSurvivesMissingAuthor(t *testing.T) {
	notices := &recordingNotifier{}
	handler := NewSendToReworkHandler(
		&stubReworker{result: reworkedTask(uintPtr(2), nil)},
		&stubStaff{}, &recordingPublisher{}, notices, zap.NewNop(),
	)

	if _, err := handler.Handle(
		context.Background(), SendToReworkCommand{TaskID: 19, Note: "Доделать"},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notices.reworks) != 1 {
		t.Fatal("notice must still be sent when the author is unknown")
	}
}
