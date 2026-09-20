package socket

import (
	"encoding/json"

	"RTM-Task/internal/app/use_cases/idea_action"
	"RTM-Task/internal/app/use_cases/task_action"
	ideadto "RTM-Task/internal/transport/http/dto/idea"
	dto "RTM-Task/internal/transport/http/dto/task"
)

// listTasksPayload — параметры выборки, приходящие от клиента событием
// `tasks:list`. Все поля опциональны.
type listTasksPayload struct {
	Status     *string `json:"status"`
	Type       *string `json:"type"`
	Priority   *int    `json:"priority"`
	Project    *string `json:"project"`
	CreatorID  *uint   `json:"creatorId"`
	AssigneeID *uint   `json:"assigneeId"`
	Unassigned bool    `json:"unassigned"`
	Sort       *string `json:"sort"`
	Order      *string `json:"order"`
	Limit      int     `json:"limit"`
	Offset     int     `json:"offset"`
}

// getTaskPayload — параметры события `tasks:get`.
type getTaskPayload struct {
	TaskID uint `json:"taskId"`
}

// subscribePayload — параметры события `tasks:subscribe`.
// Клиент просит присылать ему изменения по задачам конкретного исполнителя.
type subscribePayload struct {
	AssigneeID *uint `json:"assigneeId"`
}

// decodePayload переводит произвольные данные события в типизированную структуру.
// Библиотека отдаёт уже разобранный JSON как interface{}, поэтому
// проще всего пройти через повторную сериализацию.
func decodePayload(raw any, target any) error {
	if raw == nil {
		return nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// ackOK — успешный ответ на запрос клиента.
func ackOK(payload map[string]any) map[string]any {
	result := map[string]any{"success": true}
	for key, value := range payload {
		result[key] = value
	}
	return result
}

// ackError — ответ с ошибкой в том же формате, что и HTTP-ошибки.
func ackError(err error) map[string]any {
	body := errorBody(err)
	return map[string]any{
		"success": false,
		"error":   body,
	}
}

// taskPayload переводит результат use case'а в то же представление,
// что отдаёт HTTP: клиенту не приходится различать каналы.
func taskPayload(result task_action.TaskResult) dto.TaskResponse {
	return dto.NewTaskResponse(result)
}

func taskListPayload(result task_action.TaskListResult) dto.TaskListResponse {
	return dto.NewTaskListResponse(result)
}

// commentPayload и checklistPayload переводят результаты use case'ов
// в то же представление, что отдаёт HTTP.
func commentPayload(result task_action.CommentResult) dto.CommentResponse {
	return dto.NewCommentResponse(result)
}

func checklistPayload(result task_action.ChecklistItemResult) dto.ChecklistItemResponse {
	return dto.NewChecklistItemResponse(result)
}

// ideaPayload и ideaCommentPayload переводят результаты сценариев идей
// в те же тела, что отдаёт REST: клиент разбирает идею одним кодом,
// пришла она ответом на запрос или событием.
func ideaPayload(result idea_action.IdeaResult) ideadto.Response {
	return ideadto.NewResponse(result)
}

func ideaCommentPayload(result idea_action.CommentResult) ideadto.CommentResponse {
	return ideadto.NewCommentResponse(result)
}
