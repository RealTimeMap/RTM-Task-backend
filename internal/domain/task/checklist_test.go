package task

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"RTM-Task/internal/utils/apperror"
)

func TestCreateTaskWithChecklist(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)

	obj, err := svc.Create(context.Background(), developerActor(1), CreateTaskParams{
		Title:     "Задача с планом",
		Type:      FeatureType,
		Checklist: []string{"Сверстать", "  ", "Подключить API", ""},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items, err := svc.ListChecklist(context.Background(), obj.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Пустые строки формы отбрасываются, порядок сохраняется.
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].Title != "Сверстать" || items[1].Title != "Подключить API" {
		t.Fatalf("unexpected items: %v, %v", items[0].Title, items[1].Title)
	}
	if items[0].Position != 0 || items[1].Position != 1 {
		t.Fatalf("positions = %d, %d", items[0].Position, items[1].Position)
	}
}

func TestCreateTaskRejectsOversizedChecklist(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)

	titles := make([]string, maxChecklistItems+1)
	for i := range titles {
		titles[i] = fmt.Sprintf("Пункт %d", i)
	}

	_, err := svc.Create(context.Background(), developerActor(1), CreateTaskParams{
		Title:     "Слишком большой план",
		Type:      FeatureType,
		Checklist: titles,
	})
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindConflict {
		t.Fatalf("err = %v, want conflict error", err)
	}
}

func TestAddChecklistItemAppendsToEnd(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	first, err := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Первый")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Второй")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.Position != 0 || second.Position != 1 {
		t.Fatalf("positions = %d, %d", first.Position, second.Position)
	}
}

func TestAddChecklistItemRejectsEmptyTitle(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	_, err := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "   ")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestAddChecklistItemRejectsTooLongTitle(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	_, err := svc.AddChecklistItem(
		context.Background(),
		developerActor(1),
		obj.ID,
		strings.Repeat("я", maxChecklistTitle+1),
	)
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestAddChecklistItemForbiddenForOutsider(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	_, err := svc.AddChecklistItem(context.Background(), developerActor(99), obj.ID, "Чужой пункт")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindForbidden {
		t.Fatalf("err = %v, want forbidden error", err)
	}
}

func TestChecklistLimitOnAdd(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	for i := 0; i < maxChecklistItems; i++ {
		if _, err := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, fmt.Sprintf("Пункт %d", i)); err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}

	_, err := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Лишний")
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindConflict {
		t.Fatalf("err = %v, want conflict error", err)
	}
}

func TestMarkChecklistItemDone(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	item, _ := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Сверстать")

	done := true
	updated, err := svc.UpdateChecklistItem(context.Background(), developerActor(1), obj.ID, item.ID, nil, &done)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.IsDone() {
		t.Fatal("item must be done")
	}
	if updated.DoneByID == nil || *updated.DoneByID != 1 {
		t.Fatalf("doneById = %v, want 1", updated.DoneByID)
	}
}

func TestUnmarkChecklistItemClearsAuthor(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	item, _ := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Сверстать")

	done := true
	if _, err := svc.UpdateChecklistItem(context.Background(), developerActor(1), obj.ID, item.ID, nil, &done); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done = false
	updated, err := svc.UpdateChecklistItem(context.Background(), developerActor(1), obj.ID, item.ID, nil, &done)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Снятая отметка не должна оставлять «выполнил такой-то»:
	// иначе в интерфейсе висел бы автор у незакрытого пункта.
	if updated.IsDone() || updated.DoneByID != nil {
		t.Fatalf("item must be clean: done=%v by=%v", updated.IsDone(), updated.DoneByID)
	}
}

func TestChecklistDoesNotBlockCompletion(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	assignee := uint(1)
	obj := seedTask(svc, 1, &assignee)

	if _, err := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Не сделано"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Незакрытый пункт не мешает провести задачу до завершения:
	// чек-лист — подсказка исполнителю, а не условие приёмки.
	for _, status := range []Status{WorkingStatus, ReviewStatus, CompleteStatus} {
		updated, err := svc.ChangeStatus(context.Background(), developerActor(1), obj.ID, status)
		if err != nil {
			t.Fatalf("transition to %s failed: %v", status, err)
		}
		obj = updated
	}

	if obj.Status != CompleteStatus {
		t.Fatalf("status = %s, want complete", obj.Status)
	}
}

func TestRenameChecklistItem(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	item, _ := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Старое")

	title := "Новое"
	updated, err := svc.UpdateChecklistItem(context.Background(), developerActor(1), obj.ID, item.ID, &title, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Title != "Новое" {
		t.Fatalf("title = %q", updated.Title)
	}
}

func TestChecklistItemFromForeignTaskNotFound(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	first := seedTask(svc, 1, nil)
	second := seedTask(svc, 1, nil)

	item, _ := svc.AddChecklistItem(context.Background(), developerActor(1), first.ID, "Пункт первой задачи")

	err := svc.DeleteChecklistItem(context.Background(), developerActor(1), second.ID, item.ID)
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindNotFound {
		t.Fatalf("err = %v, want not found error", err)
	}
}

func TestSummaryCountsChildren(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	obj := seedTask(svc, 1, nil)

	first, _ := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Первый")
	if _, err := svc.AddChecklistItem(context.Background(), developerActor(1), obj.ID, "Второй"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.AddComment(context.Background(), developerActor(1), obj.ID, "реплика"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done := true
	if _, err := svc.UpdateChecklistItem(context.Background(), developerActor(1), obj.ID, first.ID, nil, &done); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	summaries, err := svc.SummaryFor(context.Background(), []*Task{obj})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	summary := summaries[obj.ID]
	if summary.ChecklistTotal != 2 || summary.ChecklistDone != 1 || summary.CommentCount != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestSummaryForEmptyInput(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)

	summaries, err := svc.SummaryFor(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("summaries = %d, want 0", len(summaries))
	}
}
