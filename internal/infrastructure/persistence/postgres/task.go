package postgres

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
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
		Order(orderClause(filter.Sort)).
		Limit(filter.Pagination.Limit).
		Offset(filter.Pagination.Offset).
		Find(&objs).Error
	if err != nil {
		r.log.Error("list tasks failed", zap.Error(err))
		return nil, 0, task.ErrDatabaseQuery("list tasks", err)
	}

	return objs, total, nil
}

// orderClause строит ORDER BY по доменному порядку сортировки.
//
// Выражение собирается из констант, а не из пришедшей строки: поле и
// направление уже проверены доменом, но склеивать SQL с внешним вводом
// нельзя даже после проверки.
func orderClause(sort task.Sort) string {
	sort = sort.Normalize()
	if sort.IsZero() {
		// Порядок по умолчанию: сначала важные, внутри — свежие.
		return "priority ASC, created_at DESC"
	}

	direction := "ASC"
	if sort.Order == task.DescOrder {
		direction = "DESC"
	}

	var expr string
	switch sort.Field {
	case task.SortByCreatedAt:
		expr = "created_at"
	case task.SortByPriority:
		expr = "priority"
	case task.SortByStatus:
		expr = statusOrderExpr
	case task.SortByType:
		expr = typeOrderExpr
	default:
		return "priority ASC, created_at DESC"
	}

	// Вторым ключом всегда id: без него страницы с одинаковыми
	// значениями поля могли бы перемешиваться между запросами.
	return expr + " " + direction + ", id DESC"
}

// statusOrderExpr и typeOrderExpr задают смысловой порядок вместо
// алфавитного: по алфавиту «complete» оказался бы раньше «new», а
// жизненный цикл идёт new → working → review → complete.
const (
	statusOrderExpr = `CASE status ` +
		`WHEN 'new' THEN 1 ` +
		`WHEN 'working' THEN 2 ` +
		`WHEN 'review' THEN 3 ` +
		`WHEN 'complete' THEN 4 ` +
		`ELSE 5 END`

	typeOrderExpr = `CASE type ` +
		`WHEN 'bug' THEN 1 ` +
		`WHEN 'feature' THEN 2 ` +
		`WHEN 'fix' THEN 3 ` +
		`WHEN 'refactor' THEN 4 ` +
		`WHEN 'update' THEN 5 ` +
		`ELSE 6 END`
)

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
	if filter.Project != nil {
		query = query.Where("project = ?", *filter.Project)
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
			"project":     obj.Project,
			"assignee_id": obj.AssigneeID,
			// Привязка бага меняется вместе с задачей и должна уметь
			// сбрасываться в NULL — поэтому она в этой карте, а не
			// выводится из ненулевых полей структуры.
			"bug_id": obj.BugID,
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

// Delete удаляет задачу вместе с её обсуждением и чек-листом.
//
// Дочерние записи убираются здесь, а не каскадом на уровне схемы:
// внешних ключей у нас нет, и осиротевшие комментарии остались бы
// в базе навсегда. Всё идёт одной транзакцией — задача без обсуждения
// или обсуждение без задачи одинаково бессмысленны.
func (r *TaskRepository) Delete(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&task.Task{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return task.ErrTaskNotFound(id)
		}

		if err := tx.Where("task_id = ?", id).Delete(&task.Comment{}).Error; err != nil {
			return err
		}
		return tx.Where("task_id = ?", id).Delete(&task.ChecklistItem{}).Error
	})

	if err != nil {
		// Доменную ошибку пробрасываем как есть: это не сбой запроса,
		// а осмысленный ответ («задачи нет»).
		if _, ok := apperror.As(err); ok {
			return err
		}
		r.log.Error("delete task failed", zap.Uint("id", id), zap.Error(err))
		return task.ErrDatabaseQuery("delete task", err)
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
