package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"RTM-Task/internal/app"
	"RTM-Task/internal/transport/http/handlers"
	"RTM-Task/internal/transport/http/middleware"
)

// RegisterRoutes собирает дерево маршрутов сервиса.
func RegisterRoutes(g *gin.Engine, container *app.Container) {
	// Health не кэшируется: по нему судят о состоянии сервиса сейчас, а
	// не о том, каким оно было, когда ответ положили в кэш.
	g.GET("/health", middleware.NoCache(), healthHandler(container))

	// Публичных маршрутов у сервиса нет: он живёт за шлюзом, который
	// валидирует токен в auth-service и проставляет заголовки пользователя.
	// Сотрудник заводится сам при первом обращении, поэтому регистрация
	// отдельной ручкой не нужна.
	api := g.Group("/api/v1")
	// Кэш запрещаем раньше аутентификации: заголовки должны попасть и в
	// ответы 401/403, иначе браузер закэширует отказ и будет показывать
	// его после входа.
	api.Use(middleware.NoCache())
	api.Use(middleware.AuthRequired(container.StaffService, container.Logger))

	handlers.InitStaffHandler(api, container.StaffUseCases, container.Logger)
	handlers.InitTaskHandler(api, container.TaskUseCases, container.Logger)

	// Socket.IO: получение задач в реальном времени и push об изменениях.
	// Аутентификация выполняется при рукопожатии внутри namespace,
	// поэтому HTTP-middleware здесь не применяется.
	if container.Socket != nil {
		socketHandler := gin.WrapH(container.Socket.HttpHandler())
		g.GET("/socket.io/*any", socketHandler)
		g.POST("/socket.io/*any", socketHandler)
	}
}

// healthHandler отвечает на проверку живости вместе со статусом БД.
func healthHandler(container *app.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := container.DB.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service":  "rtm-task",
				"status":   "degraded",
				"database": "down",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"service":  "rtm-task",
			"status":   "ok",
			"database": "up",
		})
	}
}
