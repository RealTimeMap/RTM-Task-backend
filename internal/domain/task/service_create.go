package task

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
)

// Create заводит новую задачу от имени actor.
func (s *Service) Create(ctx context.Context, actor role.Actor, params CreateTaskParams) (*Task, error) {
	if !actor.Role.CanCreateTask() {
		return nil, ErrCreateForbidden(actor.Role.String())
	}

	title := strings.TrimSpace(params.Title)
	if err := validateTitle(title); err != nil {
		return nil, err
	}
	if err := validateDescription(params.Description); err != nil {
		return nil, err
	}
	if !params.Type.IsValid() {
		return nil, ErrInvalidType(params.Type.String())
	}

	priority := params.Priority
	if priority == 0 {
		priority = HighPriority
	}
	if !priority.IsValid() {
		return nil, ErrInvalidPriority(priority.Int())
	}

	project := params.Project
	if project == "" {
		project = DefaultProject
	}
	if !project.IsValid() {
		return nil, ErrInvalidProject(project.String())
	}

	// Баг привязывают только к задаче типа «баг» — проверяем до записи,
	// чтобы не заводить задачу, которую всё равно придётся исправлять.
	if params.BugID != nil && params.Type != BugType {
		return nil, ErrBugOnNonBugTask(params.Type.String())
	}

	if err := s.ensureDailyLimit(ctx, actor.StaffID); err != nil {
		return nil, err
	}

	obj := &Task{
		Title:       title,
		Description: params.Description,
		Type:        params.Type,
		Priority:    priority,
		Project:     project,
		Status:      NewStatus,
		CreatorID:   actor.StaffID,
		Version:     1,
		BugID:       params.BugID,
	}

	// Исполнитель на старте необязателен, но если указан — проверяем,
	// что сотрудник существует и активен, и что actor вправе его назначить.
	if params.AssigneeID != nil {
		if err := s.ensureCanAssign(ctx, actor, *params.AssigneeID); err != nil {
			return nil, err
		}
		obj.AssigneeID = params.AssigneeID
	}

	if len(params.Checklist) > maxChecklistItems {
		return nil, ErrChecklistLimitReached(maxChecklistItems)
	}

	created, err := s.repo.Create(ctx, obj)
	if err != nil {
		return nil, err
	}

	// Баг отмечается занятым после создания задачи: до этого момента
	// её идентификатора ещё нет, а привязка без него бессмысленна.
	//
	// Отказ каталога откатывает привязку в самой задаче, но не саму
	// задачу: она уже заведена и полезна, а баг к ней можно привязать
	// повторно. Оставить ссылку на баг, который каталог считает
	// свободным, нельзя — его выдали бы второй задаче.
	if created.HasBug() {
		if err := s.linkBug(ctx, created); err != nil {
			return nil, err
		}
	}

	// Чек-лист заводится после задачи: пунктам нужен её идентификатор.
	// Сбой здесь не откатывает саму задачу — она уже создана и полезна
	// без списка, а пункты пользователь добавит вручную.
	if _, err := s.createChecklist(ctx, created.ID, params.Checklist); err != nil {
		s.logger.Warn("create task checklist failed",
			zap.Uint("task_id", created.ID),
			zap.Error(err),
		)
	}

	s.logger.Info("task created",
		zap.Uint("task_id", created.ID),
		zap.Uint("creator_id", actor.StaffID),
	)
	return created, nil
}

// ensureDailyLimit защищает от массового создания задач одним сотрудником.
func (s *Service) ensureDailyLimit(ctx context.Context, creatorID uint) error {
	count, err := s.repo.TodayCreated(ctx, creatorID)
	if err != nil {
		return err
	}
	if count >= maxTasksPerDay {
		return ErrDailyLimitReached(maxTasksPerDay)
	}
	return nil
}

// ensureCanAssign проверяет право назначения и пригодность исполнителя.
func (s *Service) ensureCanAssign(ctx context.Context, actor role.Actor, assigneeID uint) error {
	// Разработчик вправе взять задачу только на себя.
	if !actor.Role.CanAssignAnyone() && !actor.Is(assigneeID) {
		return ErrAssignForbidden()
	}
	return s.staff.EnsureAssignable(ctx, assigneeID)
}

// linkBug отмечает баг занятым этой задачей.
//
// Если каталог отказал (баг уже забрали, сервис недоступен), ссылка
// снимается с задачи: расхождение, в котором задача считает баг своим,
// а каталог отдаёт его другим, хуже, чем задача вовсе без бага.
func (s *Service) linkBug(ctx context.Context, obj *Task) error {
	if s.bugs == nil {
		return ErrBugUnavailable(nil)
	}

	err := s.bugs.Link(ctx, *obj.BugID, obj.ID)
	if err == nil {
		return nil
	}

	s.logger.Warn("link bug failed, task created without it",
		zap.Uint("task_id", obj.ID),
		zap.Uint("bug_id", *obj.BugID),
		zap.Error(err),
	)

	obj.BugID = nil
	if _, saveErr := s.save(ctx, obj); saveErr != nil {
		s.logger.Error("rollback bug link failed",
			zap.Uint("task_id", obj.ID),
			zap.Error(saveErr),
		)
	}
	return err
}
