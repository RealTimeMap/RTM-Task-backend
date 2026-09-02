package task

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
)

// ListChecklist возвращает пункты чек-листа задачи по порядку.
func (s *Service) ListChecklist(ctx context.Context, taskID uint) ([]*ChecklistItem, error) {
	if _, err := s.load(ctx, taskID); err != nil {
		return nil, err
	}
	return s.checklist.ListByTask(ctx, taskID)
}

// AddChecklistItem добавляет пункт в конец чек-листа.
func (s *Service) AddChecklistItem(ctx context.Context, actor role.Actor, taskID uint, title string) (*ChecklistItem, error) {
	obj, err := s.load(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !canEdit(obj, actor) {
		return nil, ErrChecklistForbidden()
	}
	if err := s.ensureChecklistCapacity(ctx, taskID, 1); err != nil {
		return nil, err
	}

	position, err := s.checklist.NextPosition(ctx, taskID)
	if err != nil {
		return nil, err
	}

	item, err := NewChecklistItem(taskID, title, position)
	if err != nil {
		return nil, err
	}

	created, err := s.checklist.Create(ctx, item)
	if err != nil {
		return nil, err
	}

	s.logger.Info("checklist item added",
		zap.Uint("task_id", taskID),
		zap.Uint("item_id", created.ID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return created, nil
}

// UpdateChecklistItem меняет текст пункта и/или отметку выполнения.
// Оба поля опциональны: nil означает «не трогать».
func (s *Service) UpdateChecklistItem(
	ctx context.Context,
	actor role.Actor,
	taskID, itemID uint,
	title *string,
	done *bool,
) (*ChecklistItem, error) {
	obj, err := s.load(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !canEdit(obj, actor) {
		return nil, ErrChecklistForbidden()
	}

	item, err := s.loadChecklistItem(ctx, taskID, itemID)
	if err != nil {
		return nil, err
	}

	if title != nil {
		if err := item.Rename(*title); err != nil {
			return nil, err
		}
	}
	if done != nil {
		item.SetDone(*done, actor.StaffID)
	}

	updated, err := s.checklist.Update(ctx, item)
	if err != nil {
		return nil, err
	}

	s.logger.Info("checklist item updated",
		zap.Uint("task_id", taskID),
		zap.Uint("item_id", itemID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return updated, nil
}

// DeleteChecklistItem убирает пункт из чек-листа.
func (s *Service) DeleteChecklistItem(ctx context.Context, actor role.Actor, taskID, itemID uint) error {
	obj, err := s.load(ctx, taskID)
	if err != nil {
		return err
	}
	if !canEdit(obj, actor) {
		return ErrChecklistForbidden()
	}
	if _, err := s.loadChecklistItem(ctx, taskID, itemID); err != nil {
		return err
	}

	if err := s.checklist.Delete(ctx, itemID); err != nil {
		return err
	}

	s.logger.Info("checklist item deleted",
		zap.Uint("task_id", taskID),
		zap.Uint("item_id", itemID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return nil
}

// createChecklist заводит стартовый чек-лист вместе с задачей.
//
// Пустые строки отбрасываются молча: форма создания отдаёт заготовленные
// поля, и незаполненное поле — не ошибка, а просто ненужный пункт.
func (s *Service) createChecklist(ctx context.Context, taskID uint, titles []string) ([]*ChecklistItem, error) {
	items := make([]*ChecklistItem, 0, len(titles))
	for _, title := range titles {
		if strings.TrimSpace(title) == "" {
			continue
		}
		item, err := NewChecklistItem(taskID, title, len(items))
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, nil
	}
	if len(items) > maxChecklistItems {
		return nil, ErrChecklistLimitReached(maxChecklistItems)
	}

	return s.checklist.CreateMany(ctx, items)
}

// ensureChecklistCapacity проверяет, что список не перерастает предел.
func (s *Service) ensureChecklistCapacity(ctx context.Context, taskID uint, adding int) error {
	count, err := s.checklist.CountByTask(ctx, taskID)
	if err != nil {
		return err
	}
	if int(count)+adding > maxChecklistItems {
		return ErrChecklistLimitReached(maxChecklistItems)
	}
	return nil
}

// loadChecklistItem достаёт пункт и проверяет его принадлежность задаче.
func (s *Service) loadChecklistItem(ctx context.Context, taskID, itemID uint) (*ChecklistItem, error) {
	item, err := s.checklist.GetByID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrChecklistItemNotFound(itemID)
	}
	if item.TaskID != taskID {
		return nil, ErrForeignChild("checklistItem", itemID)
	}
	return item, nil
}
