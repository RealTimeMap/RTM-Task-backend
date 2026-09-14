package idea

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Comment — обсуждение идеи.
//
// Устроен так же, как комментарий задачи, но хранится отдельно:
// общая таблица потребовала бы полиморфной ссылки, а с ней — потери
// внешнего ключа и проверок целостности на стороне базы.
type Comment struct {
	gorm.Model

	IdeaID   uint `gorm:"not null;index"`
	AuthorID uint `gorm:"not null;index"`

	// Body хранится сырым markdown — разметку разбирает фронтенд.
	Body string `gorm:"type:text;not null"`

	// EditedAt заполняется при первой правке: читателю важно понимать,
	// что текст менялся после отправки.
	EditedAt *time.Time
}

func (Comment) TableName() string { return "idea_comments" }

// IsWrittenBy сообщает, автор ли указанный сотрудник.
func (c *Comment) IsWrittenBy(staffID uint) bool { return c.AuthorID == staffID }

// IsEdited сообщает, правился ли текст после отправки.
func (c *Comment) IsEdited() bool { return c.EditedAt != nil }

// NewComment собирает комментарий, проверив текст.
func NewComment(ideaID, authorID uint, body string) (*Comment, error) {
	body = strings.TrimSpace(body)
	if err := ValidateCommentBody(body); err != nil {
		return nil, err
	}
	return &Comment{IdeaID: ideaID, AuthorID: authorID, Body: body}, nil
}

// Rewrite заменяет текст комментария и отмечает правку.
func (c *Comment) Rewrite(body string) error {
	body = strings.TrimSpace(body)
	if err := ValidateCommentBody(body); err != nil {
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

// ValidateCommentBody проверяет текст комментария.
func ValidateCommentBody(body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrCommentBodyRequired()
	}
	if len([]rune(body)) > MaxCommentSize {
		return ErrCommentBodyTooLong(MaxCommentSize)
	}
	return nil
}
