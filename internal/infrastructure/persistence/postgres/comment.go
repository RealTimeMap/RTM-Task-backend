package postgres

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/domain/task"
)

type CommentRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewCommentRepository(db *gorm.DB, log *zap.Logger) task.CommentRepository {
	return &CommentRepository{db: db, log: log.Named("comment_repository")}
}

func (r *CommentRepository) Create(ctx context.Context, obj *task.Comment) (*task.Comment, error) {
	if err := r.db.WithContext(ctx).Create(obj).Error; err != nil {
		r.log.Error("create comment failed", zap.Uint("task_id", obj.TaskID), zap.Error(err))
		return nil, task.ErrDatabaseQuery("create comment", err)
	}
	return obj, nil
}

func (r *CommentRepository) GetByID(ctx context.Context, id uint) (*task.Comment, error) {
	var obj task.Comment
	err := r.db.WithContext(ctx).First(&obj, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, task.ErrCommentNotFound(id)
		}
		r.log.Error("get comment failed", zap.Uint("id", id), zap.Error(err))
		return nil, task.ErrDatabaseQuery("get comment", err)
	}
	return &obj, nil
}

func (r *CommentRepository) ListByTask(ctx context.Context, taskID uint) ([]*task.Comment, error) {
	var objs []*task.Comment
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at ASC, id ASC").
		Find(&objs).Error
	if err != nil {
		r.log.Error("list comments failed", zap.Uint("task_id", taskID), zap.Error(err))
		return nil, task.ErrDatabaseQuery("list comments", err)
	}
	return objs, nil
}

// ListByTasks достаёт обсуждения набора задач одним запросом:
// список задач иначе выродился бы в запрос на каждую карточку.
func (r *CommentRepository) ListByTasks(ctx context.Context, taskIDs []uint) (map[uint][]*task.Comment, error) {
	grouped := make(map[uint][]*task.Comment, len(taskIDs))
	if len(taskIDs) == 0 {
		return grouped, nil
	}

	var objs []*task.Comment
	err := r.db.WithContext(ctx).
		Where("task_id IN ?", taskIDs).
		Order("created_at ASC, id ASC").
		Find(&objs).Error
	if err != nil {
		r.log.Error("list comments by tasks failed", zap.Error(err))
		return nil, task.ErrDatabaseQuery("list comments by tasks", err)
	}

	for _, obj := range objs {
		grouped[obj.TaskID] = append(grouped[obj.TaskID], obj)
	}
	return grouped, nil
}

// Update сохраняет текст комментария. Оптимистичной блокировки здесь нет:
// правит его только автор или менеджер, и гонка за одну реплику
// маловероятна настолько, что версия не окупает усложнения.
func (r *CommentRepository) Update(ctx context.Context, obj *task.Comment) (*task.Comment, error) {
	result := r.db.WithContext(ctx).
		Model(&task.Comment{}).
		Where("id = ?", obj.ID).
		// Колонки перечислены явно: карта не выводится из структуры,
		// и забытое поле молча не сохранится.
		Updates(map[string]any{
			"body":       obj.Body,
			"edited_at":  obj.EditedAt,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		r.log.Error("update comment failed", zap.Uint("id", obj.ID), zap.Error(result.Error))
		return nil, task.ErrDatabaseQuery("update comment", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, task.ErrCommentNotFound(obj.ID)
	}

	return r.GetByID(ctx, obj.ID)
}

func (r *CommentRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&task.Comment{}, id)
	if result.Error != nil {
		r.log.Error("delete comment failed", zap.Uint("id", id), zap.Error(result.Error))
		return task.ErrDatabaseQuery("delete comment", result.Error)
	}
	if result.RowsAffected == 0 {
		return task.ErrCommentNotFound(id)
	}
	return nil
}
