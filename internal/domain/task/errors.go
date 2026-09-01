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

	ErrNotClosed = func(id uint, status string) error {
		return apperror.NewConflictError(
			"status",
			fmt.Sprintf("task %d is in status %q, only a completed task can be sent to rework", id, status),
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
			"must be one of: bug, feature, fix, refactor, update",
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

	ErrReworkNoteRequired = func() error {
		return apperror.NewRequiredError("note")
	}

	ErrReworkNoteTooShort = func(min int) error {
		return apperror.NewValidationError(
			"note",
			fmt.Sprintf("must be at least %d characters, describe what to fix", min),
			"value_error.any_str.min_length",
			min,
		)
	}

	ErrReworkNoteTooLong = func(max int) error {
		return apperror.NewValidationError(
			"note",
			fmt.Sprintf("must be at most %d characters", max),
			"value_error.any_str.max_length",
			max,
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

	ErrReworkForbidden = func() error {
		return apperror.NewForbiddenError(
			"only the creator, the assignee or a manager can send this task to rework",
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
