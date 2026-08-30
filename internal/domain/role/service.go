package role

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/utils/apperror"
)

type Service struct {
	repo   Repository
	logger *zap.Logger
}

func NewService(repo Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// GetByID возвращает сотрудника по идентификатору.
func (s *Service) GetByID(ctx context.Context, id uint) (*Staff, error) {
	return s.repo.GetByID(ctx, id)
}

// List возвращает всех сотрудников.
func (s *Service) List(ctx context.Context) ([]*Staff, error) {
	return s.repo.List(ctx)
}

// ChangeRole меняет роль сотрудника. Право на операцию есть только у админа.
func (s *Service) ChangeRole(ctx context.Context, actor Actor, staffID uint, newRole Role) (*Staff, error) {
	if actor.Role != AdminRole {
		return nil, apperror.NewForbiddenError("only admin can change staff roles")
	}
	if !newRole.IsValid() {
		return nil, ErrInvalidRole(newRole.String())
	}

	staff, err := s.repo.GetByID(ctx, staffID)
	if err != nil {
		return nil, err
	}

	staff.Role = newRole
	return s.repo.Update(ctx, staff)
}

// Deactivate отключает сотрудника, сохраняя историю его задач.
func (s *Service) Deactivate(ctx context.Context, actor Actor, staffID uint) (*Staff, error) {
	if actor.Role != AdminRole {
		return nil, apperror.NewForbiddenError("only admin can deactivate staff")
	}

	staff, err := s.repo.GetByID(ctx, staffID)
	if err != nil {
		return nil, err
	}

	staff.IsActive = false
	return s.repo.Update(ctx, staff)
}

// ResolveActor превращает пользователя платформы в участника операции.
//
// Сотрудник заводится при первом обращении: попасть сюда может только
// администратор платформы (это проверяет транспорт), а значит человек
// заведомо имеет право работать с задачами — заставлять кого-то заводить
// ему учётную запись вручную было бы лишним шагом.
//
// Стартовая роль — manager: доступ администратора платформы к сервису
// не делает его администратором самого сервиса, менять роли сотрудников
// по-прежнему может только staff-админ.
func (s *Service) ResolveActor(ctx context.Context, identity Identity) (Actor, error) {
	staff, err := s.repo.GetByID(ctx, identity.UserID)
	if err == nil {
		if !staff.IsActive {
			return Actor{}, ErrStaffInactive(staff.ID)
		}

		// Имя приходит из auth-service и там могло измениться.
		if identity.UserName != "" && staff.FullName != identity.UserName {
			staff.FullName = identity.UserName
			if updated, err := s.repo.Update(ctx, staff); err == nil {
				staff = updated
			} else {
				s.logger.Warn("sync staff name failed",
					zap.Uint("staff_id", staff.ID),
					zap.Error(err),
				)
			}
		}

		return staff.AsActor(), nil
	}

	if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindNotFound {
		return Actor{}, err
	}

	created, err := s.repo.Create(ctx, &Staff{
		Model:    gorm.Model{ID: identity.UserID},
		FullName: identity.UserName,
		Role:     ManagerRole,
		IsActive: true,
	})
	if err != nil {
		return Actor{}, err
	}

	s.logger.Info("staff provisioned from platform identity",
		zap.Uint("staff_id", created.ID),
		zap.String("user_name", created.FullName),
	)

	return created.AsActor(), nil
}

// EnsureAssignable проверяет, что на сотрудника можно повесить задачу.
func (s *Service) EnsureAssignable(ctx context.Context, staffID uint) error {
	staff, err := s.repo.GetByID(ctx, staffID)
	if err != nil {
		return err
	}
	if !staff.IsActive {
		return ErrStaffInactive(staff.ID)
	}
	return nil
}
