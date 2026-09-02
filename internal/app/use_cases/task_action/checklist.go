package task_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

// TaskChecklister — часть домена, обслуживающая чек-лист задачи.
type TaskChecklister interface {
	ListChecklist(ctx context.Context, taskID uint) ([]*task.ChecklistItem, error)
	AddChecklistItem(ctx context.Context, actor role.Actor, taskID uint, title string) (*task.ChecklistItem, error)
	UpdateChecklistItem(
		ctx context.Context,
		actor role.Actor,
		taskID, itemID uint,
		title *string,
		done *bool,
	) (*task.ChecklistItem, error)
	DeleteChecklistItem(ctx context.Context, actor role.Actor, taskID, itemID uint) error
}

type ListChecklistQuery struct {
	TaskID uint
}

type AddChecklistItemCommand struct {
	Actor  role.Actor
	TaskID uint
	Title  string
}

func (c AddChecklistItemCommand) Validate() error {
	if c.Title == "" {
		return apperror.NewRequiredError("title")
	}
	return nil
}

// UpdateChecklistItemCommand — частичное изменение пункта:
// nil-поле означает «не менять».
type UpdateChecklistItemCommand struct {
	Actor  role.Actor
	TaskID uint
	ItemID uint
	Title  *string
	Done   *bool
}

func (c UpdateChecklistItemCommand) Validate() error {
	if c.Title == nil && c.Done == nil {
		return apperror.NewValidationError(
			"body",
			"at least one of title or done must be provided",
			"value_error.missing",
			nil,
		)
	}
	return nil
}

type DeleteChecklistItemCommand struct {
	Actor  role.Actor
	TaskID uint
	ItemID uint
}

// ChecklistHandler обслуживает чек-лист задачи целиком.
type ChecklistHandler struct {
	tasks     TaskChecklister
	publisher EventPublisher
	logger    *zap.Logger
}

func NewChecklistHandler(tasks TaskChecklister, publisher EventPublisher, logger *zap.Logger) *ChecklistHandler {
	return &ChecklistHandler{tasks: tasks, publisher: publisher, logger: logger}
}

func (h *ChecklistHandler) List(ctx context.Context, query ListChecklistQuery) ([]ChecklistItemResult, error) {
	objs, err := h.tasks.ListChecklist(ctx, query.TaskID)
	if err != nil {
		return nil, err
	}
	return toChecklistItemResults(objs), nil
}

func (h *ChecklistHandler) Add(ctx context.Context, cmd AddChecklistItemCommand) (ChecklistItemResult, error) {
	if err := cmd.Validate(); err != nil {
		return ChecklistItemResult{}, err
	}

	obj, err := h.tasks.AddChecklistItem(ctx, cmd.Actor, cmd.TaskID, cmd.Title)
	if err != nil {
		return ChecklistItemResult{}, err
	}

	result := toChecklistItemResult(obj)
	publishChecklist(ctx, h.publisher, ChecklistEvent{Name: EventChecklistAdded, Item: result})
	return result, nil
}

func (h *ChecklistHandler) Update(ctx context.Context, cmd UpdateChecklistItemCommand) (ChecklistItemResult, error) {
	if err := cmd.Validate(); err != nil {
		return ChecklistItemResult{}, err
	}

	obj, err := h.tasks.UpdateChecklistItem(ctx, cmd.Actor, cmd.TaskID, cmd.ItemID, cmd.Title, cmd.Done)
	if err != nil {
		return ChecklistItemResult{}, err
	}

	result := toChecklistItemResult(obj)
	publishChecklist(ctx, h.publisher, ChecklistEvent{Name: EventChecklistUpdated, Item: result})
	return result, nil
}

func (h *ChecklistHandler) Delete(ctx context.Context, cmd DeleteChecklistItemCommand) error {
	if err := h.tasks.DeleteChecklistItem(ctx, cmd.Actor, cmd.TaskID, cmd.ItemID); err != nil {
		return err
	}

	publishChecklist(ctx, h.publisher, ChecklistEvent{
		Name: EventChecklistDeleted,
		Item: ChecklistItemResult{ID: cmd.ItemID, TaskID: cmd.TaskID},
	})
	return nil
}
