package http

import (
	"context"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
)

type contextKey string

const actorContextKey contextKey = "rtm_task_actor"

// WithActor кладёт участника операции в контекст запроса.
func WithActor(ctx context.Context, actor role.Actor) context.Context {
	return context.WithValue(ctx, actorContextKey, actor)
}

// ActorFrom достаёт участника операции из контекста.
func ActorFrom(ctx context.Context) (role.Actor, error) {
	actor, ok := ctx.Value(actorContextKey).(role.Actor)
	if !ok {
		return role.Actor{}, apperror.NewForbiddenError("request is not authenticated")
	}
	return actor, nil
}
