package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"RTM-Task/internal/app"
)

// Server — HTTP-сервер сервиса задач.
type Server struct {
	http   *http.Server
	logger *zap.Logger
}

// NewServer собирает gin-движок с маршрутами и оборачивает его в http.Server.
func NewServer(container *app.Container) *Server {
	if container.Config.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	RegisterRoutes(engine, container)

	cfg := container.Config.HTTP
	return &Server{
		http: &http.Server{
			Addr:         cfg.Address(),
			Handler:      engine,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
		logger: container.Logger,
	}
}

// Start слушает порт до остановки сервера.
func (s *Server) Start() error {
	s.logger.Info("http server started", zap.String("addr", s.http.Addr))

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown корректно завершает работу сервера.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	return s.http.Shutdown(ctx)
}
