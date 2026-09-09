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

// TaskSummarizer отдаёт счётчики дочерних записей набора задач.
// Отдельный порт: он нужен только чтению, но не изменению задач.
type TaskSummarizer interface {
	SummaryFor(ctx context.Context, objs []*task.Task) (map[uint]task.Summary, error)
}

type GetTaskHandler struct {
	tasks     TaskGetter
	summaries TaskSummarizer
	logger    *zap.Logger
}

func NewGetTaskHandler(tasks TaskGetter, summaries TaskSummarizer, logger *zap.Logger) *GetTaskHandler {
	return &GetTaskHandler{tasks: tasks, summaries: summaries, logger: logger}
}

func (h *GetTaskHandler) Handle(ctx context.Context, query GetTaskQuery) (TaskResult, error) {
	obj, err := h.tasks.GetByID(ctx, query.TaskID)
	if err != nil {
		return TaskResult{}, err
	}

	result := toTaskResult(obj)
	// Счётчики — украшение карточки: если они не собрались, задачу всё
	// равно надо отдать, иначе сбой во второстепенном запросе прячет
	// основные данные.
	if summaries, err := h.summary(ctx, []*task.Task{obj}); err == nil {
		result = result.withSummary(summaries[obj.ID])
	}
	return result, nil
}

// summary достаёт счётчики, если порт подключён.
func (h *GetTaskHandler) summary(ctx context.Context, objs []*task.Task) (map[uint]task.Summary, error) {
	if h.summaries == nil {
		return map[uint]task.Summary{}, nil
	}
	return h.summaries.SummaryFor(ctx, objs)
}

type ListTasksQuery struct {
	Status         *string
	Type           *string
	Priority       *int
	Project        *string
	CreatorID      *uint
	AssigneeID     *uint
	OnlyUnassigned bool

	// Sort и Order задают порядок выборки. Пустые значения означают
	// порядок по умолчанию.
	Sort  string
	Order string

	Limit  int
	Offset int
}

// TaskLister — часть домена, нужная для выборки списка задач.
type TaskLister interface {
	List(ctx context.Context, filter task.Filter) ([]*task.Task, int64, error)
}

type ListTasksHandler struct {
	tasks     TaskLister
	summaries TaskSummarizer
	logger    *zap.Logger
}

func NewListTasksHandler(tasks TaskLister, summaries TaskSummarizer, logger *zap.Logger) *ListTasksHandler {
	return &ListTasksHandler{tasks: tasks, summaries: summaries, logger: logger}
}

func (h *ListTasksHandler) Handle(ctx context.Context, query ListTasksQuery) (TaskListResult, error) {
	params := pagination.New(query.Limit, query.Offset)

	filter := task.Filter{
		CreatorID:      query.CreatorID,
		AssigneeID:     query.AssigneeID,
		OnlyUnassigned: query.OnlyUnassigned,
		Sort: task.Sort{
			Field: task.SortField(query.Sort),
			Order: task.SortOrder(query.Order),
		},
		Pagination: params,
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
	if query.Project != nil {
		project := task.Project(*query.Project)
		filter.Project = &project
	}

	objs, total, err := h.tasks.List(ctx, filter)
	if err != nil {
		return TaskListResult{}, err
	}

	items := toTaskResults(objs)
	if h.summaries != nil {
		summaries, err := h.summaries.SummaryFor(ctx, objs)
		if err != nil {
			// Список задач важнее счётчиков: отдаём его без сводки.
			h.logger.Warn("task summaries skipped", zap.Error(err))
		} else {
			for i := range items {
				items[i] = items[i].withSummary(summaries[items[i].ID])
			}
		}
	}

	return TaskListResult{
		Items:  items,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}
