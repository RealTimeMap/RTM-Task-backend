package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"RTM-Task/internal/app/use_cases/task_action"
	dto "RTM-Task/internal/transport/http/dto/task"
	"RTM-Task/internal/transport/http/middleware"
	"RTM-Task/internal/utils/apperror"
	utilhttp "RTM-Task/internal/utils/http"
)

type TaskHandler struct {
	useCases *task_action.Application
	logger   *zap.Logger
}

// InitTaskHandler регистрирует маршруты задач в переданной группе.
// Группа уже должна быть защищена auth-middleware.
func InitTaskHandler(rg *gin.RouterGroup, useCases *task_action.Application, logger *zap.Logger) {
	h := &TaskHandler{useCases: useCases, logger: logger}

	tasks := rg.Group("/tasks")
	{
		tasks.POST("", h.Create)
		tasks.GET("", h.List)
		tasks.GET("/:id", h.Get)
		tasks.PATCH("/:id", h.Update)
		tasks.PATCH("/:id/status", h.ChangeStatus)
		tasks.POST("/:id/rework", h.SendToRework)
		tasks.PUT("/:id/assignee", h.Assign)
		tasks.DELETE("/:id/assignee", h.Unassign)
		tasks.DELETE("/:id", h.Delete)

		initCommentRoutes(tasks, h)
	}
}

func (h *TaskHandler) Create(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.CreateTask.Handle(c.Request.Context(), task_action.CreateTaskCommand{
		Actor:       actor,
		Title:       req.Title,
		Description: req.Description,
		Type:        req.Type,
		Priority:    req.Priority,
		AssigneeID:  req.AssigneeID,
		Checklist:   req.Checklist,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusCreated, dto.NewTaskResponse(result))
}

func (h *TaskHandler) Get(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	result, err := h.useCases.GetTask.Handle(c.Request.Context(), task_action.GetTaskQuery{TaskID: id})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewTaskResponse(result))
}

func (h *TaskHandler) List(c *gin.Context) {
	var query dto.ListTasksQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.ListTasks.Handle(c.Request.Context(), task_action.ListTasksQuery{
		Status:         query.Status,
		Type:           query.Type,
		Priority:       query.Priority,
		CreatorID:      query.CreatorID,
		AssigneeID:     query.AssigneeID,
		OnlyUnassigned: query.Unassigned,
		Sort:           derefString(query.Sort),
		Order:          derefString(query.Order),
		Limit:          query.Limit,
		Offset:         query.Offset,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewTaskListResponse(result))
}

func (h *TaskHandler) Update(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.UpdateTask.Handle(c.Request.Context(), task_action.UpdateTaskCommand{
		Actor:       actor,
		TaskID:      id,
		Title:       req.Title,
		Description: req.Description,
		Type:        req.Type,
		Priority:    req.Priority,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewTaskResponse(result))
}

func (h *TaskHandler) ChangeStatus(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.ChangeStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.ChangeStatus.Handle(c.Request.Context(), task_action.ChangeStatusCommand{
		Actor:  actor,
		TaskID: id,
		Status: req.Status,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewTaskResponse(result))
}

// SendToRework возвращает завершённую задачу в работу с замечанием.
//
// Отдельный маршрут, а не статус в PATCH /status: переход из complete
// требует обязательного описания доработки, и это видно по контракту.
func (h *TaskHandler) SendToRework(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.SendToReworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.SendToRework.Handle(c.Request.Context(), task_action.SendToReworkCommand{
		Actor:  actor,
		TaskID: id,
		Note:   req.Note,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewTaskResponse(result))
}

func (h *TaskHandler) Assign(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.AssignTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.AssignTask.Handle(c.Request.Context(), task_action.AssignTaskCommand{
		Actor:      actor,
		TaskID:     id,
		AssigneeID: req.AssigneeID,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewTaskResponse(result))
}

func (h *TaskHandler) Unassign(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	result, err := h.useCases.UnassignTask.Handle(c.Request.Context(), task_action.UnassignTaskCommand{
		Actor:  actor,
		TaskID: id,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewTaskResponse(result))
}

func (h *TaskHandler) Delete(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	if err := h.useCases.DeleteTask.Handle(c.Request.Context(), task_action.DeleteTaskCommand{
		Actor:  actor,
		TaskID: id,
	}); err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.Status(http.StatusNoContent)
}

// derefString разворачивает необязательный параметр запроса.
// Отсутствие значения и пустая строка для сортировки означают одно
// и то же — «порядок по умолчанию».
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// parseIDParam читает положительный числовой параметр пути.
func parseIDParam(c *gin.Context, name string) (uint, error) {
	raw := c.Param(name)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, apperror.NewValidationError(
			name,
			"must be a positive integer",
			"value_error.integer",
			raw,
		)
	}
	return uint(id), nil
}
