package task_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/pagination"
)

type GetTaskQuery struct {
	TaskID uint
}

// TaskGetter — часть домена, нужная для чтения одной задачи.
type TaskGetter interface {
	GetByID(ctx context.Context, id uint) (*task.Task, error)
}

type GetTaskHandler struct {
	tasks  TaskGetter
	logger *zap.Logger
}

func NewGetTaskHandler(tasks TaskGetter, logger *zap.Logger) *GetTaskHandler {
	return &GetTaskHandler{tasks: tasks, logger: logger}
}

func (h *GetTaskHandler) Handle(ctx context.Context, query GetTaskQuery) (TaskResult, error) {
	obj, err := h.tasks.GetByID(ctx, query.TaskID)
	if err != nil {
		return TaskResult{}, err
	}
	return toTaskResult(obj), nil
}

type ListTasksQuery struct {
	Status         *string
	Type           *string
	Priority       *int
	CreatorID      *uint
	AssigneeID     *uint
	OnlyUnassigned bool
	Limit          int
	Offset         int
}

// TaskLister — часть домена, нужная для выборки списка задач.
type TaskLister interface {
	List(ctx context.Context, filter task.Filter) ([]*task.Task, int64, error)
}

type ListTasksHandler struct {
	tasks  TaskLister
	logger *zap.Logger
}

func NewListTasksHandler(tasks TaskLister, logger *zap.Logger) *ListTasksHandler {
	return &ListTasksHandler{tasks: tasks, logger: logger}
}

func (h *ListTasksHandler) Handle(ctx context.Context, query ListTasksQuery) (TaskListResult, error) {
	params := pagination.New(query.Limit, query.Offset)

	filter := task.Filter{
		CreatorID:      query.CreatorID,
		AssigneeID:     query.AssigneeID,
		OnlyUnassigned: query.OnlyUnassigned,
		Pagination:     params,
	}
	if query.Status != nil {
		status := task.Status(*query.Status)
		filter.Status = &status
	}
	if query.Type != nil {
		taskType := task.Type(*query.Type)
		filter.Type = &taskType
	}
	if query.Priority != nil {
		priority := task.Priority(*query.Priority)
		filter.Priority = &priority
	}

	objs, total, err := h.tasks.List(ctx, filter)
	if err != nil {
		return TaskListResult{}, err
	}

	return TaskListResult{
		Items:  toTaskResults(objs),
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}
