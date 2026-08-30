package task

import (
	"time"

	"gorm.io/gorm"
)

type Type string

const (
	BugType     Type = "bug"
	FeatureType Type = "feature"
	FixType     Type = "fix"
)

func (t Type) IsValid() bool {
	switch t {
	case BugType, FeatureType, FixType:
		return true
	default:
		return false
	}
}

func (t Type) String() string { return string(t) }

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

// IsFinal сообщает, что из статуса нет исходящих переходов.
func (s Status) IsFinal() bool { return s == CompleteStatus }

// allowedTransitions описывает жизненный цикл задачи. Это правило домена,
// поэтому таблица переходов лежит рядом с моделью, а не в use case.
var allowedTransitions = map[Status][]Status{
	NewStatus:      {WorkingStatus},
	WorkingStatus:  {ReviewStatus, NewStatus},
	ReviewStatus:   {CompleteStatus, WorkingStatus},
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

	Title       string `gorm:"type:varchar(300);not null"`
	Description string `gorm:"type:text"`

	CreatorID  uint  `gorm:"not null;index"`
	AssigneeID *uint `gorm:"index"`

	Version  int        `gorm:"not null;default:1"`
	ClosedAt *time.Time `gorm:"index"`
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
	} else {
		t.ClosedAt = nil
	}
	return nil
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
func (t *Task) ApplyDetails(title, description *string, priority *Priority, taskType *Type) error {
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
		t.Type = *taskType
	}
	return nil
}
