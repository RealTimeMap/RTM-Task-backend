// Package idea — контракт HTTP для копилки идей.
package idea

import (
	"time"

	"RTM-Task/internal/app/use_cases/idea_action"
)

// CreateRequest — тело заведения идеи.
type CreateRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

// UpdateRequest — правка идеи.
//
// Указатели, а не строки: nil означает «поле не прислали, не трогать»,
// а пустая строка — осознанное стирание описания. Без различия между
// ними правка одного заголовка стирала бы описание.
type UpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

// SetDoneRequest — отметка выполнения.
type SetDoneRequest struct {
	// Указатель: без него отсутствующее поле молча означало бы false,
	// и запрос без тела снимал бы отметку вместо отказа.
	Done *bool `json:"done" binding:"required"`
}

// CommentRequest — тело реплики обсуждения.
type CommentRequest struct {
	Body string `json:"body" binding:"required"`
}

// ListQuery — параметры перечня идей.
type ListQuery struct {
	// Done отбирает по состоянию. Отсутствие параметра — все идеи.
	Done *bool `form:"done"`

	AuthorID uint `form:"authorId"`
	Limit    int  `form:"limit"`
	Offset   int  `form:"offset"`
}

// Response — идея в ответе.
type Response struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	AuthorID    uint   `json:"authorId"`

	Done     bool       `json:"done"`
	DoneAt   *time.Time `json:"doneAt,omitempty"`
	DoneByID *uint      `json:"doneById,omitempty"`

	CommentCount int `json:"commentCount"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CommentResponse — реплика обсуждения в ответе.
type CommentResponse struct {
	ID       uint       `json:"id"`
	IdeaID   uint       `json:"ideaId"`
	AuthorID uint       `json:"authorId"`
	Body     string     `json:"body"`
	EditedAt *time.Time `json:"editedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ListResponse — страница перечня идей.
type ListResponse struct {
	Items  []Response `json:"items"`
	Total  int64      `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// CommentListResponse — обсуждение одной идеи.
type CommentListResponse struct {
	Items []CommentResponse `json:"items"`
	Total int               `json:"total"`
}

func NewResponse(result idea_action.IdeaResult) Response {
	return Response{
		ID:           result.ID,
		Title:        result.Title,
		Description:  result.Description,
		AuthorID:     result.AuthorID,
		Done:         result.Done,
		DoneAt:       result.DoneAt,
		DoneByID:     result.DoneByID,
		CommentCount: result.CommentCount,
		CreatedAt:    result.CreatedAt,
		UpdatedAt:    result.UpdatedAt,
	}
}

func NewListResponse(result idea_action.ListResult) ListResponse {
	// Пустой срез вместо nil: клиенту не нужно отдельно разбирать
	// случай «идей нет» — он получает [] и рисует пустой список.
	items := make([]Response, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, NewResponse(item))
	}
	return ListResponse{
		Items:  items,
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	}
}

func NewCommentResponse(result idea_action.CommentResult) CommentResponse {
	return CommentResponse{
		ID:        result.ID,
		IdeaID:    result.IdeaID,
		AuthorID:  result.AuthorID,
		Body:      result.Body,
		EditedAt:  result.EditedAt,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
	}
}

func NewCommentListResponse(results []idea_action.CommentResult) CommentListResponse {
	items := make([]CommentResponse, 0, len(results))
	for _, result := range results {
		items = append(items, NewCommentResponse(result))
	}
	return CommentListResponse{Items: items, Total: len(items)}
}
