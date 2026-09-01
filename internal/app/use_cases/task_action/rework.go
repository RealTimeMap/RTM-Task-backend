package task_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

type SendToReworkCommand struct {
	Actor  role.Actor
	TaskID uint
	Note   string
}

func (c SendToReworkCommand) Validate() error {
	if c.Note == "" {
		return apperror.NewRequiredError("note")
	}
	return nil
}

// TaskReworker — часть домена, возвращающая завершённую задачу в работу.
type TaskReworker interface {
	SendToRework(ctx context.Context, actor role.Actor, id uint, params task.ReworkParams) (*task.Task, error)
}

type SendToReworkHandler struct {
	tasks     TaskReworker
	staff     StaffReader
	publisher EventPublisher
	notifier  Notifier
	logger    *zap.Logger
}

func NewSendToReworkHandler(
	tasks TaskReworker,
	staff StaffReader,
	publisher EventPublisher,
	notifier Notifier,
	logger *zap.Logger,
) *SendToReworkHandler {
	return &SendToReworkHandler{
		tasks:     tasks,
		staff:     staff,
		publisher: publisher,
		notifier:  notifier,
		logger:    logger,
	}
}

func (h *SendToReworkHandler) Handle(ctx context.Context, cmd SendToReworkCommand) (TaskResult, error) {
	if err := cmd.Validate(); err != nil {
		return TaskResult{}, err
	}

	obj, err := h.tasks.SendToRework(ctx, cmd.Actor, cmd.TaskID, task.ReworkParams{Note: cmd.Note})
	if err != nil {
		return TaskResult{}, err
	}

	result := toTaskResult(obj)
	publish(ctx, h.publisher, TaskEvent{Name: EventTaskReworked, Task: result})

	h.notifyAssignee(ctx, obj)

	return result, nil
}

// notifyAssignee сообщает исполнителю, что задачу вернули с замечанием.
//
// Задача без исполнителя уходит в new и ждёт назначения — адресата нет.
// Уведомлять себя самого тоже незачем: тот, кто вернул задачу, и так знает.
func (h *SendToReworkHandler) notifyAssignee(ctx context.Context, obj *task.Task) {
	if h.notifier == nil || h.staff == nil || !obj.IsAssigned() {
		return
	}
	// ReworkByID проставляет домен, но обработчик не должен падать,
	// если поле пустое: уведомление важнее, чем отсечь письмо самому себе.
	if obj.ReworkByID != nil && obj.IsAssignedTo(*obj.ReworkByID) {
		return
	}

	recipient, err := h.staff.GetByID(ctx, *obj.AssigneeID)
	if err != nil {
		h.logger.Warn("notify rework skipped: staff lookup failed",
			zap.Uint("staff_id", *obj.AssigneeID),
			zap.Error(err),
		)
		return
	}

	notifyRework(ctx, h.notifier, ReworkNotice{
		Recipient: *recipient,
		Task:      *obj,
		Note:      obj.ReworkNote,
	})
}
