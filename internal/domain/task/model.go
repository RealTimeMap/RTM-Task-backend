package task

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type Type string

const (
	BugType      Type = "bug"
	FeatureType  Type = "feature"
	FixType      Type = "fix"
	RefactorType Type = "refactor"
	UpdateType   Type = "update"
)

func (t Type) IsValid() bool {
	switch t {
	case BugType, FeatureType, FixType, RefactorType, UpdateType:
		return true
	default:
		return false
	}
}

func (t Type) String() string { return string(t) }

// Project — продукт, к которому относится задача.
//
// Список закрыт и живёт в домене: проектов ровно два, и заводить ради
// них справочник в базе значило бы поддерживать таблицу из двух строк
// вместе с CRUD, миграциями и проверками целостности.
type Project string

const (
	TaskProject Project = "rtm-task"
	AppProject  Project = "rtm-app"
)

// DefaultProject — куда попадает задача, если проект не указали.
// Сервис задач ведёт сам себя, поэтому умолчание — он.
const DefaultProject = TaskProject

func (p Project) IsValid() bool {
	switch p {
	case TaskProject, AppProject:
		return true
	default:
		return false
	}
}

func (p Project) String() string { return string(p) }

type Status string

const (
	NewStatus      Status = "new"
	WorkingStatus  Status = "working"
	ReviewStatus   Status = "review"
	CompleteStatus Status = "complete"
)

func (s Status) IsValid() bool {
	switch s {
	case NewStatus, WorkingStatus, ReviewStatus, CompleteStatus:
		return true
	default:
		return false
	}
}

func (s Status) String() string { return string(s) }

// IsFinal сообщает, что статус закрывает задачу. Из complete есть один
// выход — возврат в работу через SendToRework, поэтому «финальный» здесь
// означает «закрытая», а не «тупиковая» вершина графа.
func (s Status) IsFinal() bool { return s == CompleteStatus }

// allowedTransitions описывает жизненный цикл задачи. Это правило домена,
// поэтому таблица переходов лежит рядом с моделью, а не в use case.
var allowedTransitions = map[Status][]Status{
	NewStatus:     {WorkingStatus},
	WorkingStatus: {ReviewStatus, NewStatus},
	ReviewStatus:  {CompleteStatus, WorkingStatus},
	// Из complete обычного перехода нет: вернуть задачу в работу можно
	// только через SendToRework, который требует описания доработки.
	CompleteStatus: {},
}

// CanTransitionTo проверяет допустимость перехода по таблице жизненного цикла.
func (s Status) CanTransitionTo(target Status) bool {
	for _, allowed := range allowedTransitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}

type Priority int

const (
	BasePriority   Priority = 10
	HighPriority   Priority = 20
	MediumPriority Priority = 30
	LowPriority    Priority = 40
)

func (p Priority) IsValid() bool {
	switch p {
	case BasePriority, HighPriority, MediumPriority, LowPriority:
		return true
	default:
		return false
	}
}

func (p Priority) Int() int { return int(p) }

// Task — корень агрегата. Инварианты жизненного цикла и владения
// поддерживаются методами самой сущности.
type Task struct {
	gorm.Model

	Type     Type     `gorm:"type:varchar(20);not null;index"`
	Status   Status   `gorm:"type:varchar(20);not null;default:'new';index"`
	Priority Priority `gorm:"not null;default:20;index"`

	// Project — продукт, в который направлена задача. По нему доска
	// делится между командами.
	Project Project `gorm:"type:varchar(20);not null;default:'rtm-task';index"`

	Title       string `gorm:"type:varchar(300);not null"`
	Description string `gorm:"type:text"`

	CreatorID  uint  `gorm:"not null;index"`
	AssigneeID *uint `gorm:"index"`

	Version  int        `gorm:"not null;default:1"`
	ClosedAt *time.Time `gorm:"index"`

	// Доработка: заполняется при возврате завершённой задачи в работу и
	// живёт до следующего закрытия. Хранится сырым markdown — как и
	// Description, разметку разбирает фронтенд.
	ReworkNote string     `gorm:"type:text"`
	ReworkByID *uint      `gorm:"index"`
	ReworkAt   *time.Time `gorm:"index"`

	// BugID — баг из feedback-service, над которым ведётся работа.
	// Заполняется только у задач типа bug: привязывать баг к рефакторингу
	// нечего. Идентификатор внешний, чужая база — поэтому просто индекс,
	// без внешнего ключа.
	BugID *uint `gorm:"index"`
}

// HasBug сообщает, что к задаче привязан баг.
func (t *Task) HasBug() bool { return t.BugID != nil }

// AttachBug привязывает баг к задаче.
//
// Баг ведут только в задаче типа «баг»: привязка к рефакторингу или
// обновлению не значила бы ничего, а обратная синхронизация закрывала
// бы баг по завершении посторонней работы.
func (t *Task) AttachBug(bugID uint) error {
	if t.IsClosed() {
		return ErrTaskClosed(t.ID)
	}
	if t.Type != BugType {
		return ErrBugOnNonBugTask(t.Type.String())
	}
	if t.BugID != nil && *t.BugID == bugID {
		return ErrBugAlreadyAttached(bugID)
	}

	t.BugID = &bugID
	return nil
}

