package task

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ChecklistItem — пункт чек-листа задачи: то, что нужно сделать внутри неё.
//
// Прогресс намеренно не влияет на жизненный цикл: пункт может стать
// неактуальным, и требование «закрыть всё» вынуждало бы удалять историю,
// чтобы завершить задачу.
type ChecklistItem struct {
	gorm.Model

	TaskID uint `gorm:"not null;index"`

	Title string `gorm:"type:varchar(300);not null"`

	// Position задаёт порядок в списке. Хранится явно: сортировка по
	// времени создания рассыпалась бы при перестановке пунктов.
	Position int `gorm:"not null;default:0;index"`

	// DoneAt и DoneByID заполняются вместе: отметка без автора не даёт
	// понять, кто закрыл пункт.
	DoneAt   *time.Time
	DoneByID *uint `gorm:"index"`
}

func (ChecklistItem) TableName() string { return "task_checklist_items" }

// IsDone сообщает, отмечен ли пункт выполненным.
func (i *ChecklistItem) IsDone() bool { return i.DoneAt != nil }

// NewChecklistItem собирает пункт чек-листа, проверив заголовок.
func NewChecklistItem(taskID uint, title string, position int) (*ChecklistItem, error) {
	title = strings.TrimSpace(title)
	if err := validateChecklistTitle(title); err != nil {
		return nil, err
	}
	return &ChecklistItem{TaskID: taskID, Title: title, Position: position}, nil
}

// Rename меняет текст пункта.
func (i *ChecklistItem) Rename(title string) error {
	title = strings.TrimSpace(title)
	if err := validateChecklistTitle(title); err != nil {
		return err
	}
	i.Title = title
	return nil
}

// SetDone отмечает пункт выполненным или снимает отметку.
// Повторная отметка не переписывает автора: важно, кто закрыл пункт
// первым, а не кто нажал кнопку последним.
func (i *ChecklistItem) SetDone(done bool, staffID uint) {
	if done == i.IsDone() {
		return
	}
	if !done {
		i.DoneAt = nil
		i.DoneByID = nil
		return
	}

	now := time.Now()
	i.DoneAt = &now
	i.DoneByID = &staffID
}

// ChecklistProgress — сводка по чек-листу для представления задачи.
type ChecklistProgress struct {
	Total int
	Done  int
}

// NewChecklistProgress считает сводку по набору пунктов.
func NewChecklistProgress(items []*ChecklistItem) ChecklistProgress {
	progress := ChecklistProgress{Total: len(items)}
	for _, item := range items {
		if item.IsDone() {
			progress.Done++
		}
	}
	return progress
}
