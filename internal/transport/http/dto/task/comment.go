package task

import (
	"time"

	"RTM-Task/internal/app/use_cases/task_action"
)

// CreateCommentRequest — тело запроса на добавление комментария.
type CreateCommentRequest struct {
	Body string `json:"body" binding:"required"`
}

// UpdateCommentRequest — тело запроса на правку комментария.
type UpdateCommentRequest struct {
	Body string `json:"body" binding:"required"`
}

// CommentResponse — представление комментария в API.
type CommentResponse struct {
	ID       uint       `json:"id"`
	TaskID   uint       `json:"taskId"`
	AuthorID uint       `json:"authorId"`
	Body     string     `json:"body"`
	EditedAt *time.Time `json:"editedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewCommentResponse(result task_action.CommentResult) CommentResponse {
	return CommentResponse{
		ID:        result.ID,
		TaskID:    result.TaskID,
		AuthorID:  result.AuthorID,
		Body:      result.Body,
		EditedAt:  result.EditedAt,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
	}
}

// CommentListResponse — обсуждение задачи целиком.
//
// Пагинации нет намеренно: обсуждение одной задачи не вырастает до
// размеров, ради которых стоит усложнять клиент.
type CommentListResponse struct {
	Items []CommentResponse `json:"items"`
	Total int               `json:"total"`
}

func NewCommentListResponse(results []task_action.CommentResult) CommentListResponse {
	items := make([]CommentResponse, 0, len(results))
	for _, result := range results {
		items = append(items, NewCommentResponse(result))
	}
	return CommentListResponse{Items: items, Total: len(items)}
}

// CreateChecklistItemRequest — тело запроса на добавление пункта.
type CreateChecklistItemRequest struct {
	Title string `json:"title" binding:"required"`
}

// UpdateChecklistItemRequest — частичное изменение пункта:
// nil-поле означает «не менять».
type UpdateChecklistItemRequest struct {
	Title *string `json:"title"`
	Done  *bool   `json:"done"`
}

// ChecklistItemResponse — представление пункта чек-листа в API.
type ChecklistItemResponse struct {
	ID       uint       `json:"id"`
	TaskID   uint       `json:"taskId"`
	Title    string     `json:"title"`
	Position int        `json:"position"`
	Done     bool       `json:"done"`
	DoneAt   *time.Time `json:"doneAt,omitempty"`
	DoneByID *uint      `json:"doneById,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewChecklistItemResponse(result task_action.ChecklistItemResult) ChecklistItemResponse {
	return ChecklistItemResponse{
		ID:        result.ID,
		TaskID:    result.TaskID,
		Title:     result.Title,
		Position:  result.Position,
		Done:      result.Done,
		DoneAt:    result.DoneAt,
		DoneByID:  result.DoneByID,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
	}
}

// ChecklistResponse — чек-лист задачи вместе со сводкой прогресса.
type ChecklistResponse struct {
	Items []ChecklistItemResponse `json:"items"`
	Total int                     `json:"total"`
	Done  int                     `json:"done"`
}

func NewChecklistResponse(results []task_action.ChecklistItemResult) ChecklistResponse {
	items := make([]ChecklistItemResponse, 0, len(results))
	done := 0
	for _, result := range results {
		if result.Done {
			done++
		}
		items = append(items, NewChecklistItemResponse(result))
	}
	return ChecklistResponse{Items: items, Total: len(items), Done: done}
}
