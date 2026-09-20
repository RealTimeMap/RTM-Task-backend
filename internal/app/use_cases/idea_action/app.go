// Package idea_action — сценарии работы с копилкой идей.
//
// Собраны в один обработчик, а не разложены по файлу на сценарий, как
// у задач: у идеи нет ни жизненного цикла, ни уведомлений — все
// операции умещаются в несколько строк каждая, и дробить их значило
// бы плодить папку однострочных типов.
//
// События в сокет идеи всё же рассылают: их состав виден бейджем в
// меню с любого экрана, и узнавать об изменении только при заходе в
// раздел значило бы держать на виду неверное число.
package idea_action

import (
	"context"
	"time"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/idea"
	"RTM-Task/internal/domain/role"
)

// IdeaService — то, что сценариям нужно от домена идей.
type IdeaService interface {
	Create(ctx context.Context, actor role.Actor, params idea.CreateParams) (*idea.Idea, error)
	Get(ctx context.Context, id uint) (*idea.Idea, error)
	List(ctx context.Context, filter idea.Filter) ([]*idea.Idea, int64, error)
	CommentCounts(ctx context.Context, ids []uint) (map[uint]int, error)
	Update(ctx context.Context, actor role.Actor, id uint, params idea.UpdateParams) (*idea.Idea, error)
	SetDone(ctx context.Context, actor role.Actor, id uint, done bool) (*idea.Idea, error)
	Delete(ctx context.Context, actor role.Actor, id uint) error

	Comments(ctx context.Context, ideaID uint) ([]*idea.Comment, error)
	AddComment(ctx context.Context, actor role.Actor, ideaID uint, body string) (*idea.Comment, error)
	UpdateComment(ctx context.Context, actor role.Actor, ideaID, commentID uint, body string) (*idea.Comment, error)
	DeleteComment(ctx context.Context, actor role.Actor, ideaID, commentID uint) error
}

// Application — точка входа транспортного слоя в сценарии идей.
type Application struct {
	service   IdeaService
	publisher EventPublisher
	logger    *zap.Logger
}

// NewApplication собирает сценарии идей.
//
// publisher может быть nil: без сокета сценарии работают как прежде,
// просто не рассылая событий — так их можно поднять и в тесте.
func NewApplication(service IdeaService, publisher EventPublisher, logger *zap.Logger) *Application {
	return &Application{
		service:   service,
		publisher: publisher,
		logger:    logger.Named("idea_use_cases"),
	}
}

// IdeaResult — идея в виде, пригодном для транспорта.
type IdeaResult struct {
	ID          uint
	Title       string
	Description string
	AuthorID    uint

	Done     bool
	DoneAt   *time.Time
	DoneByID *uint

	// CommentCount проставляется только в списке и при чтении одной
	// идеи: по нему видно, что замысел обсуждали, не открывая его.
	CommentCount int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// CommentResult — реплика обсуждения в виде, пригодном для транспорта.
type CommentResult struct {
	ID       uint
	IdeaID   uint
	AuthorID uint
	Body     string
	EditedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ListResult — страница перечня идей.
type ListResult struct {
	Items  []IdeaResult
	Total  int64
	Limit  int
	Offset int
}

func toIdeaResult(obj *idea.Idea) IdeaResult {
	if obj == nil {
		return IdeaResult{}
	}
	return IdeaResult{
		ID:          obj.ID,
		Title:       obj.Title,
		Description: obj.Description,
		AuthorID:    obj.AuthorID,
		Done:        obj.Done,
		DoneAt:      obj.DoneAt,
		DoneByID:    obj.DoneByID,
		CreatedAt:   obj.CreatedAt,
		UpdatedAt:   obj.UpdatedAt,
	}
}

func toCommentResult(obj *idea.Comment) CommentResult {
	if obj == nil {
		return CommentResult{}
	}
	return CommentResult{
		ID:        obj.ID,
		IdeaID:    obj.IdeaID,
		AuthorID:  obj.AuthorID,
		Body:      obj.Body,
		EditedAt:  obj.EditedAt,
		CreatedAt: obj.CreatedAt,
		UpdatedAt: obj.UpdatedAt,
	}
}

func toCommentResults(objs []*idea.Comment) []CommentResult {
	results := make([]CommentResult, 0, len(objs))
	for _, obj := range objs {
		results = append(results, toCommentResult(obj))
	}
	return results
}
