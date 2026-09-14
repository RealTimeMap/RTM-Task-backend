package postgres

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/domain/idea"
)

type IdeaRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewIdeaRepository(db *gorm.DB, log *zap.Logger) idea.Repository {
	return &IdeaRepository{db: db, log: log.Named("idea_repository")}
}

func (r *IdeaRepository) Create(ctx context.Context, obj *idea.Idea) (*idea.Idea, error) {
	if err := r.db.WithContext(ctx).Create(obj).Error; err != nil {
		r.log.Error("create idea failed", zap.Uint("author_id", obj.AuthorID), zap.Error(err))
		return nil, idea.ErrDatabaseQuery("create idea", err)
	}
	return obj, nil
}

// GetByID отдаёт nil без ошибки, если записи нет: решение, считать ли
// это ошибкой, принимает доменный сервис — в разных операциях оно разное.
func (r *IdeaRepository) GetByID(ctx context.Context, id uint) (*idea.Idea, error) {
	var obj idea.Idea
	err := r.db.WithContext(ctx).First(&obj, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		r.log.Error("get idea failed", zap.Uint("id", id), zap.Error(err))
		return nil, idea.ErrDatabaseQuery("get idea", err)
	}
	return &obj, nil
}

func (r *IdeaRepository) List(ctx context.Context, filter idea.Filter) ([]*idea.Idea, int64, error) {
	query := r.db.WithContext(ctx).Model(&idea.Idea{})

	if filter.Done != nil {
		query = query.Where("done = ?", *filter.Done)
	}
	if filter.AuthorID != 0 {
		query = query.Where("author_id = ?", filter.AuthorID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		r.log.Error("count ideas failed", zap.Error(err))
		return nil, 0, idea.ErrDatabaseQuery("count ideas", err)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Невыполненные идут первыми, внутри — свежие сверху: список
	// открывают, чтобы посмотреть, что ещё не сделано, а сделанное
	// остаётся историей под ними.
	var objs []*idea.Idea
	if err := query.Order("done ASC, created_at DESC, id DESC").Find(&objs).Error; err != nil {
		r.log.Error("list ideas failed", zap.Error(err))
		return nil, 0, idea.ErrDatabaseQuery("list ideas", err)
	}

	return objs, total, nil
}

func (r *IdeaRepository) Update(ctx context.Context, obj *idea.Idea) (*idea.Idea, error) {
	// Save, а не Updates: отметка выполнения обнуляет done_at и
	// done_by_id при возврате идеи в работу, а Updates пропускает
	// нулевые значения и оставил бы в базе старую подпись.
	if err := r.db.WithContext(ctx).Save(obj).Error; err != nil {
		r.log.Error("update idea failed", zap.Uint("id", obj.ID), zap.Error(err))
		return nil, idea.ErrDatabaseQuery("update idea", err)
	}
	return obj, nil
}

func (r *IdeaRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&idea.Idea{}, id).Error; err != nil {
		r.log.Error("delete idea failed", zap.Uint("id", id), zap.Error(err))
		return idea.ErrDatabaseQuery("delete idea", err)
	}
	return nil
}
