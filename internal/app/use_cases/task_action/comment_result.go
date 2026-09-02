package task_action

import (
	"time"

	"RTM-Task/internal/domain/task"
)

// CommentResult — комментарий в виде, пригодном для транспорта.
type CommentResult struct {
	ID       uint
	TaskID   uint
	AuthorID uint
	Body     string
	EditedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func toCommentResult(obj *task.Comment) CommentResult {
	if obj == nil {
		return CommentResult{}
	}
	return CommentResult{
		ID:        obj.ID,
		TaskID:    obj.TaskID,
		AuthorID:  obj.AuthorID,
		Body:      obj.Body,
		EditedAt:  obj.EditedAt,
		CreatedAt: obj.CreatedAt,
		UpdatedAt: obj.UpdatedAt,
	}
}

func toCommentResults(objs []*task.Comment) []CommentResult {
	results := make([]CommentResult, 0, len(objs))
	for _, obj := range objs {
		results = append(results, toCommentResult(obj))
	}
	return results
}

// ChecklistItemResult — пункт чек-листа для транспорта.
type ChecklistItemResult struct {
	ID       uint
	TaskID   uint
	Title    string
	Position int
	Done     bool
	DoneAt   *time.Time
	DoneByID *uint

	CreatedAt time.Time
	UpdatedAt time.Time
}

func toChecklistItemResult(obj *task.ChecklistItem) ChecklistItemResult {
	if obj == nil {
		return ChecklistItemResult{}
	}
	return ChecklistItemResult{
		ID:        obj.ID,
		TaskID:    obj.TaskID,
		Title:     obj.Title,
		Position:  obj.Position,
		Done:      obj.IsDone(),
		DoneAt:    obj.DoneAt,
		DoneByID:  obj.DoneByID,
		CreatedAt: obj.CreatedAt,
		UpdatedAt: obj.UpdatedAt,
	}
}

func toChecklistItemResults(objs []*task.ChecklistItem) []ChecklistItemResult {
	results := make([]ChecklistItemResult, 0, len(objs))
	for _, obj := range objs {
		results = append(results, toChecklistItemResult(obj))
	}
	return results
}
