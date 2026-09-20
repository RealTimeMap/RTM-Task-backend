package idea_action

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/idea"
	"RTM-Task/internal/domain/role"
)

// CreateCommand — заведение идеи.
type CreateCommand struct {
	Actor       role.Actor
	Title       string
	Description string
}

// ListQuery — перечень идей.
type ListQuery struct {
	// Done отбирает по состоянию: nil — показать все.
	Done *bool

	AuthorID uint
	Limit    int
	Offset   int
}

// UpdateCommand — правка заголовка и описания. Нулевые поля означают
// «не трогать»: форма правит их по отдельности.
type UpdateCommand struct {
	Actor       role.Actor
	ID          uint
	Title       *string
	Description *string
}

// SetDoneCommand — отметка выполнения.
type SetDoneCommand struct {
	Actor role.Actor
	ID    uint
	Done  bool
}

// DeleteCommand — удаление идеи вместе с обсуждением.
type DeleteCommand struct {
	Actor role.Actor
	ID    uint
}

// CommentCommand — добавление или правка реплики.
type CommentCommand struct {
	Actor     role.Actor
	IdeaID    uint
	CommentID uint
	Body      string
}

const (
	defaultLimit = 100
	maxLimit     = 200
)

// Create заводит идею.
func (a *Application) Create(ctx context.Context, cmd CreateCommand) (IdeaResult, error) {
	obj, err := a.service.Create(ctx, cmd.Actor, idea.CreateParams{
		Title:       cmd.Title,
		Description: cmd.Description,
	})
	if err != nil {
		return IdeaResult{}, err
	}

	a.logger.Info("idea created", zap.Uint("id", obj.ID), zap.Uint("author_id", obj.AuthorID))

	result := toIdeaResult(obj)
	publish(ctx, a.publisher, IdeaEvent{Name: EventIdeaCreated, Idea: result})
	return result, nil
}

// Get отдаёт идею вместе с числом реплик.
func (a *Application) Get(ctx context.Context, id uint) (IdeaResult, error) {
	obj, err := a.service.Get(ctx, id)
	if err != nil {
		return IdeaResult{}, err
	}

	result := toIdeaResult(obj)

	// Счётчик реплик — вспомогательная величина: если он не собрался,
	// это не повод не отдать саму идею.
	if counts, err := a.service.CommentCounts(ctx, []uint{id}); err != nil {
		a.logger.Warn("idea comment count failed", zap.Uint("id", id), zap.Error(err))
	} else {
		result.CommentCount = counts[id]
	}

	return result, nil
}

// List отдаёт страницу перечня идей со счётчиками реплик.
func (a *Application) List(ctx context.Context, query ListQuery) (ListResult, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	objs, total, err := a.service.List(ctx, idea.Filter{
		Done:     query.Done,
		AuthorID: query.AuthorID,
		Limit:    limit,
		Offset:   query.Offset,
	})
	if err != nil {
		return ListResult{}, err
	}

	items := make([]IdeaResult, 0, len(objs))
	ids := make([]uint, 0, len(objs))
	for _, obj := range objs {
		items = append(items, toIdeaResult(obj))
		ids = append(ids, obj.ID)
	}

	// Счётчики одним запросом на всю страницу, а не по одному на
	// карточку. Сбой здесь не должен ронять список: идеи важнее
	// числа реплик под ними.
	if counts, err := a.service.CommentCounts(ctx, ids); err != nil {
		a.logger.Warn("idea comment counts failed", zap.Error(err))
	} else {
		for i := range items {
			items[i].CommentCount = counts[items[i].ID]
		}
	}

	return ListResult{Items: items, Total: total, Limit: limit, Offset: query.Offset}, nil
}

// Update правит заголовок и описание идеи.
func (a *Application) Update(ctx context.Context, cmd UpdateCommand) (IdeaResult, error) {
	obj, err := a.service.Update(ctx, cmd.Actor, cmd.ID, idea.UpdateParams{
		Title:       cmd.Title,
		Description: cmd.Description,
	})
	if err != nil {
		return IdeaResult{}, err
	}

	result := toIdeaResult(obj)
	publish(ctx, a.publisher, IdeaEvent{Name: EventIdeaUpdated, Idea: result})
	return result, nil
}

// SetDone отмечает идею выполненной или возвращает её в работу.
func (a *Application) SetDone(ctx context.Context, cmd SetDoneCommand) (IdeaResult, error) {
	obj, err := a.service.SetDone(ctx, cmd.Actor, cmd.ID, cmd.Done)
	if err != nil {
		return IdeaResult{}, err
	}

	a.logger.Info("idea done changed", zap.Uint("id", cmd.ID), zap.Bool("done", cmd.Done))

	// Отметка выполнения — та же правка идеи: получателю приходит её
	// новое состояние целиком, и отдельное событие про один флаг
	// заставило бы клиент собирать идею из кусков.
	result := toIdeaResult(obj)
	publish(ctx, a.publisher, IdeaEvent{Name: EventIdeaUpdated, Idea: result})
	return result, nil
}

// Delete убирает идею вместе с обсуждением.
func (a *Application) Delete(ctx context.Context, cmd DeleteCommand) error {
	if err := a.service.Delete(ctx, cmd.Actor, cmd.ID); err != nil {
		return err
	}

	a.logger.Info("idea deleted", zap.Uint("id", cmd.ID))

	// Идеи уже нет — в событии несём только идентификатор: по нему
	// получатель уберёт её у себя.
	publish(ctx, a.publisher, IdeaEvent{Name: EventIdeaDeleted, Idea: IdeaResult{ID: cmd.ID}})
	return nil
}

// --- Обсуждение ------------------------------------------------------

// Comments отдаёт обсуждение идеи.
func (a *Application) Comments(ctx context.Context, ideaID uint) ([]CommentResult, error) {
	objs, err := a.service.Comments(ctx, ideaID)
	if err != nil {
		return nil, err
	}
	return toCommentResults(objs), nil
}

// AddComment добавляет реплику.
func (a *Application) AddComment(ctx context.Context, cmd CommentCommand) (CommentResult, error) {
	obj, err := a.service.AddComment(ctx, cmd.Actor, cmd.IdeaID, cmd.Body)
	if err != nil {
		return CommentResult{}, err
	}

	result := toCommentResult(obj)
	publishComment(ctx, a.publisher, IdeaCommentEvent{Name: EventIdeaCommentAdded, Comment: result})
	return result, nil
}

// UpdateComment правит текст реплики.
func (a *Application) UpdateComment(ctx context.Context, cmd CommentCommand) (CommentResult, error) {
	obj, err := a.service.UpdateComment(ctx, cmd.Actor, cmd.IdeaID, cmd.CommentID, cmd.Body)
	if err != nil {
		return CommentResult{}, err
	}

	result := toCommentResult(obj)
	publishComment(ctx, a.publisher, IdeaCommentEvent{Name: EventIdeaCommentUpdated, Comment: result})
	return result, nil
}

// DeleteComment убирает реплику.
func (a *Application) DeleteComment(ctx context.Context, cmd CommentCommand) error {
	if err := a.service.DeleteComment(ctx, cmd.Actor, cmd.IdeaID, cmd.CommentID); err != nil {
		return err
	}

	// Реплики уже нет — несём идентификаторы, по которым получатель
	// найдёт её у себя: саму реплику восстанавливать не из чего.
	publishComment(ctx, a.publisher, IdeaCommentEvent{
		Name:    EventIdeaCommentDeleted,
		Comment: CommentResult{ID: cmd.CommentID, IdeaID: cmd.IdeaID},
	})
	return nil
}
