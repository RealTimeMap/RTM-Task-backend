package postgres

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/domain/task"
)

type ChecklistRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewChecklistRepository(db *gorm.DB, log *zap.Logger) task.ChecklistRepository {
	return &ChecklistRepository{db: db, log: log.Named("checklist_repository")}
}

func (r *ChecklistRepository) Create(ctx context.Context, obj *task.ChecklistItem) (*task.ChecklistItem, error) {
	if err := r.db.WithContext(ctx).Create(obj).Error; err != nil {
		r.log.Error("create checklist item failed", zap.Uint("task_id", obj.TaskID), zap.Error(err))
		return nil, task.ErrDatabaseQuery("create checklist item", err)
	}
	return obj, nil
}

func (r *ChecklistRepository) CreateMany(ctx context.Context, objs []*task.ChecklistItem) ([]*task.ChecklistItem, error) {
	if len(objs) == 0 {
		return nil, nil
	}
	if err := r.db.WithContext(ctx).Create(&objs).Error; err != nil {
		r.log.Error("create checklist items failed", zap.Int("count", len(objs)), zap.Error(err))
		return nil, task.ErrDatabaseQuery("create checklist items", err)
	}
	return objs, nil
}

func (r *ChecklistRepository) GetByID(ctx context.Context, id uint) (*task.ChecklistItem, error) {
	var obj task.ChecklistItem
	err := r.db.WithContext(ctx).First(&obj, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, task.ErrChecklistItemNotFound(id)
		}
		r.log.Error("get checklist item failed", zap.Uint("id", id), zap.Error(err))
		return nil, task.ErrDatabaseQuery("get checklist item", err)
	}
	return &obj, nil
}

func (r *ChecklistRepository) ListByTask(ctx context.Context, taskID uint) ([]*task.ChecklistItem, error) {
	var objs []*task.ChecklistItem
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("position ASC, id ASC").
		Find(&objs).Error
	if err != nil {
		r.log.Error("list checklist failed", zap.Uint("task_id", taskID), zap.Error(err))
		return nil, task.ErrDatabaseQuery("list checklist", err)
	}
	return objs, nil
}

func (r *ChecklistRepository) ListByTasks(ctx context.Context, taskIDs []uint) (map[uint][]*task.ChecklistItem, error) {
	grouped := make(map[uint][]*task.ChecklistItem, len(taskIDs))
	if len(taskIDs) == 0 {
		return grouped, nil
	}

	var objs []*task.ChecklistItem
	err := r.db.WithContext(ctx).
		Where("task_id IN ?", taskIDs).
		Order("position ASC, id ASC").
		Find(&objs).Error
	if err != nil {
		r.log.Error("list checklist by tasks failed", zap.Error(err))
		return nil, task.ErrDatabaseQuery("list checklist by tasks", err)
	}

	for _, obj := range objs {
		grouped[obj.TaskID] = append(grouped[obj.TaskID], obj)
	}
	return grouped, nil
}

func (r *ChecklistRepository) CountByTask(ctx context.Context, taskID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&task.ChecklistItem{}).
		Where("task_id = ?", taskID).
		Count(&count).Error
	if err != nil {
		r.log.Error("count checklist failed", zap.Uint("task_id", taskID), zap.Error(err))
		return 0, task.ErrDatabaseQuery("count checklist", err)
	}
	return count, nil
}

// Update сохраняет пункт. Колонки перечислены явно: сброс отметки
// записывает NULL, а Save по структуре пропустил бы нулевые значения.
func (r *ChecklistRepository) Update(ctx context.Context, obj *task.ChecklistItem) (*task.ChecklistItem, error) {
	result := r.db.WithContext(ctx).
		Model(&task.ChecklistItem{}).
		Where("id = ?", obj.ID).
		Updates(map[string]any{
			"title":      obj.Title,
			"position":   obj.Position,
			"done_at":    obj.DoneAt,
			"done_by_id": obj.DoneByID,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		r.log.Error("update checklist item failed", zap.Uint("id", obj.ID), zap.Error(result.Error))
		return nil, task.ErrDatabaseQuery("update checklist item", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, task.ErrChecklistItemNotFound(obj.ID)
	}

	return r.GetByID(ctx, obj.ID)
}

func (r *ChecklistRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&task.ChecklistItem{}, id)
	if result.Error != nil {
		r.log.Error("delete checklist item failed", zap.Uint("id", id), zap.Error(result.Error))
		return task.ErrDatabaseQuery("delete checklist item", result.Error)
	}
	if result.RowsAffected == 0 {
		return task.ErrChecklistItemNotFound(id)
	}
	return nil
}

// NextPosition возвращает позицию за последним пунктом.
//
// COALESCE нужен для пустого списка: MAX по нулю строк даёт NULL,
// а не ноль, и скан в int упал бы.
func (r *ChecklistRepository) NextPosition(ctx context.Context, taskID uint) (int, error) {
	var maxPosition int
	err := r.db.WithContext(ctx).
		Model(&task.ChecklistItem{}).
		Where("task_id = ?", taskID).
		Select("COALESCE(MAX(position), -1)").
		Scan(&maxPosition).Error
	if err != nil {
		r.log.Error("next checklist position failed", zap.Uint("task_id", taskID), zap.Error(err))
		return 0, task.ErrDatabaseQuery("next checklist position", err)
	}
	return maxPosition + 1, nil
}
