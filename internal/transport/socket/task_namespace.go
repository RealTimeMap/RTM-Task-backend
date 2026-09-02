package socket

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	sio "github.com/zishang520/socket.io/v2/socket"
	"go.uber.org/zap"

	"RTM-Task/internal/app/use_cases/task_action"
	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/transport/http/middleware"
	"RTM-Task/internal/utils/apperror"
)

// TaskNamespace — namespace задач.
const TaskNamespace = "/tasks"

// Имена событий Client → Server.
const (
	eventList        = "tasks:list"
	eventGet         = "tasks:get"
	eventSubscribe   = "tasks:subscribe"
	eventUnsubscribe = "tasks:unsubscribe"
)

// Комнаты рассылки.
const (
	// roomAll получает все события изменения задач.
	roomAll = "tasks:all"
	// roomAssigneePrefix + id — события по задачам конкретного исполнителя.
	roomAssigneePrefix = "tasks:assignee:"
)

// assigneeRoom возвращает имя комнаты исполнителя.
func assigneeRoom(staffID uint) string {
	return roomAssigneePrefix + strconv.FormatUint(uint64(staffID), 10)
}

// handlerTimeout ограничивает время обработки одного запроса от клиента.
const handlerTimeout = 10 * time.Second

// actorKey — ключ, под которым участник операции хранится в данных сокета.
const actorKey = "rtm.actor"

// InitTaskNamespace поднимает namespace /tasks.
//
// Client → Server:
//   - tasks:list        выборка задач с фильтрами, ответ через ack
//   - tasks:get         одна задача по id, ответ через ack
//   - tasks:subscribe   подписка на события по исполнителю
//   - tasks:unsubscribe отписка от событий по исполнителю
//
// Server → Client:
//   - taskCreated, taskUpdated, taskStatusChanged, taskReworked,
//     taskAssigned, taskUnassigned, taskDeleted
func InitTaskNamespace(s *Server) {
	ns := s.io.Of(TaskNamespace, nil)
	s.logger.Info("init task namespace", zap.String("namespace", TaskNamespace))

	// Аутентификация до установления соединения: неавторизованный клиент
	// получает ошибку рукопожатия и в namespace не попадает.
	ns.Use(func(socket *sio.Socket, next func(*sio.ExtendedError)) {
		actor, err := s.authenticate(socket)
		if err != nil {
			s.logger.Warn("socket authentication failed",
				zap.String("socket_id", string(socket.Id())),
				zap.Error(err),
			)
			next(sio.NewExtendedError(err.Error(), errorBody(err)))
			return
		}

		socket.SetData(map[string]any{actorKey: actor})
		next(nil)
	})

	ns.On("connection", func(clients ...any) {
		socket := clients[0].(*sio.Socket)

		actor, ok := actorFrom(socket)
		if !ok {
			// Middleware обязан положить участника; страховка от рассинхрона.
			socket.Disconnect(true)
			return
		}

		// Каждое подключение по умолчанию слушает общий поток изменений
		// и поток задач, назначенных на самого сотрудника.
		socket.Join(sio.Room(roomAll), sio.Room(assigneeRoom(actor.StaffID)))

		s.logger.Info("socket connected",
			zap.String("socket_id", string(socket.Id())),
			zap.Uint("staff_id", actor.StaffID),
		)

		socket.On(eventList, func(args ...any) {
			s.handleList(args)
		})

		socket.On(eventGet, func(args ...any) {
			s.handleGet(args)
		})

		socket.On(eventSubscribe, func(args ...any) {
			s.handleSubscription(socket, actor, args, true)
		})

		socket.On(eventUnsubscribe, func(args ...any) {
			s.handleSubscription(socket, actor, args, false)
		})

		socket.On("disconnect", func(...any) {
			s.logger.Info("socket disconnected",
				zap.String("socket_id", string(socket.Id())),
				zap.Uint("staff_id", actor.StaffID),
			)
		})
	})
}

