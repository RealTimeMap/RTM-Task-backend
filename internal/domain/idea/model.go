// Package idea — копилка замыслов по проекту.
//
// Идея намеренно устроена проще задачи: у неё нет исполнителя,
// приоритета, жизненного цикла и сроков. Всё это появляется тогда,
// когда за замысел берутся, — а до того он живёт в списке «подумать»
// и требует ровно двух состояний: сделано или ещё нет.
//
// Отдельный домен, а не тип задачи: задача с исполнителем и статусом
// попадала бы в доску, счётчики и фильтры, где ей делать нечего, —
// и ради этого пришлось бы прятать её в каждом месте по отдельности.
package idea

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"RTM-Task/internal/utils/apperror"
)

// Ограничения текста. Совпадают по смыслу с задачными: человек пишет
// идею в том же окне и тем же способом, и разные пределы для одного
// и того же действия он воспринимал бы как поломку.
const (
	MinTitleLength     = 3
	MaxTitleLength     = 300
	MaxDescriptionSize = 20000

	MaxCommentSize = 5000
)

// Idea — замысел, который стоит обдумать.
type Idea struct {
	gorm.Model

	Title string `gorm:"type:varchar(300);not null"`

	// Description хранится сырым markdown — как и описание задачи,
	// разметку разбирает фронтенд.
	Description string `gorm:"type:text"`

	// AuthorID — кто завёл идею. Идею нельзя переназначить: автор
	// остаётся автором, даже когда её выполняет кто-то другой.
	AuthorID uint `gorm:"not null;index"`

	// Done — сделана ли идея. Двух состояний достаточно: у замысла
	// нет «в работе» — как только за него взялись всерьёз, заводят
	// задачу, а идею отмечают выполненной.
	Done bool `gorm:"not null;default:false;index"`

	// DoneAt и DoneByID заполняются вместе с Done: по списку
	// выполненного видно, кто и когда его закрыл.
	DoneAt   *time.Time
	DoneByID *uint
}

func (Idea) TableName() string { return "ideas" }

// New собирает идею, проверив текст.
func New(authorID uint, title, description string) (*Idea, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)

	if err := ValidateTitle(title); err != nil {
		return nil, err
	}
	if err := ValidateDescription(description); err != nil {
		return nil, err
	}

	return &Idea{
		Title:       title,
		Description: description,
		AuthorID:    authorID,
	}, nil
}

// IsAuthoredBy сообщает, автор ли указанный сотрудник.
func (i *Idea) IsAuthoredBy(staffID uint) bool { return i.AuthorID == staffID }

// Rename заменяет заголовок.
func (i *Idea) Rename(title string) error {
	title = strings.TrimSpace(title)
	if err := ValidateTitle(title); err != nil {
		return err
	}
	i.Title = title
	return nil
}

// Describe заменяет описание. Пустая строка стирает его — это
// осмысленное действие, а не ошибка: идею могли записать наспех.
func (i *Idea) Describe(description string) error {
	description = strings.TrimSpace(description)
	if err := ValidateDescription(description); err != nil {
		return err
	}
	i.Description = description
	return nil
}

// MarkDone отмечает идею выполненной.
//
// Повторная отметка ничего не меняет: кто закрыл идею первым, тот и
// останется в записи — переписывать автора закрытия на каждом
// повторном щелчке значило бы врать о том, кто это сделал.
func (i *Idea) MarkDone(staffID uint) {
	if i.Done {
		return
	}

	now := time.Now()
	i.Done = true
	i.DoneAt = &now
	i.DoneByID = &staffID
}

// Reopen возвращает идею в список невыполненных, стирая отметку.
func (i *Idea) Reopen() {
	if !i.Done {
		return
	}

	i.Done = false
	i.DoneAt = nil
	i.DoneByID = nil
}

// ValidateTitle проверяет заголовок идеи.
func ValidateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return apperror.NewRequiredError("title")
	}
	if length := len([]rune(title)); length < MinTitleLength {
		return apperror.NewTooShortError("title", MinTitleLength, title)
	} else if length > MaxTitleLength {
		return apperror.NewTooLongError("title", MaxTitleLength, title)
	}
	return nil
}

// ValidateDescription проверяет описание идеи.
func ValidateDescription(description string) error {
	if len([]rune(description)) > MaxDescriptionSize {
		return apperror.NewTooLongError("description", MaxDescriptionSize, len(description))
	}
	return nil
}
