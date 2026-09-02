package task

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Comment — обсуждение задачи. Отдельная сущность, а не поле агрегата:
// комментарий живёт своей жизнью и не должен поднимать версию задачи,
// иначе каждая реплика конфликтовала бы с параллельным редактированием.
type Comment struct {
	gorm.Model

	TaskID   uint `gorm:"not null;index"`
	AuthorID uint `gorm:"not null;index"`

	// Body хранится сырым markdown — как и Description задачи,
	// разметку разбирает фронтенд.
	Body string `gorm:"type:text;not null"`

	// EditedAt заполняется при первой правке: читателю важно понимать,
	// что текст менялся после отправки.
	EditedAt *time.Time
}

func (Comment) TableName() string { return "task_comments" }

// IsWrittenBy сообщает, автор ли указанный сотрудник.
func (c *Comment) IsWrittenBy(staffID uint) bool { return c.AuthorID == staffID }

// IsEdited сообщает, правился ли текст после отправки.
func (c *Comment) IsEdited() bool { return c.EditedAt != nil }

// NewComment собирает комментарий, проверив текст.
func NewComment(taskID, authorID uint, body string) (*Comment, error) {
	body = strings.TrimSpace(body)
	if err := validateCommentBody(body); err != nil {
		return nil, err
	}
	return &Comment{TaskID: taskID, AuthorID: authorID, Body: body}, nil
}

// Rewrite заменяет текст комментария и отмечает правку.
func (c *Comment) Rewrite(body string) error {
	body = strings.TrimSpace(body)
	if err := validateCommentBody(body); err != nil {
		return err
	}
	// Текст тот же — правкой это не считается, отметку не ставим.
	if c.Body == body {
		return nil
	}

	now := time.Now()
	c.Body = body
	c.EditedAt = &now
	return nil
}
