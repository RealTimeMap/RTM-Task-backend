package task_action

import "context"

// TaskEvent — уведомление об изменении задачи, отправляемое подписчикам.
// Тип принадлежит application-слою: транспорт реализует доставку,
// но не решает, что и когда публикуется.
type TaskEvent struct {
	Name string
	Task TaskResult

	// PreviousAssigneeID заполняется, когда исполнитель сменился:
	// прошлому исполнителю тоже нужно узнать, что задача от него ушла.
	PreviousAssigneeID *uint
}

// Имена событий Server → Client.
const (
	EventTaskCreated       = "taskCreated"
	EventTaskUpdated       = "taskUpdated"
	EventTaskStatusChanged = "taskStatusChanged"
	EventTaskReworked      = "taskReworked"
	EventTaskAssigned      = "taskAssigned"
	EventTaskUnassigned    = "taskUnassigned"
	EventTaskDeleted       = "taskDeleted"

	EventCommentAdded   = "taskCommentAdded"
	EventCommentUpdated = "taskCommentUpdated"
	EventCommentDeleted = "taskCommentDeleted"

	EventChecklistAdded   = "taskChecklistAdded"
	EventChecklistUpdated = "taskChecklistUpdated"
	EventChecklistDeleted = "taskChecklistDeleted"
)

// CommentEvent — уведомление об изменении обсуждения задачи.
type CommentEvent struct {
	Name    string
	Comment CommentResult
}

// ChecklistEvent — уведомление об изменении чек-листа задачи.
type ChecklistEvent struct {
	Name string
	Item ChecklistItemResult
}

// EventPublisher — порт доставки событий подписчикам.
// Реализуется socket-транспортом; nil допустим — тогда push просто не идёт.
type EventPublisher interface {
	PublishTask(ctx context.Context, event TaskEvent)

	// PublishComment и PublishChecklist рассылают изменения внутри
	// задачи. Отдельные методы, а не общий: полезная нагрузка у них
	// разная, и сводить её к map[string]any значило бы потерять типы.
	PublishComment(ctx context.Context, event CommentEvent)
	PublishChecklist(ctx context.Context, event ChecklistEvent)
}

// publish отправляет событие, если publisher подключён.
// Сбой доставки не должен влиять на результат операции — она уже выполнена.
func publish(ctx context.Context, publisher EventPublisher, event TaskEvent) {
	if publisher == nil {
		return
	}
	publisher.PublishTask(ctx, event)
}

// publishComment отправляет событие обсуждения, если publisher подключён.
func publishComment(ctx context.Context, publisher EventPublisher, event CommentEvent) {
	if publisher == nil {
		return
	}
	publisher.PublishComment(ctx, event)
}

// publishChecklist отправляет событие чек-листа, если publisher подключён.
func publishChecklist(ctx context.Context, publisher EventPublisher, event ChecklistEvent) {
	if publisher == nil {
		return
	}
	publisher.PublishChecklist(ctx, event)
}
