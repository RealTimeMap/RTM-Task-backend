package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Kind — категория ошибки. Определяет HTTP-статус на транспортном уровне,
// благодаря чему домен остаётся независимым от HTTP.
type Kind string

const (
	KindValidation   Kind = "validation"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindInternal     Kind = "internal"

	// KindUnavailable — сбой не у нас, а у сервиса, от которого мы
	// зависим. Отдельно от KindInternal: «мы сломались» и «сосед не
	// отвечает» требуют разной реакции — второе лечится повтором,
	// и клиенту стоит сказать об этом прямо, а не показывать
	// общую ошибку сервера.
	KindUnavailable Kind = "unavailable"
)

// AppError — ошибка приложения, переносимая между слоями без потери смысла.
type AppError struct {
	Kind    Kind
	Code    string
	Message string
	Field   string
	Value   any
	cause   error
}

func (e *AppError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s: %s", e.Kind, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *AppError) Unwrap() error { return e.cause }

// HTTPStatus переводит категорию ошибки в HTTP-статус.
func (e *AppError) HTTPStatus() int {
	switch e.Kind {
	case KindValidation:
		return http.StatusBadRequest
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// As извлекает *AppError из цепочки ошибок.
func As(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

func NewValidationError(field, message, code string, value any) error {
	return &AppError{
		Kind:    KindValidation,
		Code:    code,
		Message: message,
		Field:   field,
		Value:   value,
	}
}

func NewRequiredError(field string) error {
	return NewValidationError(field, "field is required", "value_error.missing", nil)
}

func NewTooLongError(field string, max int, value any) error {
	return NewValidationError(
		field,
		fmt.Sprintf("must be at most %d characters", max),
		"value_error.any_str.max_length",
		value,
	)
}

func NewTooShortError(field string, min int, value any) error {
	return NewValidationError(
		field,
		fmt.Sprintf("must be at least %d characters", min),
		"value_error.any_str.min_length",
		value,
	)
}

func NewNotFoundError(resource string, id any) error {
	return &AppError{
		Kind:    KindNotFound,
		Code:    "not_found",
		Message: fmt.Sprintf("%s not found", resource),
		Value:   id,
	}
}

func NewConflictError(resource, message string) error {
	return &AppError{
		Kind:    KindConflict,
		Code:    "conflict",
		Message: message,
		Field:   resource,
	}
}

func NewUnauthorizedError(message string) error {
	return &AppError{
		Kind:    KindUnauthorized,
		Code:    "unauthorized",
		Message: message,
	}
}

func NewForbiddenError(message string) error {
	return &AppError{
		Kind:    KindForbidden,
		Code:    "forbidden",
		Message: message,
	}
}

func WrapInternalError(message string, cause error) error {
	return &AppError{
		Kind:    KindInternal,
		Code:    "internal_error",
		Message: message,
		cause:   cause,
	}
}

// NewUnavailableError сообщает, что сервис, от которого мы зависим,
// сейчас недоступен.
//
// service попадает в Field: клиенту важно знать, чего именно сейчас нет,
// чтобы сказать об этом человеку, а не показывать общий отказ.
func NewUnavailableError(service, message string, cause error) error {
	return &AppError{
		Kind:    KindUnavailable,
		Code:    "service_unavailable",
		Message: message,
		Field:   service,
		cause:   cause,
	}
}
