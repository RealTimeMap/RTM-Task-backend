package task_action

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
)

// statusChanger — заглушка домена для смены статуса.
type statusChanger struct {
	stored *task.Task
}

func (s *statusChanger) ChangeStatus(
	_ context.Context, _ role.Actor, _ uint, target task.Status,
) (*task.Task, error) {
	updated := *s.stored
	updated.Status = target
	return &updated, nil
}

// summarizingTasks отдаёт сводку — так ведёт себя доменный сервис там,
// где задача читается вместе со счётчиками.
type summarizingTasks struct {
	summary task.Summary
}

func (s *summarizingTasks) SummaryFor(
	_ context.Context, objs []*task.Task,
) (map[uint]task.Summary, error) {
	result := make(map[uint]task.Summary, len(objs))
	for _, obj := range objs {
		result[obj.ID] = s.summary
	}
	return result, nil
}

func taskWithChecklist() *task.Task {
	obj := &task.Task{
		Title:      "Обновить сервис комментариев",
		Type:       task.FeatureType,
		Status:     task.WorkingStatus,
		Priority:   task.HighPriority,
		Project:    task.TaskProject,
		CreatorID:  1,
		AssigneeID: ptrUint(1),
		Version:    1,
	}
	obj.ID = 25
	return obj
}

func ptrUint(v uint) *uint { return &v }

// Смена статуса сводки не несёт, и счётчики должны остаться пустыми,
// а не превратиться в нули.
//
// Ноль здесь означал бы «записей нет»: карточка на доске погасила бы
// прогресс чек-листа и число реплик при каждом перетаскивании задачи
// между колонками — ровно тот баг, ради которого счётчики стали
// указателями.
func TestChangeStatusLeavesSummaryUnknown(t *testing.T) {
	handler := NewChangeStatusHandler(
		&statusChanger{stored: taskWithChecklist()},
		&recordingPublisher{},
		zap.NewNop(),
	)

	result, err := handler.Handle(context.Background(), ChangeStatusCommand{
		Actor:  role.Actor{StaffID: 1, Role: role.ManagerRole},
		TaskID: 25,
		Status: task.ReviewStatus.String(),
	})
	if err != nil {
		t.Fatalf("change status: %v", err)
	}

	if result.ChecklistTotal != nil {
		t.Fatalf("checklistTotal = %d, want nil — сводки у смены статуса нет", *result.ChecklistTotal)
	}
	if result.ChecklistDone != nil {
		t.Fatalf("checklistDone = %d, want nil", *result.ChecklistDone)
	}
	if result.CommentCount != nil {
		t.Fatalf("commentCount = %d, want nil", *result.CommentCount)
	}
}

// Там, где сводка есть, она доезжает до результата.
func TestWithSummaryFillsCounters(t *testing.T) {
	result := toTaskResult(taskWithChecklist()).withSummary(task.Summary{
		ChecklistTotal: 5,
		ChecklistDone:  2,
		CommentCount:   6,
	})

	if result.ChecklistTotal == nil || *result.ChecklistTotal != 5 {
		t.Fatalf("checklistTotal = %v, want 5", result.ChecklistTotal)
	}
	if result.ChecklistDone == nil || *result.ChecklistDone != 2 {
		t.Fatalf("checklistDone = %v, want 2", result.ChecklistDone)
	}
	if result.CommentCount == nil || *result.CommentCount != 6 {
		t.Fatalf("commentCount = %v, want 6", result.CommentCount)
	}
}

// Пустая сводка — это именно нули, а не «не знаем»: у задачи без
// чек-листа и обсуждения счётчики должны приходить нулями, иначе
// карточка показывала бы прогресс от прошлой задачи.
func TestWithSummaryKeepsExplicitZeroes(t *testing.T) {
	result := toTaskResult(taskWithChecklist()).withSummary(task.Summary{})

	if result.ChecklistTotal == nil || *result.ChecklistTotal != 0 {
		t.Fatalf("checklistTotal = %v, want explicit 0", result.ChecklistTotal)
	}
	if result.CommentCount == nil || *result.CommentCount != 0 {
		t.Fatalf("commentCount = %v, want explicit 0", result.CommentCount)
	}
}

// Чтение задачи сводку несёт: карточка, открытая напрямую, должна
// показывать прогресс сразу.
func TestGetTaskCarriesSummary(t *testing.T) {
	handler := NewGetTaskHandler(
		&stubTasks{stored: taskWithChecklist()},
		&summarizingTasks{summary: task.Summary{ChecklistTotal: 5, ChecklistDone: 2, CommentCount: 6}},
		zap.NewNop(),
	)

	result, err := handler.Handle(context.Background(), GetTaskQuery{TaskID: 25})
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	if result.ChecklistTotal == nil || *result.ChecklistTotal != 5 {
		t.Fatalf("checklistTotal = %v, want 5", result.ChecklistTotal)
	}
	if result.CommentCount == nil || *result.CommentCount != 6 {
		t.Fatalf("commentCount = %v, want 6", result.CommentCount)
	}
}
