package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"RTM-Task/internal/app/use_cases/task_action"
	dto "RTM-Task/internal/transport/http/dto/task"
	"RTM-Task/internal/transport/http/middleware"
	utilhttp "RTM-Task/internal/utils/http"
)

// initCommentRoutes вешает обсуждение и чек-лист на группу задачи.
//
// Маршруты вложены в /tasks/:id: комментарий и пункт не существуют
// в отрыве от задачи, и адрес это отражает.
func initCommentRoutes(tasks *gin.RouterGroup, h *TaskHandler) {
	tasks.GET("/:id/comments", h.ListComments)
	tasks.POST("/:id/comments", h.AddComment)
	tasks.PATCH("/:id/comments/:commentId", h.EditComment)
	tasks.DELETE("/:id/comments/:commentId", h.DeleteComment)

	tasks.GET("/:id/checklist", h.ListChecklist)
	tasks.POST("/:id/checklist", h.AddChecklistItem)
	tasks.PATCH("/:id/checklist/:itemId", h.UpdateChecklistItem)
	tasks.DELETE("/:id/checklist/:itemId", h.DeleteChecklistItem)
}

func (h *TaskHandler) ListComments(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	results, err := h.useCases.Comments.List(c.Request.Context(), task_action.ListCommentsQuery{TaskID: id})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewCommentListResponse(results))
}

func (h *TaskHandler) AddComment(c *gin.Context) {
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

	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.Comments.Add(c.Request.Context(), task_action.AddCommentCommand{
		Actor:  actor,
		TaskID: id,
		Body:   req.Body,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusCreated, dto.NewCommentResponse(result))
}

func (h *TaskHandler) EditComment(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, commentID, err := parseTaskChildParams(c, "commentId")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.Comments.Edit(c.Request.Context(), task_action.EditCommentCommand{
		Actor:     actor,
		TaskID:    id,
		CommentID: commentID,
		Body:      req.Body,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewCommentResponse(result))
}

func (h *TaskHandler) DeleteComment(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, commentID, err := parseTaskChildParams(c, "commentId")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	err = h.useCases.Comments.Delete(c.Request.Context(), task_action.DeleteCommentCommand{
		Actor:     actor,
		TaskID:    id,
		CommentID: commentID,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) ListChecklist(c *gin.Context) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	results, err := h.useCases.Checklist.List(c.Request.Context(), task_action.ListChecklistQuery{TaskID: id})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewChecklistResponse(results))
}

func (h *TaskHandler) AddChecklistItem(c *gin.Context) {
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

	var req dto.CreateChecklistItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.Checklist.Add(c.Request.Context(), task_action.AddChecklistItemCommand{
		Actor:  actor,
		TaskID: id,
		Title:  req.Title,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusCreated, dto.NewChecklistItemResponse(result))
}

func (h *TaskHandler) UpdateChecklistItem(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, itemID, err := parseTaskChildParams(c, "itemId")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	var req dto.UpdateChecklistItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.AbortWithBindingError(c, err, h.logger)
		return
	}

	result, err := h.useCases.Checklist.Update(c.Request.Context(), task_action.UpdateChecklistItemCommand{
		Actor:  actor,
		TaskID: id,
		ItemID: itemID,
		Title:  req.Title,
		Done:   req.Done,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, dto.NewChecklistItemResponse(result))
}

func (h *TaskHandler) DeleteChecklistItem(c *gin.Context) {
	actor, err := utilhttp.ActorFrom(c.Request.Context())
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	id, itemID, err := parseTaskChildParams(c, "itemId")
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	err = h.useCases.Checklist.Delete(c.Request.Context(), task_action.DeleteChecklistItemCommand{
		Actor:  actor,
		TaskID: id,
		ItemID: itemID,
	})
	if err != nil {
		middleware.HandleError(c, err, h.logger)
		return
	}

	c.Status(http.StatusNoContent)
}

// parseTaskChildParams читает идентификаторы задачи и вложенной записи.
func parseTaskChildParams(c *gin.Context, childName string) (uint, uint, error) {
	taskID, err := parseIDParam(c, "id")
	if err != nil {
		return 0, 0, err
	}
	childID, err := parseIDParam(c, childName)
	if err != nil {
		return 0, 0, err
	}
	return taskID, childID, nil
}
