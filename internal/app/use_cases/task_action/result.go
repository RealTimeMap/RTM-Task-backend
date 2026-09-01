package task_action

import (
	"time"

	"RTM-Task/internal/domain/task"
)

// TaskResult — результат use case'а. Отдельный от доменной модели тип:
// транспорт не должен зависеть от внутреннего устройства агрегата.
type TaskResult struct {
	ID          uint
	Title       string
	Description string
	Type        string
	Status      string
	Priority    int
	CreatorID   uint
	AssigneeID  *uint
	Version     int
	ClosedAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Доработка: непустая, пока замечание не снято закрытием задачи.
	ReworkNote string
	ReworkByID *uint
	ReworkAt   *time.Time
}

func toTaskResult(obj *task.Task) TaskResult {
	if obj == nil {
		return TaskResult{}
	}
	return TaskResult{
		ID:          obj.ID,
		Title:       obj.Title,
		Description: obj.Description,
		Type:        obj.Type.String(),
		Status:      obj.Status.String(),
		Priority:    obj.Priority.Int(),
		CreatorID:   obj.CreatorID,
		AssigneeID:  obj.AssigneeID,
		Version:     obj.Version,
		ClosedAt:    obj.ClosedAt,
		CreatedAt:   obj.CreatedAt,
		UpdatedAt:   obj.UpdatedAt,
		ReworkNote:  obj.ReworkNote,
		ReworkByID:  obj.ReworkByID,
		ReworkAt:    obj.ReworkAt,
	}
}

func toTaskResults(objs []*task.Task) []TaskResult {
	results := make([]TaskResult, 0, len(objs))
	for _, obj := range objs {
		results = append(results, toTaskResult(obj))
	}
	return results
}

// TaskListResult — страница выборки задач вместе с общим числом совпадений.
type TaskListResult struct {
	Items  []TaskResult
	Total  int64
	Limit  int
	Offset int
}
