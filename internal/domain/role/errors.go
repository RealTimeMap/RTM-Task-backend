package role

import (
	"fmt"

	"RTM-Task/internal/utils/apperror"
)

var (
	ErrStaffNotFound = func(id any) error {
		return apperror.NewNotFoundError("staff", id)
	}

	ErrStaffInactive = func(id uint) error {
		return apperror.NewForbiddenError(
			fmt.Sprintf("staff %d is deactivated", id),
		)
	}

	ErrEmailAlreadyUsed = func(email string) error {
		return apperror.NewConflictError("email", fmt.Sprintf("email %s is already registered", email))
	}

	ErrInvalidRole = func(value string) error {
		return apperror.NewValidationError(
			"role",
			"must be one of: admin, manager, developer, viewer",
			"value_error.invalid_choice",
			value,
		)
	}

	ErrInvalidEmail = func(value string) error {
		return apperror.NewValidationError(
			"email",
			"must be a valid email address",
			"value_error.email",
			value,
		)
	}
)
