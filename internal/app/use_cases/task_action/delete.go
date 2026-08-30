package task_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
)

type DeleteTaskCommand struct {
	Actor  role.Actor
	TaskID uint
}

// TaskRemover — часть домена, отвечающая за удаление задачи.
// Чтение нужно, чтобы собрать событие до того, как задача исчезнет.
type TaskRemover interface {
	GetByID(ctx context.Context, id uint) (*task.Task, error)
	Delete(ctx context.Context, actor role.Actor, id uint) error
}

type DeleteTaskHandler struct {
	tasks     TaskRemover
	publisher EventPublisher
	logger    *zap.Logger
}

func NewDeleteTaskHandler(tasks TaskRemover, publisher EventPublisher, logger *zap.Logger) *DeleteTaskHandler {
	return &DeleteTaskHandler{tasks: tasks, publisher: publisher, logger: logger}
}

func (h *DeleteTaskHandler) Handle(ctx context.Context, cmd DeleteTaskCommand) error {
	// Снимок делается до удаления: после него подписчикам нечего было бы слать.
	var snapshot TaskResult
	if obj, err := h.tasks.GetByID(ctx, cmd.TaskID); err == nil {
		snapshot = toTaskResult(obj)
	}

	if err := h.tasks.Delete(ctx, cmd.Actor, cmd.TaskID); err != nil {
		return err
	}

	if snapshot.ID != 0 {
		publish(ctx, h.publisher, TaskEvent{Name: EventTaskDeleted, Task: snapshot})
	}

	return nil
}
