package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"RTM-Task/internal/utils/apperror"
)

// ErrorResponse — единый формат ошибки API.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Value   any    `json:"value,omitempty"`
}

// HandleError переводит ошибку любого слоя в HTTP-ответ.
// Внутренние ошибки логируются с деталями, но наружу отдаются обезличенно.
func HandleError(c *gin.Context, err error, logger *zap.Logger) {
	appErr, ok := apperror.As(err)
	if !ok {
		logger.Error("unhandled error",
			zap.String("path", c.FullPath()),
			zap.Error(err),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error: ErrorBody{
				Code:    "internal_error",
				Message: "internal server error",
			},
		})
		return
	}

	// Недоступность зависимости — не вина клиента, и о ней нужно знать
	// в логах. Но текст наружу не прячем: клиенту важно отличить
	// «сосед не отвечает» от собственной ошибки, чтобы предложить
	// повтор вместо общего отказа.
	if appErr.Kind == apperror.KindUnavailable {
		logger.Warn("dependency unavailable",
			zap.String("path", c.FullPath()),
			zap.String("service", appErr.Field),
			zap.Error(err),
		)
		c.AbortWithStatusJSON(appErr.HTTPStatus(), ErrorResponse{
			Error: ErrorBody{
				Code:    appErr.Code,
				Message: appErr.Message,
				Field:   appErr.Field,
			},
		})
		return
	}

	if appErr.Kind == apperror.KindInternal {
		logger.Error("internal error",
			zap.String("path", c.FullPath()),
			zap.Error(err),
		)
		c.AbortWithStatusJSON(appErr.HTTPStatus(), ErrorResponse{
			Error: ErrorBody{
				Code:    appErr.Code,
				Message: "internal server error",
			},
		})
		return
	}

	c.AbortWithStatusJSON(appErr.HTTPStatus(), ErrorResponse{
		Error: ErrorBody{
			Code:    appErr.Code,
			Message: appErr.Message,
			Field:   appErr.Field,
			Value:   appErr.Value,
		},
	})
}

// AbortWithError — синоним HandleError для использования в middleware.
func AbortWithError(c *gin.Context, err error, logger *zap.Logger) {
	HandleError(c, err, logger)
}

// AbortWithBindingError сообщает о некорректном теле запроса.
func AbortWithBindingError(c *gin.Context, err error, logger *zap.Logger) {
	HandleError(c, apperror.NewValidationError(
		"body",
		err.Error(),
		"value_error.binding",
		nil,
	), logger)
}
