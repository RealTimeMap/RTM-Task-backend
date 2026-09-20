package idea_action

import "context"

// IdeaEvent — уведомление об изменении идеи, отправляемое подписчикам.
// Тип принадлежит application-слою: транспорт реализует доставку,
// но не решает, что и когда публикуется.
type IdeaEvent struct {
	Name string
	Idea IdeaResult
}

// IdeaCommentEvent — уведомление об изменении обсуждения идеи.
type IdeaCommentEvent struct {
	Name    string
	Comment CommentResult
}

// Имена событий Server → Client.
//
// Удаление приходит тем же типом, что и правка: получателю нужен один
// идентификатор, и отдельная форма ради него только добавила бы
// разбора на клиенте.
const (
	EventIdeaCreated = "ideaCreated"
	EventIdeaUpdated = "ideaUpdated"
	EventIdeaDeleted = "ideaDeleted"

	EventIdeaCommentAdded   = "ideaCommentAdded"
	EventIdeaCommentUpdated = "ideaCommentUpdated"
	EventIdeaCommentDeleted = "ideaCommentDeleted"
)

// EventPublisher — порт доставки событий подписчикам.
// Реализуется socket-транспортом; nil допустим — тогда push просто не идёт.
type EventPublisher interface {
	PublishIdea(ctx context.Context, event IdeaEvent)
	PublishIdeaComment(ctx context.Context, event IdeaCommentEvent)
}

// publish отправляет событие, если publisher подключён.
// Сбой доставки не должен влиять на результат операции — она уже выполнена.
func publish(ctx context.Context, publisher EventPublisher, event IdeaEvent) {
	if publisher == nil {
		return
	}
	publisher.PublishIdea(ctx, event)
}

// publishComment отправляет событие обсуждения, если publisher подключён.
func publishComment(ctx context.Context, publisher EventPublisher, event IdeaCommentEvent) {
	if publisher == nil {
		return
	}
	publisher.PublishIdeaComment(ctx, event)
}
