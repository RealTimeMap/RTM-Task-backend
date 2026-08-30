package staff_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
)

type ChangeRoleCommand struct {
	Actor   role.Actor
	StaffID uint
	Role    string
}

func (c ChangeRoleCommand) Validate() error {
	if c.Role == "" {
		return apperror.NewRequiredError("role")
	}
	return nil
}

// RoleChanger — часть домена, меняющая роль сотрудника.
type RoleChanger interface {
	ChangeRole(ctx context.Context, actor role.Actor, staffID uint, newRole role.Role) (*role.Staff, error)
}

type ChangeRoleHandler struct {
	staff  RoleChanger
	logger *zap.Logger
}

func NewChangeRoleHandler(staff RoleChanger, logger *zap.Logger) *ChangeRoleHandler {
	return &ChangeRoleHandler{staff: staff, logger: logger}
}

func (h *ChangeRoleHandler) Handle(ctx context.Context, cmd ChangeRoleCommand) (StaffResult, error) {
	if err := cmd.Validate(); err != nil {
		return StaffResult{}, err
	}

	obj, err := h.staff.ChangeRole(ctx, cmd.Actor, cmd.StaffID, role.Role(cmd.Role))
	if err != nil {
		return StaffResult{}, err
	}
	return toStaffResult(obj), nil
}

type DeactivateStaffCommand struct {
	Actor   role.Actor
	StaffID uint
}

// StaffDeactivator — часть домена, отключающая сотрудника.
type StaffDeactivator interface {
	Deactivate(ctx context.Context, actor role.Actor, staffID uint) (*role.Staff, error)
}

type DeactivateStaffHandler struct {
	staff  StaffDeactivator
	logger *zap.Logger
}

func NewDeactivateStaffHandler(staff StaffDeactivator, logger *zap.Logger) *DeactivateStaffHandler {
	return &DeactivateStaffHandler{staff: staff, logger: logger}
}

func (h *DeactivateStaffHandler) Handle(ctx context.Context, cmd DeactivateStaffCommand) (StaffResult, error) {
	obj, err := h.staff.Deactivate(ctx, cmd.Actor, cmd.StaffID)
	if err != nil {
		return StaffResult{}, err
	}
	return toStaffResult(obj), nil
}
