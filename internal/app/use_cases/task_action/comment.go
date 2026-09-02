package task_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

// TaskCommenter — часть домена, обслуживающая обсуждение задачи.
type TaskCommenter interface {
	ListComments(ctx context.Context, taskID uint) ([]*task.Comment, error)
	AddComment(ctx context.Context, actor role.Actor, taskID uint, body string) (*task.Comment, error)
	EditComment(ctx context.Context, actor role.Actor, taskID, commentID uint, body string) (*task.Comment, error)
	DeleteComment(ctx context.Context, actor role.Actor, taskID, commentID uint) error
}

type ListCommentsQuery struct {
	TaskID uint
}

type AddCommentCommand struct {
	Actor  role.Actor
	TaskID uint
	Body   string
}

func (c AddCommentCommand) Validate() error {
	if c.Body == "" {
		return apperror.NewRequiredError("body")
	}
	return nil
}

type EditCommentCommand struct {
	Actor     role.Actor
	TaskID    uint
	CommentID uint
	Body      string
}

func (c EditCommentCommand) Validate() error {
	if c.Body == "" {
		return apperror.NewRequiredError("body")
	}
	return nil
}

type DeleteCommentCommand struct {
	Actor     role.Actor
	TaskID    uint
	CommentID uint
}

// CommentHandler обслуживает обсуждение задачи целиком.
//
// Один обработчик на четыре операции, а не четыре структуры: все они
// работают с одним портом и одним событием, и разносить их значило бы
// повторить один и тот же конструктор четыре раза.
type CommentHandler struct {
	tasks     TaskCommenter
	publisher EventPublisher
	logger    *zap.Logger
}

func NewCommentHandler(tasks TaskCommenter, publisher EventPublisher, logger *zap.Logger) *CommentHandler {
	return &CommentHandler{tasks: tasks, publisher: publisher, logger: logger}
}

func (h *CommentHandler) List(ctx context.Context, query ListCommentsQuery) ([]CommentResult, error) {
	objs, err := h.tasks.ListComments(ctx, query.TaskID)
	if err != nil {
		return nil, err
	}
	return toCommentResults(objs), nil
}

func (h *CommentHandler) Add(ctx context.Context, cmd AddCommentCommand) (CommentResult, error) {
	if err := cmd.Validate(); err != nil {
		return CommentResult{}, err
	}

	obj, err := h.tasks.AddComment(ctx, cmd.Actor, cmd.TaskID, cmd.Body)
	if err != nil {
		return CommentResult{}, err
	}

	result := toCommentResult(obj)
	publishComment(ctx, h.publisher, CommentEvent{Name: EventCommentAdded, Comment: result})
	return result, nil
}

func (h *CommentHandler) Edit(ctx context.Context, cmd EditCommentCommand) (CommentResult, error) {
	if err := cmd.Validate(); err != nil {
		return CommentResult{}, err
	}

	obj, err := h.tasks.EditComment(ctx, cmd.Actor, cmd.TaskID, cmd.CommentID, cmd.Body)
	if err != nil {
		return CommentResult{}, err
	}

	result := toCommentResult(obj)
	publishComment(ctx, h.publisher, CommentEvent{Name: EventCommentUpdated, Comment: result})
	return result, nil
}

func (h *CommentHandler) Delete(ctx context.Context, cmd DeleteCommentCommand) error {
	if err := h.tasks.DeleteComment(ctx, cmd.Actor, cmd.TaskID, cmd.CommentID); err != nil {
		return err
	}

	// Удалённого комментария больше нет — подписчикам достаточно того,
	// какая запись и в какой задаче исчезла.
	publishComment(ctx, h.publisher, CommentEvent{
		Name:    EventCommentDeleted,
		Comment: CommentResult{ID: cmd.CommentID, TaskID: cmd.TaskID},
	})
	return nil
}
