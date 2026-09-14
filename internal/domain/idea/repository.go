package idea

import "context"

// Filter — отбор идей для списка.
type Filter struct {
	// Done отбирает по состоянию. nil — показать все: список идей
	// читают целиком чаще, чем по одной половине.
	Done *bool

	// AuthorID отбирает идеи одного автора. Ноль — все авторы.
	AuthorID uint

	Limit  int
	Offset int
}

// Repository — порт хранилища идей.
type Repository interface {
	Create(ctx context.Context, obj *Idea) (*Idea, error)
	GetByID(ctx context.Context, id uint) (*Idea, error)
	List(ctx context.Context, filter Filter) ([]*Idea, int64, error)
	Update(ctx context.Context, obj *Idea) (*Idea, error)
	Delete(ctx context.Context, id uint) error
}

// CommentRepository — порт хранилища комментариев к идеям.
type CommentRepository interface {
	Create(ctx context.Context, obj *Comment) (*Comment, error)
	GetByID(ctx context.Context, id uint) (*Comment, error)

	// ListByIdea возвращает обсуждение идеи в хронологическом порядке.
	ListByIdea(ctx context.Context, ideaID uint) ([]*Comment, error)

	// CountByIdeas собирает число реплик сразу по набору идей — чтобы
	// список не превращался в N+1 запросов ради счётчика на карточке.
	CountByIdeas(ctx context.Context, ideaIDs []uint) (map[uint]int, error)

	Update(ctx context.Context, obj *Comment) (*Comment, error)
	Delete(ctx context.Context, id uint) error

	// DeleteByIdea убирает обсуждение вместе с идеей: реплики к
	// удалённой идее не на что ссылаться.
	DeleteByIdea(ctx context.Context, ideaID uint) error
}
