package middleware

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
	utilhttp "RTM-Task/internal/utils/http"
)

// Заголовки, которые проставляет шлюз после forward auth в auth-service.
// Имена совпадают с pkg/middleware/auth остальных сервисов платформы.
const (
	HeaderUserID    = "X-User-ID"
	HeaderUserName  = "X-User-Name"
	HeaderUserAdmin = "X-User-Admin"
)

// StaffResolver — доменный сервис сотрудников в объёме, нужном middleware:
// сопоставить пользователя платформы с сотрудником и убедиться, что он активен.
type StaffResolver interface {
	ResolveActor(ctx context.Context, identity role.Identity) (role.Actor, error)
}

// AuthRequired пропускает только администраторов платформы.
//
// Заголовкам можно доверять: сервис не выставлен наружу, а запросы приходят
// через шлюз, который валидирует токен в auth-service и подставляет их сам.
// Клиент, обратившийся напрямую, до сервиса не достучится.
func AuthRequired(staff StaffResolver, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, err := IdentityFromHeaders(c.Request.Header)
		if err != nil {
			AbortWithError(c, err, logger)
			return
		}

		actor, err := staff.ResolveActor(c.Request.Context(), identity)
		if err != nil {
			AbortWithError(c, err, logger)
			return
		}

		c.Request = c.Request.WithContext(utilhttp.WithActor(c.Request.Context(), actor))
		c.Next()
	}
}

// IdentityFromHeaders разбирает заголовки шлюза в личность пользователя.
// Вынесено отдельно, чтобы тем же кодом пользовался socket-транспорт.
func IdentityFromHeaders(headers http.Header) (role.Identity, error) {
	rawID := headers.Get(HeaderUserID)
	if rawID == "" {
		return role.Identity{}, apperror.NewUnauthorizedError("X-User-ID header is missing")
	}

	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		return role.Identity{}, apperror.NewUnauthorizedError("X-User-ID must be a positive integer")
	}

	userName := decodeHeaderValue(headers.Get(HeaderUserName))
	if userName == "" {
		return role.Identity{}, apperror.NewUnauthorizedError("X-User-Name header is missing")
	}

	// Доступ к таск-менеджеру есть только у администраторов платформы.
	isAdmin, err := strconv.ParseBool(headers.Get(HeaderUserAdmin))
	if err != nil || !isAdmin {
		return role.Identity{}, apperror.NewForbiddenError("admin access required")
	}

	return role.Identity{
		UserID:   uint(id),
		UserName: userName,
		IsAdmin:  isAdmin,
	}, nil
}

// decodeHeaderValue разбирает значение заголовка, которое могло быть
// закодировано отправителем.
//
// HTTP-заголовки не переносят не-ASCII, поэтому имя с кириллицей
// приходит percent-encoded. Значение без экранирования остаётся как есть —
// декодирование применяется, только если оно действительно что-то меняет
// и даёт корректный UTF-8.
func decodeHeaderValue(value string) string {
	if value == "" || !strings.Contains(value, "%") {
		return value
	}

	decoded, err := url.QueryUnescape(value)
	if err != nil || !utf8.ValidString(decoded) {
		return value
	}
	return decoded
}
