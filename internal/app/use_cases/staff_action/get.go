package staff_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
)

type GetStaffQuery struct {
	StaffID uint
}

// StaffGetter — часть домена, нужная для чтения сотрудника.
type StaffGetter interface {
	GetByID(ctx context.Context, id uint) (*role.Staff, error)
}

type GetStaffHandler struct {
	staff  StaffGetter
	logger *zap.Logger
}

func NewGetStaffHandler(staff StaffGetter, logger *zap.Logger) *GetStaffHandler {
	return &GetStaffHandler{staff: staff, logger: logger}
}

func (h *GetStaffHandler) Handle(ctx context.Context, query GetStaffQuery) (StaffResult, error) {
	obj, err := h.staff.GetByID(ctx, query.StaffID)
	if err != nil {
		return StaffResult{}, err
	}
	return toStaffResult(obj), nil
}

// StaffLister — часть домена, нужная для выборки сотрудников.
type StaffLister interface {
	List(ctx context.Context) ([]*role.Staff, error)
}

type ListStaffHandler struct {
	staff  StaffLister
	logger *zap.Logger
}

func NewListStaffHandler(staff StaffLister, logger *zap.Logger) *ListStaffHandler {
	return &ListStaffHandler{staff: staff, logger: logger}
}

func (h *ListStaffHandler) Handle(ctx context.Context) ([]StaffResult, error) {
	objs, err := h.staff.List(ctx)
	if err != nil {
		return nil, err
	}
	return toStaffResults(objs), nil
}
