package task

import (
	"context"

	"RTM-Task/internal/utils/pagination"
)

// GetByID возвращает задачу по идентификатору.
func (s *Service) GetByID(ctx context.Context, id uint) (*Task, error) {
	return s.load(ctx, id)
}

// List возвращает отфильтрованный список задач и общее количество совпадений.
func (s *Service) List(ctx context.Context, filter Filter) ([]*Task, int64, error) {
	if filter.Status != nil && !filter.Status.IsValid() {
		return nil, 0, ErrInvalidStatus(filter.Status.String())
	}
	if filter.Type != nil && !filter.Type.IsValid() {
		return nil, 0, ErrInvalidType(filter.Type.String())
	}
	if filter.Priority != nil && !filter.Priority.IsValid() {
		return nil, 0, ErrInvalidPriority(filter.Priority.Int())
	}

	// Нормализация на случай, если фильтр собран в обход конструктора.
	filter.Pagination = pagination.New(filter.Pagination.Limit, filter.Pagination.Offset)

	return s.repo.List(ctx, filter)
}
