package postgres

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/domain/task"
)

type TaskRepository struct {
	db    *gorm.DB
	log   *zap.Logger
	layer string
}

func NewTaskRepository(db *gorm.DB, log *zap.Logger) task.Repository {
	return &TaskRepository{db: db, log: log.Named("task_repository"), layer: "task_repository"}
}

func (r *TaskRepository) Create(ctx context.Context, obj *task.Task) (*task.Task, error) {
	if err := r.db.WithContext(ctx).Create(obj).Error; err != nil {
		r.log.Error("create task failed", zap.Error(err))
		return nil, task.ErrDatabaseQuery("create task", err)
	}
	return obj, nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id uint) (*task.Task, error) {
	var obj task.Task
	err := r.db.WithContext(ctx).First(&obj, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, task.ErrTaskNotFound(id)
		}
		r.log.Error("get task by id failed", zap.Uint("id", id), zap.Error(err))
		return nil, task.ErrDatabaseQuery("get task", err)
	}
	return &obj, nil
}

func (r *TaskRepository) List(ctx context.Context, filter task.Filter) ([]*task.Task, int64, error) {
	query := r.applyFilter(r.db.WithContext(ctx).Model(&task.Task{}), filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		r.log.Error("count tasks failed", zap.Error(err))
		return nil, 0, task.ErrDatabaseQuery("count tasks", err)
	}

	var objs []*task.Task
	err := query.
		Order("priority ASC, created_at DESC").
		Limit(filter.Pagination.Limit).
		Offset(filter.Pagination.Offset).
		Find(&objs).Error
	if err != nil {
		r.log.Error("list tasks failed", zap.Error(err))
		return nil, 0, task.ErrDatabaseQuery("list tasks", err)
	}

	return objs, total, nil
}

// applyFilter переводит доменный фильтр в условия запроса.
func (r *TaskRepository) applyFilter(query *gorm.DB, filter task.Filter) *gorm.DB {
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.Priority != nil {
		query = query.Where("priority = ?", *filter.Priority)
	}
	if filter.CreatorID != nil {
		query = query.Where("creator_id = ?", *filter.CreatorID)
	}
	if filter.OnlyUnassigned {
		query = query.Where("assignee_id IS NULL")
	} else if filter.AssigneeID != nil {
		query = query.Where("assignee_id = ?", *filter.AssigneeID)
	}
	return query
}

func (r *TaskRepository) Exists(ctx context.Context, id uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&task.Task{}).
		Where("id = ?", id).
		Limit(1).
		Count(&count).Error
	if err != nil {
		r.log.Error("check task exists failed", zap.Uint("id", id), zap.Error(err))
		return false, task.ErrDatabaseQuery("check task exists", err)
	}
	return count > 0, nil
}

// Update сохраняет задачу через compare-and-swap по версии: запись меняется
// только если её версия в базе всё ещё равна expectedVersion.
func (r *TaskRepository) Update(ctx context.Context, obj *task.Task, expectedVersion int) (*task.Task, error) {
	result := r.db.WithContext(ctx).
		Model(&task.Task{}).
		Where("id = ? AND version = ?", obj.ID, expectedVersion).
		Updates(map[string]any{
			"title":       obj.Title,
			"description": obj.Description,
			"type":        obj.Type,
			"status":      obj.Status,
			"priority":    obj.Priority,
			"assignee_id": obj.AssigneeID,
			"closed_at":   obj.ClosedAt,
			// Поля доработки перечислены явно: карта колонок не выводится
			// из структуры, и забытое поле молча не сохранится.
			"rework_note":  obj.ReworkNote,
			"rework_by_id": obj.ReworkByID,
			"rework_at":    obj.ReworkAt,
			"version":      obj.Version,
			"updated_at":   time.Now(),
		})

	if result.Error != nil {
		r.log.Error("update task failed", zap.Uint("id", obj.ID), zap.Error(result.Error))
		return nil, task.ErrDatabaseQuery("update task", result.Error)
	}

	// Ноль затронутых строк — либо запись исчезла, либо её успели изменить.
	if result.RowsAffected == 0 {
		exists, err := r.Exists(ctx, obj.ID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, task.ErrTaskNotFound(obj.ID)
		}
		return nil, task.ErrVersionConflict(obj.ID)
	}

	return r.GetByID(ctx, obj.ID)
}

func (r *TaskRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&task.Task{}, id)
	if result.Error != nil {
		r.log.Error("delete task failed", zap.Uint("id", id), zap.Error(result.Error))
		return task.ErrDatabaseQuery("delete task", result.Error)
	}
	if result.RowsAffected == 0 {
		return task.ErrTaskNotFound(id)
	}
	return nil
}

func (r *TaskRepository) TodayCreated(ctx context.Context, creatorID uint) (int64, error) {
	startOfDay := time.Now().Truncate(24 * time.Hour)

	var count int64
	err := r.db.WithContext(ctx).
		Model(&task.Task{}).
		Where("creator_id = ? AND created_at >= ?", creatorID, startOfDay).
		Count(&count).Error
	if err != nil {
		r.log.Error("count today tasks failed", zap.Uint("creator_id", creatorID), zap.Error(err))
		return 0, task.ErrDatabaseQuery("count today tasks", err)
	}
	return count, nil
}
