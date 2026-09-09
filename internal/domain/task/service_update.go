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

	if err := obj.ApplyDetails(
		params.Title, params.Description, params.Priority, params.Type, params.Project,
	); err != nil {
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

	// Привязанный баг следует за задачей: взяли в работу — баг в работе,
	// завершили — баг закрыт.
	s.syncBug(ctx, updated)

	s.logger.Info("task status changed",
		zap.Uint("task_id", updated.ID),
		zap.String("status", target.String()),
		zap.Uint("actor_id", actor.StaffID),
	)
	return updated, nil
}

// AttachBug привязывает баг к существующей задаче.
//
// Права те же, что и у редактирования: тот, кто ведёт задачу, решает,
// над каким багом в ней работают.
func (s *Service) AttachBug(ctx context.Context, actor role.Actor, id, bugID uint) (*Task, error) {
	if s.bugs == nil {
		return nil, ErrBugUnavailable(nil)
	}

	obj, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canEdit(obj, actor) {
		return nil, ErrEditForbidden()
	}

	// Прежняя привязка снимается в каталоге: иначе старый баг остался бы
	// числиться за этой задачей и не вернулся бы в перечень свободных.
	previous := obj.BugID

	if err := obj.AttachBug(bugID); err != nil {
		return nil, err
	}
	// Каталог отмечает баг занятым до записи в базу: если он откажет
	// (баг уже забрали), задача останется с прежней привязкой.
	if err := s.bugs.Link(ctx, bugID, obj.ID); err != nil {
		return nil, err
	}

	updated, err := s.save(ctx, obj)
	if err != nil {
		return nil, err
	}
	// Прежний баг возвращается в перечень свободных: каталог хранит
	// привязку у себя, и без явного снятия старый баг остался бы
	// числиться занятым этой же задачей.
	if previous != nil && *previous != bugID {
		s.releaseBugByID(ctx, updated.ID, *previous)
	}

	// Задача уже могла быть в работе — новый баг должен это отражать.
	s.syncBug(ctx, updated)

	s.logger.Info("bug attached to task",
		zap.Uint("task_id", updated.ID),
		zap.Uint("bug_id", bugID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return updated, nil
}

// DetachBug снимает привязку бага и возвращает его в перечень свободных.
func (s *Service) DetachBug(ctx context.Context, actor role.Actor, id uint) (*Task, error) {
	obj, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canEdit(obj, actor) {
		return nil, ErrEditForbidden()
	}

	bugID := obj.BugID
	if err := obj.DetachBug(); err != nil {
		return nil, err
	}

	updated, err := s.save(ctx, obj)
	if err != nil {
		return nil, err
	}

	s.releaseBug(ctx, updated.ID, *bugID)

	s.logger.Info("bug detached from task",
		zap.Uint("task_id", updated.ID),
		zap.Uint("bug_id", *bugID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return updated, nil
}

// SendToRework возвращает завершённую задачу в работу с замечанием.
// Права те же, что и у закрытия: отменять принятый результат может тот,
// кто отвечает за задачу, а не любой участник.
func (s *Service) SendToRework(ctx context.Context, actor role.Actor, id uint, params ReworkParams) (*Task, error) {
	obj, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canEdit(obj, actor) {
		return nil, ErrReworkForbidden()
	}

	if err := obj.SendToRework(params.Note, actor.StaffID); err != nil {
		return nil, err
	}

	updated, err := s.save(ctx, obj)
	if err != nil {
		return nil, err
	}

	// Возврат в работу снова открывает баг: работа над ним продолжается.
	s.syncBug(ctx, updated)

	s.logger.Info("task sent to rework",
		zap.Uint("task_id", updated.ID),
		zap.String("status", updated.Status.String()),
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
