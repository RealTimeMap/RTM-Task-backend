package socket

import (
	"RTM-Task/internal/transport/http/middleware"
	"RTM-Task/internal/utils/apperror"
)

// errorBody переводит ошибку любого слоя в тело для ack-ответа.
// Формат совпадает с HTTP-ответом, но без статуса — у сокета его нет.
// Внутренние ошибки наружу обезличиваются, как и в HTTP.
func errorBody(err error) middleware.ErrorBody {
	appErr, ok := apperror.As(err)
	if !ok {
		return middleware.ErrorBody{
			Code:    "internal_error",
			Message: "internal server error",
		}
	}

	if appErr.Kind == apperror.KindInternal {
		return middleware.ErrorBody{
			Code:    appErr.Code,
			Message: "internal server error",
		}
	}

	return middleware.ErrorBody{
		Code:    appErr.Code,
		Message: appErr.Message,
		Field:   appErr.Field,
		Value:   appErr.Value,
	}
}
