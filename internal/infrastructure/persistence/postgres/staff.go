package postgres

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
)

type StaffRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewStaffRepository(db *gorm.DB, log *zap.Logger) role.Repository {
	return &StaffRepository{db: db, log: log.Named("staff_repository")}
}

func (r *StaffRepository) Create(ctx context.Context, staff *role.Staff) (*role.Staff, error) {
	err := r.db.WithContext(ctx).Create(staff).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			email := ""
			if staff.Email != nil {
				email = *staff.Email
			}
			return nil, role.ErrEmailAlreadyUsed(email)
		}
		r.log.Error("create staff failed", zap.Error(err))
		return nil, apperror.WrapInternalError("database create staff failed", err)
	}
	return staff, nil
}

func (r *StaffRepository) GetByID(ctx context.Context, id uint) (*role.Staff, error) {
	var obj role.Staff
	err := r.db.WithContext(ctx).First(&obj, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, role.ErrStaffNotFound(id)
		}
		r.log.Error("get staff by id failed", zap.Uint("id", id), zap.Error(err))
		return nil, apperror.WrapInternalError("database get staff failed", err)
	}
	return &obj, nil
}

func (r *StaffRepository) GetByEmail(ctx context.Context, email string) (*role.Staff, error) {
	var obj role.Staff
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&obj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, role.ErrStaffNotFound(email)
		}
		r.log.Error("get staff by email failed", zap.Error(err))
		return nil, apperror.WrapInternalError("database get staff failed", err)
	}
	return &obj, nil
}

func (r *StaffRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&role.Staff{}).
		Where("email = ?", email).
		Limit(1).
		Count(&count).Error
	if err != nil {
		r.log.Error("check staff email failed", zap.Error(err))
		return false, apperror.WrapInternalError("database check staff email failed", err)
	}
	return count > 0, nil
}

func (r *StaffRepository) Update(ctx context.Context, staff *role.Staff) (*role.Staff, error) {
	err := r.db.WithContext(ctx).Save(staff).Error
	if err != nil {
		r.log.Error("update staff failed", zap.Uint("id", staff.ID), zap.Error(err))
		return nil, apperror.WrapInternalError("database update staff failed", err)
	}
	return staff, nil
}

func (r *StaffRepository) List(ctx context.Context) ([]*role.Staff, error) {
	var objs []*role.Staff
	err := r.db.WithContext(ctx).Order("full_name ASC").Find(&objs).Error
	if err != nil {
		r.log.Error("list staff failed", zap.Error(err))
		return nil, apperror.WrapInternalError("database list staff failed", err)
	}
	return objs, nil
}
