package task

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
)

// Delete удаляет задачу. Право есть у автора и у управляющей роли.
func (s *Service) Delete(ctx context.Context, actor role.Actor, id uint) error {
	obj, err := s.load(ctx, id)
	if err != nil {
		return err
	}

	if !obj.IsCreatedBy(actor.StaffID) && !actor.Role.CanDeleteTask() {
		return ErrDeleteForbidden()
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.logger.Info("task deleted",
		zap.Uint("task_id", id),
		zap.Uint("actor_id", actor.StaffID),
	)
	return nil
}
