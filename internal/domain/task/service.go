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

	minCommentSize = 1
	maxCommentSize = 5000

	minChecklistTitle = 1
	maxChecklistTitle = 300
	// Ограничение на размер чек-листа: длинный список — признак того,
	// что задачу пора разбивать, а не наращивать внутри одной карточки.
	maxChecklistItems = 50
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
	Project     Project
	AssigneeID  *uint

	// BugID — баг из feedback-service, который берут в работу этой
	// задачей. Допустим только вместе с типом bug.
	BugID *uint

	// Checklist — заготовка списка дел, заполняемая прямо в форме
	// создания: пункты обычно известны сразу, и заводить их отдельными
	// запросами после создания задачи было бы лишним кругом.
	Checklist []string
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
	Project     *Project
}

type Service struct {
	repo      Repository
	comments  CommentRepository
	checklist ChecklistRepository
	staff     StaffChecker

	// bugs — каталог багов feedback-service. nil означает, что
	// интеграция не настроена: задачи работают, багов просто нет.
	bugs BugCatalog

	logger *zap.Logger
}

func NewService(
	repo Repository,
	comments CommentRepository,
	checklist ChecklistRepository,
	staff StaffChecker,
	bugs BugCatalog,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:      repo,
		comments:  comments,
		checklist: checklist,
		staff:     staff,
		bugs:      bugs,
		logger:    logger,
	}
}

// ListOpenBugs возвращает баги, которые можно взять в задачу.
//
// Без настроенного каталога перечень пуст, а не ошибочен: интеграция
// необязательна, и её отсутствие не должно ломать форму создания.
func (s *Service) ListOpenBugs(ctx context.Context, filter BugFilter) ([]Bug, error) {
	if s.bugs == nil {
		return nil, nil
	}

	objs, err := s.bugs.ListOpen(ctx, filter)
	if err != nil {
		return nil, asBugUnavailable(err)
	}
	return objs, nil
}

// asBugUnavailable приводит отказ каталога к доменной ошибке.
//
// Сбой чужого сервиса — это недоступность зависимости, а не внутренняя
// ошибка: клиент по ней покажет «сервис багов временно недоступен» и
// предложит повтор. Гарантию даёт домен, а не адаптер: адаптеров может
// стать несколько, и забытая обёртка в одном из них вернула бы
// пользователю общий 500 вместо понятного сообщения.
func asBugUnavailable(err error) error {
	if _, ok := apperror.As(err); ok {
		return err
	}
	return ErrBugUnavailable(err)
}

// GetBug отдаёт подробности бага, привязанного к задаче.
//
// Идентификатор берётся из самой задачи, а не из запроса: показывать
// произвольный баг в карточке чужой задачи незачем, а так связь
// проверяется сама собой.
func (s *Service) GetBug(ctx context.Context, taskID uint) (Bug, error) {
	obj, err := s.load(ctx, taskID)
	if err != nil {
		return Bug{}, err
	}
	if !obj.HasBug() {
		return Bug{}, ErrNoBugAttached(taskID)
	}
	if s.bugs == nil {
		return Bug{}, ErrBugUnavailable(nil)
	}

	details, err := s.bugs.Get(ctx, *obj.BugID)
	if err != nil {
		return Bug{}, asBugUnavailable(err)
	}
	return details, nil
}

// syncBug переносит состояние задачи на привязанный к ней баг.
//
// Сбой синхронизации не отменяет уже выполненную операцию с задачей:
// чужой сервис может быть недоступен, и это не повод отказывать в
// смене статуса. Расхождение видно в логах и чинится следующей сменой.
func (s *Service) syncBug(ctx context.Context, obj *Task) {
	if s.bugs == nil || !obj.HasBug() {
		return
	}

	target, ok := BugStatusFor(obj.Status)
	if !ok {
		return
	}

	if err := s.bugs.SyncStatus(ctx, obj.ID, target); err != nil {
		s.logger.Warn("sync bug status failed",
			zap.Uint("task_id", obj.ID),
			zap.Uint("bug_id", *obj.BugID),
			zap.String("status", string(target)),
			zap.Error(err),
		)
	}
}

// releaseBugByID освобождает конкретный баг — тот, который задача
// перестала вести, сменив его на другой.
func (s *Service) releaseBugByID(ctx context.Context, taskID, bugID uint) {
	if s.bugs == nil {
		return
	}

	if err := s.bugs.UnlinkBug(ctx, bugID); err != nil {
		s.logger.Warn("release previous bug failed",
			zap.Uint("task_id", taskID),
			zap.Uint("bug_id", bugID),
			zap.Error(err),
		)
	}
}

// releaseBug возвращает баг в перечень свободных.
//
// Вызывается, когда задача перестаёт вести баг: её удалили или привязку
// сняли вручную. Как и синхронизация, сбой только логируется.
func (s *Service) releaseBug(ctx context.Context, taskID, bugID uint) {
	if s.bugs == nil {
		return
	}

	if err := s.bugs.Unlink(ctx, taskID); err != nil {
		s.logger.Warn("release bug failed",
			zap.Uint("task_id", taskID),
			zap.Uint("bug_id", bugID),
			zap.Error(err),
		)
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

// validateCommentBody проверяет текст комментария.
func validateCommentBody(body string) error {
	if body == "" {
		return ErrCommentBodyRequired()
	}
	if length := len([]rune(body)); length < minCommentSize {
		return ErrCommentBodyRequired()
	} else if length > maxCommentSize {
		return ErrCommentBodyTooLong(maxCommentSize)
	}
	return nil
}

// validateChecklistTitle проверяет текст пункта чек-листа.
func validateChecklistTitle(title string) error {
	if title == "" {
		return ErrChecklistTitleRequired()
	}
	if len([]rune(title)) > maxChecklistTitle {
		return ErrChecklistTitleTooLong(maxChecklistTitle)
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
