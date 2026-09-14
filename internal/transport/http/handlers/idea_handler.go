package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"RTM-Task/internal/app/use_cases/idea_action"
	dto "RTM-Task/internal/transport/http/dto/idea"
	"RTM-Task/internal/transport/http/middleware"
	utilhttp "RTM-Task/internal/utils/http"
)

type IdeaHandler struct {
	useCases *idea_action.Application
	logger   *zap.Logger
}

// InitIdeaHandler регистрирует маршруты копилки идей.
// Группа уже должна быть защищена auth-middleware.
func InitIdeaHandler(rg *gin.RouterGroup, useCases *idea_action.Application, logger *zap.Logger) {
	h := &IdeaHandler{useCases: useCases, logger: logger}

	ideas := rg.Group("/ideas")
	{
		ideas.POST("", h.Create)
		ideas.GET("", h.List)
		ideas.GET("/:id", h.Get)
		ideas.PATCH("/:id", h.Update)

		// Отметка выполнения — отдельный маршрут, а не поле в PATCH:
		// это единственное действие, доступное всем, кому открыта
		// запись, тогда как текст правит только автор.
		ideas.PUT("/:id/done", h.SetDone)

		ideas.DELETE("/:id", h.Delete)

		ideas.GET("/:id/comments", h.ListComments)
		ideas.POST("/:id/comments", h.AddComment)
		ideas.PATCH("/:id/comments/:commentId", h.UpdateComment)
		ideas.DELETE("/:id/comments/:commentId", h.DeleteComment)
	}
}

func (h *IdeaHandler) Create(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.Create(c.Request.Context(), idea_action.CreateCommand{
		Actor:       actor,
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusCreated, dto.NewResponse(result))
}

func (h *IdeaHandler) List(c *gin.Context) {
	var query dto.ListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.List(c.Request.Context(), idea_action.ListQuery{
		Done:     query.Done,
		AuthorID: query.AuthorID,
		Limit:    query.Limit,
		Offset:   query.Offset,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewListResponse(result))
}

func (h *IdeaHandler) Get(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	result, err := h.useCases.Get(c.Request.Context(), id)
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewResponse(result))
}

func (h *IdeaHandler) Update(c *gin.Context) {
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

	var req dto.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.Update(c.Request.Context(), idea_action.UpdateCommand{
		Actor:       actor,
		ID:          id,
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewResponse(result))
}

func (h *IdeaHandler) SetDone(c *gin.Context) {
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

	var req dto.SetDoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.SetDone(c.Request.Context(), idea_action.SetDoneCommand{
		Actor: actor,
		ID:    id,
		Done:  *req.Done,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewResponse(result))
}

func (h *IdeaHandler) Delete(c *gin.Context) {
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

	if err := h.useCases.Delete(c.Request.Context(), idea_action.DeleteCommand{
		Actor: actor,
		ID:    id,
	}); err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.Status(http.StatusNoContent)
}

// --- Обсуждение ------------------------------------------------------

func (h *IdeaHandler) ListComments(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	results, err := h.useCases.Comments(c.Request.Context(), id)
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewCommentListResponse(results))
}

func (h *IdeaHandler) AddComment(c *gin.Context) {
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

	var req dto.CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.AddComment(c.Request.Context(), idea_action.CommentCommand{
		Actor:  actor,
		IdeaID: id,
		Body:   req.Body,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusCreated, dto.NewCommentResponse(result))
}

func (h *IdeaHandler) UpdateComment(c *gin.Context) {
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

	commentID, err := parseIDParam(c, "commentId")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.UpdateComment(c.Request.Context(), idea_action.CommentCommand{
		Actor:     actor,
		IdeaID:    id,
		CommentID: commentID,
		Body:      req.Body,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewCommentResponse(result))
}

func (h *IdeaHandler) DeleteComment(c *gin.Context) {
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

	commentID, err := parseIDParam(c, "commentId")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	if err := h.useCases.DeleteComment(c.Request.Context(), idea_action.CommentCommand{
		Actor:     actor,
		IdeaID:    id,
		CommentID: commentID,
	}); err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.Status(http.StatusNoContent)
}
