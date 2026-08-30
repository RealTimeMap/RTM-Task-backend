package task_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

type UpdateTaskCommand struct {
	Actor       role.Actor
	TaskID      uint
	Title       *string
	Description *string
	Type        *string
	Priority    *int
}

func (c UpdateTaskCommand) Validate() error {
	if c.Title == nil && c.Description == nil && c.Type == nil && c.Priority == nil {
		return apperror.NewValidationError(
			"body",
			"at least one field must be provided",
			"value_error.empty_payload",
			nil,
		)
	}
	return nil
}

// TaskUpdater — часть домена, нужная для обновления полей задачи.
type TaskUpdater interface {
	Update(ctx context.Context, actor role.Actor, id uint, params task.UpdateTaskParams) (*task.Task, error)
}

type UpdateTaskHandler struct {
	tasks     TaskUpdater
	publisher EventPublisher
	logger    *zap.Logger
}

func NewUpdateTaskHandler(tasks TaskUpdater, publisher EventPublisher, logger *zap.Logger) *UpdateTaskHandler {
	return &UpdateTaskHandler{tasks: tasks, publisher: publisher, logger: logger}
}

func (h *UpdateTaskHandler) Handle(ctx context.Context, cmd UpdateTaskCommand) (TaskResult, error) {
	if err := cmd.Validate(); err != nil {
		return TaskResult{}, err
	}

	params := task.UpdateTaskParams{
		Title:       cmd.Title,
		Description: cmd.Description,
	}
	if cmd.Type != nil {
		taskType := task.Type(*cmd.Type)
		params.Type = &taskType
	}
	if cmd.Priority != nil {
		priority := task.Priority(*cmd.Priority)
		params.Priority = &priority
	}

	obj, err := h.tasks.Update(ctx, cmd.Actor, cmd.TaskID, params)
	if err != nil {
		return TaskResult{}, err
	}

	result := toTaskResult(obj)
	publish(ctx, h.publisher, TaskEvent{Name: EventTaskUpdated, Task: result})

	return result, nil
}

type ChangeStatusCommand struct {
	Actor  role.Actor
	TaskID uint
	Status string
}

func (c ChangeStatusCommand) Validate() error {
	if c.Status == "" {
		return apperror.NewRequiredError("status")
	}
	return nil
}

// TaskStatusChanger — часть домена, отвечающая за жизненный цикл задачи.
type TaskStatusChanger interface {
	ChangeStatus(ctx context.Context, actor role.Actor, id uint, target task.Status) (*task.Task, error)
}

type ChangeStatusHandler struct {
	tasks     TaskStatusChanger
	publisher EventPublisher
	logger    *zap.Logger
}

func NewChangeStatusHandler(tasks TaskStatusChanger, publisher EventPublisher, logger *zap.Logger) *ChangeStatusHandler {
	return &ChangeStatusHandler{tasks: tasks, publisher: publisher, logger: logger}
}

func (h *ChangeStatusHandler) Handle(ctx context.Context, cmd ChangeStatusCommand) (TaskResult, error) {
	if err := cmd.Validate(); err != nil {
		return TaskResult{}, err
	}

	obj, err := h.tasks.ChangeStatus(ctx, cmd.Actor, cmd.TaskID, task.Status(cmd.Status))
	if err != nil {
		return TaskResult{}, err
	}

	result := toTaskResult(obj)
	publish(ctx, h.publisher, TaskEvent{Name: EventTaskStatusChanged, Task: result})

	return result, nil
}
