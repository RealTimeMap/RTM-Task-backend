package idea

import (
	"fmt"

	"RTM-Task/internal/utils/apperror"
)

var (
	ErrIdeaNotFound = func(id uint) error {
		return apperror.NewNotFoundError("idea", id)
	}

	ErrCommentNotFound = func(id uint) error {
		return apperror.NewNotFoundError("idea comment", id)
	}

	// ErrCommentForeign — комментарий относится к другой идее.
	//
	// Отдельно от «не найден»: запись существует, но адрес запроса
	// ей не соответствует, и отвечать 404 значило бы сбивать с толку.
	ErrCommentForeign = func(commentID, ideaID uint) error {
		return apperror.NewConflictError(
			"commentId",
			fmt.Sprintf("comment %d does not belong to idea %d", commentID, ideaID),
		)
	}

	ErrCommentBodyRequired = func() error {
		return apperror.NewRequiredError("body")
	}

	ErrCommentBodyTooLong = func(max int) error {
		return apperror.NewTooLongError("body", max, nil)
	}

	// ErrForbidden — роль не даёт права записи.
	//
	// Наблюдатель читает идеи наравне со всеми, но не заводит их и не
	// отмечает выполненными: список замыслов — рабочий инструмент
	// команды, а не гостевая книга.
	ErrForbidden = func() error {
		return apperror.NewForbiddenError("your role cannot modify ideas")
	}

	// ErrForeignIdea — правка или удаление чужой идеи.
	//
	// Текст замысла — слова его автора. Переписать или стереть их
	// может он сам либо управляющая роль, но не любой коллега.
	ErrForeignIdea = func() error {
		return apperror.NewForbiddenError("only the author or a manager can modify this idea")
	}

	// ErrForeignComment — правка чужой реплики.
	//
	// Автор текста — единственный, кто вправе его менять: иначе в
	// обсуждении нельзя было бы доверять ни одной подписи.
	ErrForeignComment = func() error {
		return apperror.NewForbiddenError("only the author can modify their comment")
	}

	ErrDatabaseQuery = func(operation string, cause error) error {
		return apperror.WrapInternalError(
			fmt.Sprintf("database %s failed", operation),
			cause,
		)
	}
)
