package task_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

type AssignTaskCommand struct {
	Actor      role.Actor
	TaskID     uint
	AssigneeID uint
}

func (c AssignTaskCommand) Validate() error {
	if c.AssigneeID == 0 {
		return apperror.NewRequiredError("assigneeId")
	}
	return nil
}

// TaskAssigner — часть домена, отвечающая за назначение исполнителя.
// Чтение задачи нужно, чтобы запомнить прошлого исполнителя до изменения.
type TaskAssigner interface {
	GetByID(ctx context.Context, id uint) (*task.Task, error)
	Assign(ctx context.Context, actor role.Actor, id, assigneeID uint) (*task.Task, error)
}

// StaffReader — часть домена сотрудников, нужная для уведомлений:
// адресата письма надо где-то взять.
type StaffReader interface {
	GetByID(ctx context.Context, id uint) (*role.Staff, error)
}

type AssignTaskHandler struct {
	tasks     TaskAssigner
	staff     StaffReader
	publisher EventPublisher
	notifier  Notifier
	logger    *zap.Logger
}

func NewAssignTaskHandler(
	tasks TaskAssigner,
	staff StaffReader,
	publisher EventPublisher,
	notifier Notifier,
	logger *zap.Logger,
) *AssignTaskHandler {
	return &AssignTaskHandler{
		tasks:     tasks,
		staff:     staff,
		publisher: publisher,
		notifier:  notifier,
		logger:    logger,
	}
}

func (h *AssignTaskHandler) Handle(ctx context.Context, cmd AssignTaskCommand) (TaskResult, error) {
	if err := cmd.Validate(); err != nil {
		return TaskResult{}, err
	}

	previous := h.previousAssignee(ctx, cmd.TaskID)

	obj, err := h.tasks.Assign(ctx, cmd.Actor, cmd.TaskID, cmd.AssigneeID)
	if err != nil {
		return TaskResult{}, err
	}

	result := toTaskResult(obj)
	publish(ctx, h.publisher, TaskEvent{
		Name:               EventTaskAssigned,
		Task:               result,
		PreviousAssigneeID: previous,
	})

	h.notifyAssignee(ctx, obj, cmd.AssigneeID)

	return result, nil
}

// notifyAssignee сообщает новому исполнителю о назначении.
//
// Уведомление о том, что задачу переназначили на тебя же, смысла не имеет,
// поэтому повторное назначение пропускается. Ошибка чтения сотрудника тоже
// не критична — задача уже назначена.
func (h *AssignTaskHandler) notifyAssignee(ctx context.Context, obj *task.Task, assigneeID uint) {
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

// previousAssignee читает текущего исполнителя до изменения.
// Ошибка чтения здесь не критична: назначение всё равно будет выполнено,
// просто прошлый исполнитель не получит уведомления.
func (h *AssignTaskHandler) previousAssignee(ctx context.Context, taskID uint) *uint {
	obj, err := h.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil
	}
	return obj.AssigneeID
}

type UnassignTaskCommand struct {
	Actor  role.Actor
	TaskID uint
}

// TaskUnassigner — часть домена, снимающая исполнителя с задачи.
type TaskUnassigner interface {
	GetByID(ctx context.Context, id uint) (*task.Task, error)
	Unassign(ctx context.Context, actor role.Actor, id uint) (*task.Task, error)
}

type UnassignTaskHandler struct {
	tasks     TaskUnassigner
	publisher EventPublisher
	logger    *zap.Logger
}

func NewUnassignTaskHandler(tasks TaskUnassigner, publisher EventPublisher, logger *zap.Logger) *UnassignTaskHandler {
	return &UnassignTaskHandler{tasks: tasks, publisher: publisher, logger: logger}
}

func (h *UnassignTaskHandler) Handle(ctx context.Context, cmd UnassignTaskCommand) (TaskResult, error) {
	var previous *uint
	if obj, err := h.tasks.GetByID(ctx, cmd.TaskID); err == nil {
		previous = obj.AssigneeID
	}

	obj, err := h.tasks.Unassign(ctx, cmd.Actor, cmd.TaskID)
	if err != nil {
		return TaskResult{}, err
	}

	result := toTaskResult(obj)
	publish(ctx, h.publisher, TaskEvent{
		Name:               EventTaskUnassigned,
		Task:               result,
		PreviousAssigneeID: previous,
	})

	return result, nil
}
