package task

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
)

// Update меняет описательные поля задачи.
func (s *Service) Update(ctx context.Context, actor role.Actor, id uint, params UpdateTaskParams) (*Task, error) {
	obj, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canEdit(obj, actor) {
		return nil, ErrEditForbidden()
	}

	if err := obj.ApplyDetails(params.Title, params.Description, params.Priority, params.Type); err != nil {
		return nil, err
	}

	updated, err := s.save(ctx, obj)
	if err != nil {
		return nil, err
	}

	s.logger.Info("task updated",
		zap.Uint("task_id", updated.ID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return updated, nil
}

// ChangeStatus переводит задачу по жизненному циклу.
func (s *Service) ChangeStatus(ctx context.Context, actor role.Actor, id uint, target Status) (*Task, error) {
	obj, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canEdit(obj, actor) {
		return nil, ErrEditForbidden()
	}
	// Закрытие — отдельное право: подтвердить результат может исполнитель
	// либо управляющая роль, но не любой участник.
	if target.IsFinal() && !obj.IsAssignedTo(actor.StaffID) && !actor.Role.CanCloseForeignTask() {
		return nil, ErrCloseForbidden()
	}

	if err := obj.ChangeStatus(target); err != nil {
		return nil, err
	}

	updated, err := s.save(ctx, obj)
	if err != nil {
		return nil, err
	}

	s.logger.Info("task status changed",
		zap.Uint("task_id", updated.ID),
		zap.String("status", target.String()),
		zap.Uint("actor_id", actor.StaffID),
	)
	return updated, nil
}

// Assign назначает исполнителя задачи.
func (s *Service) Assign(ctx context.Context, actor role.Actor, id, assigneeID uint) (*Task, error) {
	obj, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCanAssign(ctx, actor, assigneeID); err != nil {
		return nil, err
	}
	if err := obj.Assign(assigneeID); err != nil {
		return nil, err
	}

	updated, err := s.save(ctx, obj)
	if err != nil {
		return nil, err
	}

	s.logger.Info("task assigned",
		zap.Uint("task_id", updated.ID),
		zap.Uint("assignee_id", assigneeID),
	)
	return updated, nil
}

// Unassign снимает исполнителя с задачи.
func (s *Service) Unassign(ctx context.Context, actor role.Actor, id uint) (*Task, error) {
	obj, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	// Снять исполнителя может он сам либо управляющая роль.
	if !obj.IsAssignedTo(actor.StaffID) && !actor.Role.CanAssignAnyone() {
		return nil, ErrAssignForbidden()
	}
	if err := obj.Unassign(); err != nil {
		return nil, err
	}

	return s.save(ctx, obj)
}
