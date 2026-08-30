package task

import (
	"fmt"

	"RTM-Task/internal/utils/apperror"
)

// Ошибки жизненного цикла и владения.
var (
	ErrTaskNotFound = func(id uint) error {
		return apperror.NewNotFoundError("task", id)
	}

	ErrTaskClosed = func(id uint) error {
		return apperror.NewConflictError(
			"status",
			fmt.Sprintf("task %d is closed and cannot be modified", id),
		)
	}

	ErrInvalidTransition = func(from, to string) error {
		return apperror.NewConflictError(
			"status",
			fmt.Sprintf("transition from %q to %q is not allowed", from, to),
		)
	}

	ErrSameStatus = func(status string) error {
		return apperror.NewConflictError(
			"status",
			fmt.Sprintf("task already has status %q", status),
		)
	}

	ErrAssigneeRequired = func() error {
		return apperror.NewConflictError(
			"assigneeId",
			"task must have an assignee before moving to work",
		)
	}

	ErrAlreadyAssigned = func(staffID uint) error {
		return apperror.NewConflictError(
			"assigneeId",
			fmt.Sprintf("task is already assigned to staff %d", staffID),
		)
	}

	ErrNotAssigned = func(id uint) error {
		return apperror.NewConflictError(
			"assigneeId",
			fmt.Sprintf("task %d has no assignee", id),
		)
	}

	ErrUnassignActiveTask = func(status string) error {
		return apperror.NewConflictError(
			"assigneeId",
			fmt.Sprintf("cannot unassign task in status %q, move it back to new first", status),
		)
	}

	ErrVersionConflict = func(id uint) error {
		return apperror.NewConflictError(
			"version",
			fmt.Sprintf("task %d was modified by someone else, reload and retry", id),
		)
	}
)

// Ошибки валидации входных данных.
var (
	ErrInvalidStatus = func(value string) error {
		return apperror.NewValidationError(
			"status",
			"must be one of: new, working, review, complete",
			"value_error.invalid_choice",
			value,
		)
	}

	ErrInvalidType = func(value string) error {
		return apperror.NewValidationError(
			"type",
			"must be one of: bug, feature, fix",
			"value_error.invalid_choice",
			value,
		)
	}

	ErrInvalidPriority = func(value int) error {
		return apperror.NewValidationError(
			"priority",
			"must be one of: 10, 20, 30, 40",
			"value_error.invalid_choice",
			value,
		)
	}

	ErrDailyLimitReached = func(limit int) error {
		return apperror.NewConflictError(
			"task",
			fmt.Sprintf("daily limit of %d tasks reached", limit),
		)
	}
)

// Ошибки прав доступа.
var (
	ErrCreateForbidden = func(role string) error {
		return apperror.NewForbiddenError(
			fmt.Sprintf("role %q is not allowed to create tasks", role),
		)
	}

	ErrEditForbidden = func() error {
		return apperror.NewForbiddenError(
			"only the creator, the assignee or a manager can edit this task",
		)
	}

	ErrAssignForbidden = func() error {
		return apperror.NewForbiddenError(
			"only a manager can assign a task to another staff member",
		)
	}

	ErrDeleteForbidden = func() error {
		return apperror.NewForbiddenError(
			"only the creator or a manager can delete this task",
		)
	}

	ErrCloseForbidden = func() error {
		return apperror.NewForbiddenError(
			"only the assignee or a manager can close this task",
		)
	}
)

// Инфраструктурные ошибки.
var (
	ErrDatabaseQuery = func(operation string, cause error) error {
		return apperror.WrapInternalError(
			fmt.Sprintf("database %s failed", operation),
			cause,
		)
	}
)
