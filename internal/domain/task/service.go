package task

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
)

const (
	minTitleLength     = 3
	maxTitleLength     = 300
	maxDescriptionSize = 10000
	minReworkNoteSize  = 5
	maxReworkNoteSize  = 5000
	maxTasksPerDay     = 50
)

// StaffChecker — то, что домену задач нужно знать о сотрудниках.
// Узкий порт вместо зависимости от всего role.Service.
type StaffChecker interface {
	EnsureAssignable(ctx context.Context, staffID uint) error
}

type CreateTaskParams struct {
	Title       string
	Description string
	Type        Type
	Priority    Priority
	AssigneeID  *uint
}

// ReworkParams — данные возврата завершённой задачи в работу.
type ReworkParams struct {
	Note string
}

type UpdateTaskParams struct {
	Title       *string
	Description *string
	Type        *Type
	Priority    *Priority
}

type Service struct {
	repo  Repository
	staff StaffChecker

	logger *zap.Logger
}

func NewService(repo Repository, staff StaffChecker, logger *zap.Logger) *Service {
	return &Service{
		repo:   repo,
		staff:  staff,
		logger: logger,
	}
}

// load достаёт задачу и приводит отсутствие записи к доменной ошибке.
func (s *Service) load(ctx context.Context, id uint) (*Task, error) {
	obj, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, ErrTaskNotFound(id)
	}
	return obj, nil
}

// save сохраняет задачу и поднимает версию агрегата.
func (s *Service) save(ctx context.Context, obj *Task) (*Task, error) {
	expected := obj.Version
	obj.Version = expected + 1
	return s.repo.Update(ctx, obj, expected)
}

func validateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return apperror.NewRequiredError("title")
	}
	if len([]rune(title)) < minTitleLength {
		return apperror.NewTooShortError("title", minTitleLength, title)
	}
	if len([]rune(title)) > maxTitleLength {
		return apperror.NewTooLongError("title", maxTitleLength, title)
	}
	return nil
}

func validateDescription(description string) error {
	if len([]rune(description)) > maxDescriptionSize {
		return apperror.NewTooLongError("description", maxDescriptionSize, len(description))
	}
	return nil
}

// validateReworkNote проверяет описание доработки. Пустое замечание
// бессмысленно: исполнитель не поймёт, что от него хотят.
func validateReworkNote(note string) error {
	if note == "" {
		return ErrReworkNoteRequired()
	}
	if length := len([]rune(note)); length < minReworkNoteSize {
		return ErrReworkNoteTooShort(minReworkNoteSize)
	} else if length > maxReworkNoteSize {
		return ErrReworkNoteTooLong(maxReworkNoteSize)
	}
	return nil
}

// canEdit — общее правило доступа к изменению задачи:
// автор, исполнитель или управляющая роль.
func canEdit(obj *Task, actor role.Actor) bool {
	return obj.IsCreatedBy(actor.StaffID) ||
		obj.IsAssignedTo(actor.StaffID) ||
		actor.Role.CanEditForeignTask()
}
