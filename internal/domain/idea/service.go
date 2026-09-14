package idea

import (
	"context"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
)

// Service — доменные операции над идеями.
type Service struct {
	repo     Repository
	comments CommentRepository
	logger   *zap.Logger
}

func NewService(repo Repository, comments CommentRepository, logger *zap.Logger) *Service {
	return &Service{repo: repo, comments: comments, logger: logger}
}

// CreateParams — данные новой идеи.
type CreateParams struct {
	Title       string
	Description string
}

// UpdateParams — правка идеи. Нулевые поля означают «не трогать»:
// форма правит заголовок и описание по отдельности.
type UpdateParams struct {
	Title       *string
	Description *string
}

// Create заводит идею от имени участника.
//
// Права здесь те же, что на заведение задачи: кто может предложить
// работу, тот может и записать замысел. Наблюдателю запись закрыта —
// иначе список идей стал бы общей гостевой книгой.
func (s *Service) Create(ctx context.Context, actor role.Actor, params CreateParams) (*Idea, error) {
	if !actor.Role.CanCreateTask() {
		return nil, ErrForbidden()
	}

	obj, err := New(actor.StaffID, params.Title, params.Description)
	if err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, obj)
}

// Get отдаёт идею по идентификатору.
func (s *Service) Get(ctx context.Context, id uint) (*Idea, error) {
	obj, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, ErrIdeaNotFound(id)
	}
	return obj, nil
}

// List отдаёт идеи по фильтру вместе с общим числом.
func (s *Service) List(ctx context.Context, filter Filter) ([]*Idea, int64, error) {
	return s.repo.List(ctx, filter)
}

// CommentCounts собирает число реплик по набору идей.
func (s *Service) CommentCounts(ctx context.Context, ids []uint) (map[uint]int, error) {
	if len(ids) == 0 {
		return map[uint]int{}, nil
	}
	return s.comments.CountByIdeas(ctx, ids)
}

// Update правит заголовок и описание.
//
// Чужую идею правит только управляющая роль: текст замысла — слова
// его автора, и переписывать их походя нельзя.
func (s *Service) Update(
	ctx context.Context,
	actor role.Actor,
	id uint,
	params UpdateParams,
) (*Idea, error) {
	obj, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if !obj.IsAuthoredBy(actor.StaffID) && !actor.Role.CanEditForeignTask() {
		return nil, ErrForeignIdea()
	}

	if params.Title != nil {
		if err := obj.Rename(*params.Title); err != nil {
			return nil, err
		}
	}
	if params.Description != nil {
		if err := obj.Describe(*params.Description); err != nil {
			return nil, err
		}
	}

	return s.repo.Update(ctx, obj)
}

// SetDone отмечает идею выполненной или возвращает её в работу.
//
// Отмечать может любой, кому доступна запись, а не только автор:
// замысел мог реализовать кто угодно, и заставлять его идти к автору
// ради галочки — лишний круг. Права на правку текста это не даёт.
func (s *Service) SetDone(
	ctx context.Context,
	actor role.Actor,
	id uint,
	done bool,
) (*Idea, error) {
	if !actor.Role.CanCreateTask() {
		return nil, ErrForbidden()
	}

	obj, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if done {
		obj.MarkDone(actor.StaffID)
	} else {
		obj.Reopen()
	}

	return s.repo.Update(ctx, obj)
}

// Delete убирает идею вместе с обсуждением.
//
// Удаляет автор или управляющая роль: идея — заметка, и держать
// её вечно ради истории незачем.
func (s *Service) Delete(ctx context.Context, actor role.Actor, id uint) error {
	obj, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if !obj.IsAuthoredBy(actor.StaffID) && !actor.Role.CanDeleteTask() {
		return ErrForeignIdea()
	}

	// Обсуждение первым: если упадёт удаление самой идеи, реплики уже
	// не на что будет ссылаться, но идея останется целой — это
	// безопаснее, чем осиротевшие комментарии в базе.
	if err := s.comments.DeleteByIdea(ctx, id); err != nil {
		return err
	}

	return s.repo.Delete(ctx, id)
}

// --- Обсуждение ------------------------------------------------------

// Comments отдаёт обсуждение идеи.
func (s *Service) Comments(ctx context.Context, ideaID uint) ([]*Comment, error) {
	if _, err := s.Get(ctx, ideaID); err != nil {
		return nil, err
	}
	return s.comments.ListByIdea(ctx, ideaID)
}

// AddComment добавляет реплику в обсуждение идеи.
func (s *Service) AddComment(
	ctx context.Context,
	actor role.Actor,
	ideaID uint,
	body string,
) (*Comment, error) {
	if !actor.Role.CanCreateTask() {
		return nil, ErrForbidden()
	}

	if _, err := s.Get(ctx, ideaID); err != nil {
		return nil, err
	}

	obj, err := NewComment(ideaID, actor.StaffID, body)
	if err != nil {
		return nil, err
	}

	return s.comments.Create(ctx, obj)
}

// UpdateComment правит текст реплики. Менять его может только автор.
func (s *Service) UpdateComment(
	ctx context.Context,
	actor role.Actor,
	ideaID, commentID uint,
	body string,
) (*Comment, error) {
	obj, err := s.comment(ctx, ideaID, commentID)
	if err != nil {
		return nil, err
	}

	if !obj.IsWrittenBy(actor.StaffID) {
		return nil, ErrForeignComment()
	}

	if err := obj.Rewrite(body); err != nil {
		return nil, err
	}

	return s.comments.Update(ctx, obj)
}

// DeleteComment убирает реплику: автор — свою, управляющая роль — любую.
func (s *Service) DeleteComment(
	ctx context.Context,
	actor role.Actor,
	ideaID, commentID uint,
) error {
	obj, err := s.comment(ctx, ideaID, commentID)
	if err != nil {
		return err
	}

	if !obj.IsWrittenBy(actor.StaffID) && !actor.Role.CanDeleteTask() {
		return ErrForeignComment()
	}

	return s.comments.Delete(ctx, commentID)
}

// comment достаёт реплику, убедившись, что она принадлежит этой идее.
func (s *Service) comment(ctx context.Context, ideaID, commentID uint) (*Comment, error) {
	obj, err := s.comments.GetByID(ctx, commentID)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, ErrCommentNotFound(commentID)
	}
	// Адрес запроса должен соответствовать записи: иначе правка по
	// чужому адресу молча меняла бы обсуждение другой идеи.
	if obj.IdeaID != ideaID {
		return nil, ErrCommentForeign(commentID, ideaID)
	}
	return obj, nil
}