// authenticate разрешает участника операции по заголовкам рукопожатия —
// тем же, что шлюз проставляет для HTTP-запросов.
func (s *Server) authenticate(socket *sio.Socket) (role.Actor, error) {
	handshake := socket.Handshake()
	if handshake == nil {
		return role.Actor{}, apperror.NewUnauthorizedError("handshake data is missing")
	}

	identity, err := middleware.IdentityFromHeaders(http.Header(handshake.Headers))
	if err != nil {
		return role.Actor{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	return s.staff.ResolveActor(ctx, identity)
}

// actorFrom достаёт участника операции из данных сокета.
func actorFrom(socket *sio.Socket) (role.Actor, bool) {
	data, ok := socket.Data().(map[string]any)
	if !ok {
		return role.Actor{}, false
	}
	actor, ok := data[actorKey].(role.Actor)
	return actor, ok
}

func (s *Server) handleList(args []any) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	payload, ack := splitArgs(args)

	var query listTasksPayload
	if err := decodePayload(payload, &query); err != nil {
		s.replyError(ack, "decode tasks:list payload", malformedPayload(err))
		return
	}

	result, err := s.useCases.ListTasks.Handle(ctx, task_action.ListTasksQuery{
		Status:         query.Status,
		Type:           query.Type,
		Priority:       query.Priority,
		CreatorID:      query.CreatorID,
		AssigneeID:     query.AssigneeID,
		OnlyUnassigned: query.Unassigned,
		Sort:           optionalString(query.Sort),
		Order:          optionalString(query.Order),
		Limit:          query.Limit,
		Offset:         query.Offset,
	})
	if err != nil {
		s.replyError(ack, "handle tasks:list", err)
		return
	}

	reply(ack, ackOK(map[string]any{"tasks": taskListPayload(result)}))
}

func (s *Server) handleGet(args []any) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	payload, ack := splitArgs(args)

	var query getTaskPayload
	if err := decodePayload(payload, &query); err != nil {
		s.replyError(ack, "decode tasks:get payload", malformedPayload(err))
		return
	}
	if query.TaskID == 0 {
		s.replyError(ack, "handle tasks:get", apperror.NewRequiredError("taskId"))
		return
	}

	result, err := s.useCases.GetTask.Handle(ctx, task_action.GetTaskQuery{TaskID: query.TaskID})
	if err != nil {
		s.replyError(ack, "handle tasks:get", err)
		return
	}

	reply(ack, ackOK(map[string]any{"task": taskPayload(result)}))
}

// handleSubscription добавляет сокет в комнату исполнителя или убирает из неё.
// Подписаться на чужой поток может только управляющая роль.
func (s *Server) handleSubscription(
	socket *sio.Socket,
	actor role.Actor,
	args []any,
	join bool,
) {
	payload, ack := splitArgs(args)

	operation := "handle tasks:unsubscribe"
	if join {
		operation = "handle tasks:subscribe"
	}

	var request subscribePayload
	if err := decodePayload(payload, &request); err != nil {
		s.replyError(ack, operation, malformedPayload(err))
		return
	}

	room := roomAll
	if request.AssigneeID != nil {
		target := *request.AssigneeID
		if !actor.Is(target) && !actor.Role.CanAssignAnyone() {
			s.replyError(ack, operation, apperror.NewForbiddenError(
				"only a manager can subscribe to another staff member's tasks",
			))
			return
		}
		room = assigneeRoom(target)
	}

	if join {
		socket.Join(sio.Room(room))
	} else {
		socket.Leave(sio.Room(room))
	}

	reply(ack, ackOK(map[string]any{"room": room}))
}

// splitArgs разделяет аргументы события на полезную нагрузку и ack-колбэк.
// Клиент может прислать только колбэк, только данные, или и то и другое.
func splitArgs(args []any) (payload any, ack sio.Ack) {
	for _, arg := range args {
		if callback, ok := arg.(func([]any, error)); ok {
			ack = callback
			continue
		}
		if payload == nil {
			payload = arg
		}
	}
	return payload, ack
}

// reply отправляет ack, если клиент его ожидает.
func reply(ack sio.Ack, body map[string]any) {
	if ack == nil {
		return
	}
	ack([]any{body}, nil)
}

// replyError логирует ошибку и сообщает о ней клиенту.
func (s *Server) replyError(ack sio.Ack, operation string, err error) {
	s.logger.Warn("socket "+operation+" failed", zap.Error(err))
	reply(ack, ackError(err))
}

// optionalString разворачивает необязательное поле полезной нагрузки.
func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func malformedPayload(err error) error {
	return apperror.NewValidationError(
		"payload",
		fmt.Sprintf("malformed event payload: %v", err),
		"value_error.payload",
		nil,
	)
}
