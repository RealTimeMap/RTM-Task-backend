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
)

// EventPublisher — порт доставки событий подписчикам.
// Реализуется socket-транспортом; nil допустим — тогда push просто не идёт.
type EventPublisher interface {
	PublishTask(ctx context.Context, event TaskEvent)
}

// publish отправляет событие, если publisher подключён.
// Сбой доставки не должен влиять на результат операции — она уже выполнена.
func publish(ctx context.Context, publisher EventPublisher, event TaskEvent) {
	if publisher == nil {
		return
	}
	publisher.PublishTask(ctx, event)
}
