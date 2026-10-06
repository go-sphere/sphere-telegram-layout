package dash

import (
	"context"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere-telegram-layout/internal/service/dash"
)

// NewSessionMetaData records the request metadata the auth context stores
// alongside the session, then continues the chain.
func NewSessionMetaData() httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			// Both values go through SetContext into the standard context:
			// httpx StateStore (ctx.Set) and context.Context are separate channels
			// and the service layer only ever sees the latter.
			stdCtx := context.WithValue(ctx.Context(), dash.AuthContextKeyIP, ctx.ClientIP())
			stdCtx = context.WithValue(stdCtx, dash.AuthContextKeyUA, ctx.Header("User-Agent"))
			ctx.SetContext(stdCtx)
			return next(ctx)
		}
	}
}
