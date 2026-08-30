package socket

import (
	"context"
	"net/http"

	sio "github.com/zishang520/socket.io/v2/socket"
	"go.uber.org/zap"

	"RTM-Task/internal/app/use_cases/task_action"
	"RTM-Task/internal/domain/role"
)

// StaffResolver превращает пользователя платформы в участника операции.
// Тот же порт, что использует HTTP-middleware.
type StaffResolver interface {
	ResolveActor(ctx context.Context, identity role.Identity) (role.Actor, error)
}

// Server — socket.io транспорт сервиса задач.
type Server struct {
	io       *sio.Server
	useCases *task_action.Application
	staff    StaffResolver
	logger   *zap.Logger
}

type Deps struct {
	Staff StaffResolver

	Logger *zap.Logger
}

// NewServer поднимает сервер и регистрирует namespace задач.
//
// Use case'ы передаются отдельно через Attach: publisher, который они
// используют, сам ссылается на этот сервер, поэтому собрать всё одним
// вызовом нельзя.
func NewServer(deps Deps) *Server {
	server := &Server{
		io:     sio.NewServer(nil, nil),
		staff:  deps.Staff,
		logger: deps.Logger,
	}

	InitTaskNamespace(server)

	return server
}

// Attach подключает use case'ы, которыми сервер обслуживает запросы клиентов.
// Вызывается один раз при сборке контейнера, до старта HTTP-сервера.
func (s *Server) Attach(useCases *task_action.Application) {
	s.useCases = useCases
}

// HttpHandler отдаёт http.Handler для монтирования в маршруты.
func (s *Server) HttpHandler() http.Handler {
	return s.io.ServeHandler(nil)
}

// Close останавливает сервер сокетов.
func (s *Server) Close() {
	s.io.Close(nil)
}
