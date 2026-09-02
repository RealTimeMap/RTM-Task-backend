package task

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
)

// ListComments возвращает обсуждение задачи.
//
// Читать может любой, кто видит задачу: доступ к самим задачам
// ограничивает шлюз, отдельного правила для чтения обсуждения нет.
func (s *Service) ListComments(ctx context.Context, taskID uint) ([]*Comment, error) {
	if _, err := s.load(ctx, taskID); err != nil {
		return nil, err
	}
	return s.comments.ListByTask(ctx, taskID)
}

// AddComment добавляет реплику в обсуждение задачи.
//
// Комментировать может тот же круг, что и править задачу: обсуждение —
// часть работы над ней, а не публичная лента.
func (s *Service) AddComment(ctx context.Context, actor role.Actor, taskID uint, body string) (*Comment, error) {
	obj, err := s.load(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !canEdit(obj, actor) {
		return nil, ErrCommentForbidden()
	}

	comment, err := NewComment(taskID, actor.StaffID, body)
	if err != nil {
		return nil, err
	}

	created, err := s.comments.Create(ctx, comment)
	if err != nil {
		return nil, err
	}

	s.logger.Info("task comment added",
		zap.Uint("task_id", taskID),
		zap.Uint("comment_id", created.ID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return created, nil
}

// EditComment правит текст комментария. Свой текст правит автор,
// чужой — только управляющая роль.
func (s *Service) EditComment(ctx context.Context, actor role.Actor, taskID, commentID uint, body string) (*Comment, error) {
	comment, err := s.loadComment(ctx, taskID, commentID)
	if err != nil {
		return nil, err
	}
	if !canManageComment(comment, actor) {
		return nil, ErrCommentEditForbidden()
	}

	if err := comment.Rewrite(body); err != nil {
		return nil, err
	}

	updated, err := s.comments.Update(ctx, comment)
	if err != nil {
		return nil, err
	}

	s.logger.Info("task comment edited",
		zap.Uint("task_id", taskID),
		zap.Uint("comment_id", commentID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return updated, nil
}

// DeleteComment удаляет комментарий. Права те же, что и у правки.
func (s *Service) DeleteComment(ctx context.Context, actor role.Actor, taskID, commentID uint) error {
	comment, err := s.loadComment(ctx, taskID, commentID)
	if err != nil {
		return err
	}
	if !canManageComment(comment, actor) {
		return ErrCommentEditForbidden()
	}

	if err := s.comments.Delete(ctx, commentID); err != nil {
		return err
	}

	s.logger.Info("task comment deleted",
		zap.Uint("task_id", taskID),
		zap.Uint("comment_id", commentID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return nil
}

// loadComment достаёт комментарий и проверяет, что он принадлежит задаче.
// Без этой проверки идентификатор из чужой задачи менял бы не ту запись.
func (s *Service) loadComment(ctx context.Context, taskID, commentID uint) (*Comment, error) {
	comment, err := s.comments.GetByID(ctx, commentID)
	if err != nil {
		return nil, err
	}
	if comment == nil {
		return nil, ErrCommentNotFound(commentID)
	}
	if comment.TaskID != taskID {
		return nil, ErrForeignChild("comment", commentID)
	}
	return comment, nil
}

// canManageComment — правило доступа к чужому тексту: свой комментарий
// правит автор, чужой — только управляющая роль.
func canManageComment(comment *Comment, actor role.Actor) bool {
	return comment.IsWrittenBy(actor.StaffID) || actor.Role.CanEditForeignTask()
}
