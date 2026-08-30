package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"RTM-Task/internal/app/use_cases/staff_action"
	dto "RTM-Task/internal/transport/http/dto/staff"
	"RTM-Task/internal/transport/http/middleware"
	utilhttp "RTM-Task/internal/utils/http"
)

type StaffHandler struct {
	useCases *staff_action.Application
	logger   *zap.Logger
}

// InitStaffHandler регистрирует маршруты сотрудников.
// Группа уже защищена auth-middleware; отдельной регистрации нет —
// сотрудник заводится при первом обращении пользователя платформы.
func InitStaffHandler(rg *gin.RouterGroup, useCases *staff_action.Application, logger *zap.Logger) {
	h := &StaffHandler{useCases: useCases, logger: logger}

	staff := rg.Group("/staff")
	{
		staff.GET("", h.List)
		staff.GET("/me", h.Me)
		staff.GET("/:id", h.Get)
		staff.PATCH("/:id/role", h.ChangeRole)
		staff.DELETE("/:id", h.Deactivate)
	}
}

// Me возвращает сотрудника, от чьего имени выполняется запрос.
// Фронтенду это заменяет экран входа: кто мы — решает шлюз.
func (h *StaffHandler) Me(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	result, err := h.useCases.GetStaff.Handle(c.Request.Context(), staff_action.GetStaffQuery{
		StaffID: actor.StaffID,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewStaffResponse(result))
}

func (h *StaffHandler) Get(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	result, err := h.useCases.GetStaff.Handle(c.Request.Context(), staff_action.GetStaffQuery{StaffID: id})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewStaffResponse(result))
}

func (h *StaffHandler) List(c *gin.Context) {
	results, err := h.useCases.ListStaff.Handle(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewStaffListResponse(results))
}

func (h *StaffHandler) ChangeRole(c *gin.Context) {
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

	var req dto.ChangeRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.ChangeRole.Handle(c.Request.Context(), staff_action.ChangeRoleCommand{
		Actor:   actor,
		StaffID: id,
		Role:    req.Role,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewStaffResponse(result))
}

func (h *StaffHandler) Deactivate(c *gin.Context) {
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

	result, err := h.useCases.Deactivate.Handle(c.Request.Context(), staff_action.DeactivateStaffCommand{
		Actor:   actor,
		StaffID: id,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewStaffResponse(result))
}
