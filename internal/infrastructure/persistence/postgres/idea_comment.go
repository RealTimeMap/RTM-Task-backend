package postgres

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/domain/idea"
)

type IdeaCommentRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewIdeaCommentRepository(db *gorm.DB, log *zap.Logger) idea.CommentRepository {
	return &IdeaCommentRepository{db: db, log: log.Named("idea_comment_repository")}
}

func (r *IdeaCommentRepository) Create(ctx context.Context, obj *idea.Comment) (*idea.Comment, error) {
	if err := r.db.WithContext(ctx).Create(obj).Error; err != nil {
		r.log.Error("create idea comment failed", zap.Uint("idea_id", obj.IdeaID), zap.Error(err))
		return nil, idea.ErrDatabaseQuery("create idea comment", err)
	}
	return obj, nil
}

// GetByID отдаёт nil без ошибки, если реплики нет: сервис сам решает,
// чем это считать — она может принадлежать другой идее.
func (r *IdeaCommentRepository) GetByID(ctx context.Context, id uint) (*idea.Comment, error) {
	var obj idea.Comment
	err := r.db.WithContext(ctx).First(&obj, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		r.log.Error("get idea comment failed", zap.Uint("id", id), zap.Error(err))
		return nil, idea.ErrDatabaseQuery("get idea comment", err)
	}
	return &obj, nil
}

func (r *IdeaCommentRepository) ListByIdea(ctx context.Context, ideaID uint) ([]*idea.Comment, error) {
	var objs []*idea.Comment
	err := r.db.WithContext(ctx).
		Where("idea_id = ?", ideaID).
		Order("created_at ASC, id ASC").
		Find(&objs).Error
	if err != nil {
		r.log.Error("list idea comments failed", zap.Uint("idea_id", ideaID), zap.Error(err))
		return nil, idea.ErrDatabaseQuery("list idea comments", err)
	}
	return objs, nil
}

// CountByIdeas считает реплики сразу по набору идей: счётчик на
// карточке иначе стоил бы запроса на каждую строку списка.
func (r *IdeaCommentRepository) CountByIdeas(ctx context.Context, ideaIDs []uint) (map[uint]int, error) {
	counts := make(map[uint]int, len(ideaIDs))
	if len(ideaIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		IdeaID uint
		Total  int
	}

	err := r.db.WithContext(ctx).
		Model(&idea.Comment{}).
		Select("idea_id, count(*) as total").
		Where("idea_id IN ?", ideaIDs).
		Group("idea_id").
		Scan(&rows).Error
	if err != nil {
		r.log.Error("count idea comments failed", zap.Error(err))
		return nil, idea.ErrDatabaseQuery("count idea comments", err)
	}

	for _, row := range rows {
		counts[row.IdeaID] = row.Total
	}
	return counts, nil
}

func (r *IdeaCommentRepository) Update(ctx context.Context, obj *idea.Comment) (*idea.Comment, error) {
	if err := r.db.WithContext(ctx).Save(obj).Error; err != nil {
		r.log.Error("update idea comment failed", zap.Uint("id", obj.ID), zap.Error(err))
		return nil, idea.ErrDatabaseQuery("update idea comment", err)
	}
	return obj, nil
}

func (r *IdeaCommentRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&idea.Comment{}, id).Error; err != nil {
		r.log.Error("delete idea comment failed", zap.Uint("id", id), zap.Error(err))
		return idea.ErrDatabaseQuery("delete idea comment", err)
	}
	return nil
}

func (r *IdeaCommentRepository) DeleteByIdea(ctx context.Context, ideaID uint) error {
	err := r.db.WithContext(ctx).
		Where("idea_id = ?", ideaID).
		Delete(&idea.Comment{}).Error
	if err != nil {
		r.log.Error("delete idea comments failed", zap.Uint("idea_id", ideaID), zap.Error(err))
		return idea.ErrDatabaseQuery("delete idea comments", err)
	}
	return nil
}
