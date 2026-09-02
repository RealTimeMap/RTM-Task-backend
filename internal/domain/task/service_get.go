package task

import (
	"context"

	"RTM-Task/internal/utils/pagination"
)

// GetByID возвращает задачу по идентификатору.
func (s *Service) GetByID(ctx context.Context, id uint) (*Task, error) {
	return s.load(ctx, id)
}

// Summary — счётчики дочерних записей задачи. Их показывает карточка:
// «3 из 5 пунктов, 2 комментария» видно, не открывая задачу.
type Summary struct {
	ChecklistTotal int
	ChecklistDone  int
	CommentCount   int
}

// SummaryFor собирает счётчики для набора задач за два запроса.
//
// Отдельный метод, а не поля агрегата: комментарии и пункты живут своими
// таблицами, и подгружать их в каждую задачу означало бы тянуть весь
// текст обсуждения ради одного числа.
func (s *Service) SummaryFor(ctx context.Context, objs []*Task) (map[uint]Summary, error) {
	summaries := make(map[uint]Summary, len(objs))
	if len(objs) == 0 {
		return summaries, nil
	}

	ids := make([]uint, 0, len(objs))
	for _, obj := range objs {
		ids = append(ids, obj.ID)
	}

	items, err := s.checklist.ListByTasks(ctx, ids)
	if err != nil {
		return nil, err
	}
	comments, err := s.comments.ListByTasks(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		progress := NewChecklistProgress(items[id])
		summaries[id] = Summary{
			ChecklistTotal: progress.Total,
			ChecklistDone:  progress.Done,
			CommentCount:   len(comments[id]),
		}
	}
	return summaries, nil
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
	if !filter.Sort.IsZero() {
		if !filter.Sort.Field.IsValid() {
			return nil, 0, ErrInvalidSortField(filter.Sort.Field.String())
		}
		if filter.Sort.Order != "" && !filter.Sort.Order.IsValid() {
			return nil, 0, ErrInvalidSortOrder(filter.Sort.Order.String())
		}
	}

	// Нормализация на случай, если фильтр собран в обход конструктора.
	filter.Pagination = pagination.New(filter.Pagination.Limit, filter.Pagination.Offset)

	return s.repo.List(ctx, filter)
}
