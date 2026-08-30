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

	if err := s.ensureDailyLimit(ctx, actor.StaffID); err != nil {
		return nil, err
	}

	obj := &Task{
		Title:       title,
		Description: params.Description,
		Type:        params.Type,
		Priority:    priority,
		Status:      NewStatus,
		CreatorID:   actor.StaffID,
		Version:     1,
	}

	// Исполнитель на старте необязателен, но если указан — проверяем,
	// что сотрудник существует и активен, и что actor вправе его назначить.
	if params.AssigneeID != nil {
		if err := s.ensureCanAssign(ctx, actor, *params.AssigneeID); err != nil {
			return nil, err
		}
		obj.AssigneeID = params.AssigneeID
	}

	created, err := s.repo.Create(ctx, obj)
	if err != nil {
		return nil, err
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
