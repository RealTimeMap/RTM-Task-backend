package task_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

type CreateTaskCommand struct {
	Actor       role.Actor
	Title       string
	Description string
	Type        string
	Priority    int
	AssigneeID  *uint
}

func (c CreateTaskCommand) Validate() error {
	if c.Title == "" {
		return apperror.NewRequiredError("title")
	}
	if c.Type == "" {
		return apperror.NewRequiredError("type")
	}
	return nil
}

// TaskCreator — часть домена, нужная этому use case'у.
type TaskCreator interface {
	Create(ctx context.Context, actor role.Actor, params task.CreateTaskParams) (*task.Task, error)
}

type CreateTaskHandler struct {
	tasks     TaskCreator
	staff     StaffReader
	publisher EventPublisher
	notifier  Notifier
	logger    *zap.Logger
}

func NewCreateTaskHandler(
	tasks TaskCreator,
	staff StaffReader,
	publisher EventPublisher,
	notifier Notifier,
	logger *zap.Logger,
) *CreateTaskHandler {
	return &CreateTaskHandler{
		tasks:     tasks,
		staff:     staff,
		publisher: publisher,
		notifier:  notifier,
		logger:    logger,
	}
}

func (h *CreateTaskHandler) Handle(ctx context.Context, cmd CreateTaskCommand) (TaskResult, error) {
	if err := cmd.Validate(); err != nil {
		return TaskResult{}, err
	}

	obj, err := h.tasks.Create(ctx, cmd.Actor, task.CreateTaskParams{
		Title:       cmd.Title,
		Description: cmd.Description,
		Type:        task.Type(cmd.Type),
		Priority:    task.Priority(cmd.Priority),
		AssigneeID:  cmd.AssigneeID,
	})
	if err != nil {
		return TaskResult{}, err
	}

	result := toTaskResult(obj)
	publish(ctx, h.publisher, TaskEvent{Name: EventTaskCreated, Task: result})

	// Задачу могли сразу завести на исполнителя — для него это то же
	// событие «на вас назначена задача», что и при отдельном назначении.
	if obj.AssigneeID != nil {
		h.notifyAssignee(ctx, obj, *obj.AssigneeID)
	}

	return result, nil
}

// notifyAssignee сообщает исполнителю о назначении при создании задачи.
func (h *CreateTaskHandler) notifyAssignee(ctx context.Context, obj *task.Task, assigneeID uint) {
	if h.notifier == nil || h.staff == nil {
		return
	}

	recipient, err := h.staff.GetByID(ctx, assigneeID)
	if err != nil {
		h.logger.Warn("notify assignment skipped: staff lookup failed",
			zap.Uint("staff_id", assigneeID),
			zap.Error(err),
		)
		return
	}

	notifyAssignment(ctx, h.notifier, AssignmentNotice{
		Recipient: *recipient,
		Task:      *obj,
	})
}