// DetachBug снимает привязку бага.
func (t *Task) DetachBug() error {
	if t.IsClosed() {
		return ErrTaskClosed(t.ID)
	}
	if !t.HasBug() {
		return ErrNoBugAttached(t.ID)
	}

	t.BugID = nil
	return nil
}

func (Task) TableName() string { return "tasks" }

// IsClosed сообщает, завершена ли задача.
func (t *Task) IsClosed() bool { return t.Status.IsFinal() }

// IsAssigned сообщает, назначен ли исполнитель.
func (t *Task) IsAssigned() bool { return t.AssigneeID != nil }

// IsAssignedTo сообщает, назначена ли задача на указанного сотрудника.
func (t *Task) IsAssignedTo(staffID uint) bool {
	return t.AssigneeID != nil && *t.AssigneeID == staffID
}

// IsCreatedBy сообщает, автор ли указанный сотрудник.
func (t *Task) IsCreatedBy(staffID uint) bool { return t.CreatorID == staffID }

// ChangeStatus переводит задачу в новый статус, соблюдая жизненный цикл.
// Закрытие проставляет ClosedAt, возврат из закрытого состояния запрещён.
func (t *Task) ChangeStatus(target Status) error {
	if !target.IsValid() {
		return ErrInvalidStatus(target.String())
	}
	if t.Status == target {
		return ErrSameStatus(target.String())
	}
	if !t.Status.CanTransitionTo(target) {
		return ErrInvalidTransition(t.Status.String(), target.String())
	}
	// Взять задачу в работу без исполнителя нельзя — иначе теряется
	// ответственный за результат.
	if target == WorkingStatus && !t.IsAssigned() {
		return ErrAssigneeRequired()
	}

	t.Status = target
	if target.IsFinal() {
		now := time.Now()
		t.ClosedAt = &now
		// Замечание относилось к прошлому кругу работы: раз задачу снова
		// приняли, оно закрыто вместе с ней.
		t.clearRework()
	} else {
		t.ClosedAt = nil
	}
	return nil
}

// IsInRework сообщает, что у задачи есть незакрытое замечание.
func (t *Task) IsInRework() bool { return t.ReworkAt != nil }

// SendToRework возвращает завершённую задачу в работу с описанием того,
// что нужно доделать. Это единственный выход из complete: обычный переход
// туда запрещён таблицей жизненного цикла, потому что возврат без
// объяснения причины оставляет исполнителя без задания.
func (t *Task) SendToRework(note string, authorID uint) error {
	if !t.IsClosed() {
		return ErrNotClosed(t.ID, t.Status.String())
	}
	note = strings.TrimSpace(note)
	if err := validateReworkNote(note); err != nil {
		return err
	}
	// Возврат идёт к тому, кто задачу делал. Без исполнителя работать
	// некому, поэтому задача уходит в new и ждёт назначения.
	target := WorkingStatus
	if !t.IsAssigned() {
		target = NewStatus
	}

	now := time.Now()
	t.Status = target
	t.ClosedAt = nil
	t.ReworkNote = note
	t.ReworkByID = &authorID
	t.ReworkAt = &now
	return nil
}

// clearRework снимает замечание о доработке.
func (t *Task) clearRework() {
	t.ReworkNote = ""
	t.ReworkByID = nil
	t.ReworkAt = nil
}

// Assign назначает исполнителя. Закрытую задачу переназначать нельзя.
func (t *Task) Assign(staffID uint) error {
	if t.IsClosed() {
		return ErrTaskClosed(t.ID)
	}
	if t.IsAssignedTo(staffID) {
		return ErrAlreadyAssigned(staffID)
	}
	t.AssigneeID = &staffID
	return nil
}

// Unassign снимает исполнителя. Задача в работе требует ответственного,
// поэтому сначала её нужно вернуть в new.
func (t *Task) Unassign() error {
	if t.IsClosed() {
		return ErrTaskClosed(t.ID)
	}
	if !t.IsAssigned() {
		return ErrNotAssigned(t.ID)
	}
	if t.Status == WorkingStatus || t.Status == ReviewStatus {
		return ErrUnassignActiveTask(t.Status.String())
	}
	t.AssigneeID = nil
	return nil
}

// ApplyDetails обновляет описательные поля задачи.
func (t *Task) ApplyDetails(
	title, description *string,
	priority *Priority,
	taskType *Type,
	project *Project,
) error {
	if t.IsClosed() {
		return ErrTaskClosed(t.ID)
	}

	if title != nil {
		if err := validateTitle(*title); err != nil {
			return err
		}
		t.Title = *title
	}
	if description != nil {
		if err := validateDescription(*description); err != nil {
			return err
		}
		t.Description = *description
	}
	if priority != nil {
		if !priority.IsValid() {
			return ErrInvalidPriority(priority.Int())
		}
		t.Priority = *priority
	}
	if taskType != nil {
		if !taskType.IsValid() {
			return ErrInvalidType(taskType.String())
		}
		// Баг ведут только в задаче типа «баг». Смена типа на другой
		// оставила бы привязку висеть: обратная синхронизация закрывала
		// бы баг по завершении работы, которая к нему уже не относится.
		if *taskType != BugType && t.HasBug() {
			return ErrBugAttachedTypeChange(t.Type.String(), taskType.String())
		}
		t.Type = *taskType
	}
	if project != nil {
		if !project.IsValid() {
			return ErrInvalidProject(project.String())
		}
		t.Project = *project
	}
	return nil
}
