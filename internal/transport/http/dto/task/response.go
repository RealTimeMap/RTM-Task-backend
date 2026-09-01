package task

import (
	"time"

	"RTM-Task/internal/app/use_cases/task_action"
)

// TaskResponse — представление задачи в API.
type TaskResponse struct {
	ID          uint       `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Priority    int        `json:"priority"`
	CreatorID   uint       `json:"creatorId"`
	AssigneeID  *uint      `json:"assigneeId"`
	Version     int        `json:"version"`
	ClosedAt    *time.Time `json:"closedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`

	// Замечание к доработке: пусто, пока задачу не возвращали или пока
	// её не приняли заново.
	ReworkNote string     `json:"reworkNote,omitempty"`
	ReworkByID *uint      `json:"reworkById,omitempty"`
	ReworkAt   *time.Time `json:"reworkAt,omitempty"`
}

func NewTaskResponse(result task_action.TaskResult) TaskResponse {
	return TaskResponse{
		ID:          result.ID,
		Title:       result.Title,
		Description: result.Description,
		Type:        result.Type,
		Status:      result.Status,
		Priority:    result.Priority,
		CreatorID:   result.CreatorID,
		AssigneeID:  result.AssigneeID,
		Version:     result.Version,
		ClosedAt:    result.ClosedAt,
		CreatedAt:   result.CreatedAt,
		UpdatedAt:   result.UpdatedAt,
		ReworkNote:  result.ReworkNote,
		ReworkByID:  result.ReworkByID,
		ReworkAt:    result.ReworkAt,
	}
}

// TaskListResponse — страница задач с метаданными пагинации.
type TaskListResponse struct {
	Items  []TaskResponse `json:"items"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func NewTaskListResponse(result task_action.TaskListResult) TaskListResponse {
	items := make([]TaskResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, NewTaskResponse(item))
	}
	return TaskListResponse{
		Items:  items,
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	}
}
