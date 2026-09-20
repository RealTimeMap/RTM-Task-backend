package socket

import (
	"context"

	sio "github.com/zishang520/socket.io/v2/socket"
	"go.uber.org/zap"

	"RTM-Task/internal/app/use_cases/idea_action"
	"RTM-Task/internal/app/use_cases/task_action"
)

// Publisher доставляет доменные события подписчикам через socket.io.
// Реализует task_action.EventPublisher и idea_action.EventPublisher.
type Publisher struct {
	server *Server
	logger *zap.Logger
}

func NewPublisher(server *Server, logger *zap.Logger) *Publisher {
	return &Publisher{server: server, logger: logger}
}

// PublishTask рассылает событие в общий поток и в комнаты заинтересованных
// исполнителей.
//
// Сокет обычно состоит сразу в нескольких целевых комнатах, но рассылка по
// набору комнат за один вызов доставляет событие каждому получателю один раз.
func (p *Publisher) PublishTask(_ context.Context, event task_action.TaskEvent) {
	if p == nil || p.server == nil {
		return
	}

	rooms := append([]sio.Room{roomAll}, p.assigneeRooms(event)...)

	if err := p.server.io.Of(TaskNamespace, nil).To(rooms...).Emit(event.Name, taskPayload(event.Task)); err != nil {
		p.logger.Warn("publish task event failed",
			zap.String("event", event.Name),
			zap.Error(err),
		)
	}
}

// assigneeRooms собирает комнаты сотрудников, которым событие адресовано
// лично: текущего исполнителя и — при переназначении — прошлого.
func (p *Publisher) assigneeRooms(event task_action.TaskEvent) []sio.Room {
	rooms := make([]sio.Room, 0, 2)

	if event.Task.AssigneeID != nil {
		rooms = append(rooms, sio.Room(assigneeRoom(*event.Task.AssigneeID)))
	}
	if event.PreviousAssigneeID != nil &&
		(event.Task.AssigneeID == nil || *event.PreviousAssigneeID != *event.Task.AssigneeID) {
		rooms = append(rooms, sio.Room(assigneeRoom(*event.PreviousAssigneeID)))
	}

	return rooms
}

// PublishComment рассылает изменение обсуждения задачи.
//
// Адресат — общий поток: обсуждение видят все, кто видит задачу, а
// комнаты исполнителей здесь не помогают — комментарий может оставить
// любой участник, и о нём должны узнать все открывшие задачу.
func (p *Publisher) PublishComment(_ context.Context, event task_action.CommentEvent) {
	if p == nil || p.server == nil {
		return
	}

	err := p.server.io.Of(TaskNamespace, nil).
		To(sio.Room(roomAll)).
		Emit(event.Name, commentPayload(event.Comment))
	if err != nil {
		p.logger.Warn("publish comment event failed",
			zap.String("event", event.Name),
			zap.Error(err),
		)
	}
}

// PublishChecklist рассылает изменение чек-листа задачи.
func (p *Publisher) PublishChecklist(_ context.Context, event task_action.ChecklistEvent) {
	if p == nil || p.server == nil {
		return
	}

	err := p.server.io.Of(TaskNamespace, nil).
		To(sio.Room(roomAll)).
		Emit(event.Name, checklistPayload(event.Item))
	if err != nil {
		p.logger.Warn("publish checklist event failed",
			zap.String("event", event.Name),
			zap.Error(err),
		)
	}
}

// PublishBugsChanged зовёт подписчиков перечитать перечень багов.
//
// Событие идёт пустым: каталог живёт в feedback-service, и его состав
// знает только он. Рассылать отсюда снимок значило бы выдавать за
// истину то, что уже могло измениться, — получатель спросит сам.
func (p *Publisher) PublishBugsChanged(_ context.Context) {
	if p == nil || p.server == nil {
		return
	}

	err := p.server.io.Of(TaskNamespace, nil).
		To(sio.Room(roomAll)).
		Emit(task_action.EventBugsChanged)
	if err != nil {
		p.logger.Warn("publish bugs changed event failed", zap.Error(err))
	}
}

// PublishIdea рассылает изменение идеи.
//
// Адресат — общий поток: копилка одна на команду, комнат исполнителей
// у идеи нет — её никому не назначают. Событие уходит и тому, кто его
// вызвал: свой же экран обновится тем же путём, что и чужие, и
// расхождению взяться неоткуда.
func (p *Publisher) PublishIdea(_ context.Context, event idea_action.IdeaEvent) {
	if p == nil || p.server == nil {
		return
	}

	err := p.server.io.Of(TaskNamespace, nil).
		To(sio.Room(roomAll)).
		Emit(event.Name, ideaPayload(event.Idea))
	if err != nil {
		p.logger.Warn("publish idea event failed",
			zap.String("event", event.Name),
			zap.Error(err),
		)
	}
}

// PublishIdeaComment рассылает изменение обсуждения идеи.
func (p *Publisher) PublishIdeaComment(_ context.Context, event idea_action.IdeaCommentEvent) {
	if p == nil || p.server == nil {
		return
	}

	err := p.server.io.Of(TaskNamespace, nil).
		To(sio.Room(roomAll)).
		Emit(event.Name, ideaCommentPayload(event.Comment))
	if err != nil {
		p.logger.Warn("publish idea comment event failed",
			zap.String("event", event.Name),
			zap.Error(err),
		)
	}
}
