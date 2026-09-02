package task

import (
	"context"
	"strings"
	"testing"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
)

// managerActor и authorActor — типовые участники для проверок доступа.
func managerActor(id uint) role.Actor {
	return role.Actor{StaffID: id, Role: role.ManagerRole}
}

func developerActor(id uint) role.Actor {
	return role.Actor{StaffID: id, Role: role.DeveloperRole}
}

// seedTask кладёт задачу в хранилище и возвращает её.
func seedTask(svc testService, creatorID uint, assigneeID *uint) *Task {
	obj := &Task{
		Title:      "Задача",
		Type:       BugType,
		Status:     NewStatus,
		Priority:   HighPriority,
		CreatorID:  creatorID,
		AssigneeID: assigneeID,
		Version:    1,
	}
	return svc.repo.put(obj)
}

func TestAddCommentStoresBody(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	comment, err := svc.AddComment(context.Background(), developerActor(1), obj.ID, "  Проверил, работает  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Пробелы по краям срезаются: иначе в ленте появлялись бы реплики,
	// визуально пустые, но проходящие проверку на непустоту.
	if comment.Body != "Проверил, работает" {
		t.Fatalf("body = %q, want trimmed", comment.Body)
	}
	if comment.AuthorID != 1 {
		t.Fatalf("authorId = %d, want 1", comment.AuthorID)
	}
	if comment.IsEdited() {
		t.Fatal("new comment must not be marked as edited")
	}
}

func TestAddCommentRejectsEmptyBody(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	_, err := svc.AddComment(context.Background(), developerActor(1), obj.ID, "   ")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestAddCommentRejectsTooLongBody(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	_, err := svc.AddComment(context.Background(), developerActor(1), obj.ID, strings.Repeat("я", maxCommentSize+1))
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestAddCommentForbiddenForOutsider(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	// Разработчик, не связанный с задачей, комментировать её не может.
	_, err := svc.AddComment(context.Background(), developerActor(99), obj.ID, "мимо проходил")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindForbidden {
		t.Fatalf("err = %v, want forbidden error", err)
	}
}

func TestAddCommentAllowedForAssignee(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	assignee := uint(7)
	obj := seedTask(svc, 1, &assignee)

	if _, err := svc.AddComment(context.Background(), developerActor(7), obj.ID, "взял в работу"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddCommentAllowedOnClosedTask(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)
	obj.Status = CompleteStatus
	svc.repo.put(obj)

	// Завершённую задачу редактировать нельзя, а обсуждать — можно:
	// вопросы к результату возникают именно после сдачи.
	if _, err := svc.AddComment(context.Background(), developerActor(1), obj.ID, "а как это работает?"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditCommentMarksEdited(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	created, err := svc.AddComment(context.Background(), developerActor(1), obj.ID, "первый вариант")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := svc.EditComment(context.Background(), developerActor(1), obj.ID, created.ID, "второй вариант")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Body != "второй вариант" {
		t.Fatalf("body = %q", updated.Body)
	}
	if !updated.IsEdited() {
		t.Fatal("edited comment must carry editedAt")
	}
}

func TestEditCommentKeepsMarkWhenTextUnchanged(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	created, _ := svc.AddComment(context.Background(), developerActor(1), obj.ID, "текст")

	updated, err := svc.EditComment(context.Background(), developerActor(1), obj.ID, created.ID, "текст")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Отправка того же текста правкой не считается — иначе пометка
	// «изменено» появлялась бы от случайного повторного сохранения.
	if updated.IsEdited() {
		t.Fatal("unchanged text must not mark comment as edited")
	}
}

func TestEditForeignCommentForbidden(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	assignee := uint(7)
	obj := seedTask(svc, 1, &assignee)

	created, _ := svc.AddComment(context.Background(), developerActor(1), obj.ID, "текст автора")

	// Исполнитель имеет доступ к задаче, но не к чужому тексту.
	_, err := svc.EditComment(context.Background(), developerActor(7), obj.ID, created.ID, "подмена")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindForbidden {
		t.Fatalf("err = %v, want forbidden error", err)
	}
}

func TestManagerEditsForeignComment(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	created, _ := svc.AddComment(context.Background(), developerActor(1), obj.ID, "текст автора")

	if _, err := svc.EditComment(context.Background(), managerActor(50), obj.ID, created.ID, "поправлено"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommentFromForeignTaskNotFound(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	first := seedTask(svc, 1, nil)
	second := seedTask(svc, 1, nil)

	created, _ := svc.AddComment(context.Background(), developerActor(1), first.ID, "к первой задаче")

	// Идентификатор из другой задачи не должен попадать в чужую ленту.
	err := svc.DeleteComment(context.Background(), developerActor(1), second.ID, created.ID)
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindNotFound {
		t.Fatalf("err = %v, want not found error", err)
	}
}

func TestDeleteCommentRemovesIt(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	created, _ := svc.AddComment(context.Background(), developerActor(1), obj.ID, "лишний")

	if err := svc.DeleteComment(context.Background(), developerActor(1), obj.ID, created.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	comments, err := svc.ListComments(context.Background(), obj.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("comments = %d, want 0", len(comments))
	}
}

func TestListCommentsKeepsOrder(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	for _, body := range []string{"первый", "второй", "третий"} {
		if _, err := svc.AddComment(context.Background(), developerActor(1), obj.ID, body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	comments, err := svc.ListComments(context.Background(), obj.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 3 || comments[0].Body != "первый" || comments[2].Body != "третий" {
		t.Fatalf("unexpected order: %v", comments)
	}
}
