package task_action

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
)

// stubDiscussion — заглушка домена для обсуждения и чек-листа.
type stubDiscussion struct {
	comment *task.Comment
	item    *task.ChecklistItem
	err     error

	deleted bool
}

func (s *stubDiscussion) ListComments(_ context.Context, _ uint) ([]*task.Comment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []*task.Comment{s.comment}, nil
}

func (s *stubDiscussion) AddComment(_ context.Context, _ role.Actor, _ uint, _ string) (*task.Comment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.comment, nil
}

func (s *stubDiscussion) EditComment(_ context.Context, _ role.Actor, _, _ uint, _ string) (*task.Comment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.comment, nil
}

func (s *stubDiscussion) DeleteComment(_ context.Context, _ role.Actor, _, _ uint) error {
	s.deleted = true
	return s.err
}

func (s *stubDiscussion) ListChecklist(_ context.Context, _ uint) ([]*task.ChecklistItem, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []*task.ChecklistItem{s.item}, nil
}

func (s *stubDiscussion) AddChecklistItem(_ context.Context, _ role.Actor, _ uint, _ string) (*task.ChecklistItem, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.item, nil
}

func (s *stubDiscussion) UpdateChecklistItem(
	_ context.Context,
	_ role.Actor,
	_, _ uint,
	_ *string,
	_ *bool,
) (*task.ChecklistItem, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.item, nil
}

func (s *stubDiscussion) DeleteChecklistItem(_ context.Context, _ role.Actor, _, _ uint) error {
	s.deleted = true
	return s.err
}

func newComment(id, taskID uint, body string) *task.Comment {
	comment := &task.Comment{TaskID: taskID, AuthorID: 1, Body: body}
	comment.ID = id
	return comment
}

func newItem(id, taskID uint, title string) *task.ChecklistItem {
	item := &task.ChecklistItem{TaskID: taskID, Title: title}
	item.ID = id
	return item
}

func TestAddCommentPublishesEvent(t *testing.T) {
	events := &recordingPublisher{}
	handler := NewCommentHandler(&stubDiscussion{comment: newComment(5, 3, "текст")}, events, zap.NewNop())

	result, err := handler.Add(context.Background(), AddCommentCommand{TaskID: 3, Body: "текст"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 5 {
		t.Fatalf("id = %d, want 5", result.ID)
	}

	if len(events.comments) != 1 || events.comments[0].Name != EventCommentAdded {
		t.Fatalf("comment events = %+v", events.comments)
	}
}

func TestAddCommentRejectsEmptyCommand(t *testing.T) {
	events := &recordingPublisher{}
	handler := NewCommentHandler(&stubDiscussion{}, events, zap.NewNop())

	if _, err := handler.Add(context.Background(), AddCommentCommand{TaskID: 3}); err == nil {
		t.Fatal("expected validation error")
	}
	if len(events.comments) != 0 {
		t.Fatal("failed command must not publish an event")
	}
}

func TestDeleteCommentPublishesIdentity(t *testing.T) {
	events := &recordingPublisher{}
	domain := &stubDiscussion{}
	handler := NewCommentHandler(domain, events, zap.NewNop())

	if err := handler.Delete(context.Background(), DeleteCommentCommand{TaskID: 3, CommentID: 5}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !domain.deleted {
		t.Fatal("domain delete was not called")
	}

	// Удалённой записи больше нет — подписчику нужны только адреса.
	if len(events.comments) != 1 {
		t.Fatalf("comment events = %d, want 1", len(events.comments))
	}
	event := events.comments[0]
	if event.Name != EventCommentDeleted || event.Comment.ID != 5 || event.Comment.TaskID != 3 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestCommentHandlerSurvivesMissingPublisher(t *testing.T) {
	handler := NewCommentHandler(&stubDiscussion{comment: newComment(5, 3, "текст")}, nil, zap.NewNop())

	// Publisher необязателен: без сокет-сервера операция всё равно
	// должна проходить, а не падать на публикации.
	if _, err := handler.Add(context.Background(), AddCommentCommand{TaskID: 3, Body: "текст"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddChecklistItemPublishesEvent(t *testing.T) {
	events := &recordingPublisher{}
	handler := NewChecklistHandler(&stubDiscussion{item: newItem(9, 3, "Сверстать")}, events, zap.NewNop())

	result, err := handler.Add(context.Background(), AddChecklistItemCommand{TaskID: 3, Title: "Сверстать"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Сверстать" || result.Done {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(events.checklists) != 1 || events.checklists[0].Name != EventChecklistAdded {
		t.Fatalf("checklist events = %+v", events.checklists)
	}
}

func TestUpdateChecklistItemRequiresAnyField(t *testing.T) {
	events := &recordingPublisher{}
	handler := NewChecklistHandler(&stubDiscussion{item: newItem(9, 3, "Сверстать")}, events, zap.NewNop())

	// Запрос без единого поля бессмысленен: домен получил бы два nil
	// и молча ничего не изменил, вернув «успех».
	_, err := handler.Update(context.Background(), UpdateChecklistItemCommand{TaskID: 3, ItemID: 9})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(events.checklists) != 0 {
		t.Fatal("failed command must not publish an event")
	}
}

func TestUpdateChecklistItemAcceptsDoneOnly(t *testing.T) {
	events := &recordingPublisher{}
	handler := NewChecklistHandler(&stubDiscussion{item: newItem(9, 3, "Сверстать")}, events, zap.NewNop())

	done := true
	if _, err := handler.Update(context.Background(), UpdateChecklistItemCommand{
		TaskID: 3,
		ItemID: 9,
		Done:   &done,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events.checklists) != 1 || events.checklists[0].Name != EventChecklistUpdated {
		t.Fatalf("checklist events = %+v", events.checklists)
	}
}

func TestDeleteChecklistItemPublishesIdentity(t *testing.T) {
	events := &recordingPublisher{}
	handler := NewChecklistHandler(&stubDiscussion{}, events, zap.NewNop())

	if err := handler.Delete(context.Background(), DeleteChecklistItemCommand{TaskID: 3, ItemID: 9}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := events.checklists[0]
	if event.Name != EventChecklistDeleted || event.Item.ID != 9 || event.Item.TaskID != 3 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestChecklistResultCarriesDoneFlag(t *testing.T) {
	item := newItem(9, 3, "Сверстать")
	item.SetDone(true, 7)

	result := toChecklistItemResult(item)
	if !result.Done || result.DoneByID == nil || *result.DoneByID != 7 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
