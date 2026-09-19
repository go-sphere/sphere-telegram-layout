package dash

import (
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere-telegram-layout/internal/service/dash"
)

// NewSessionMetaData records the request metadata the auth context stores
// alongside the session, then continues the chain.
func NewSessionMetaData() httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			ctx.Set(dash.AuthContextKeyIP, ctx.ClientIP())
			ctx.Set(dash.AuthContextKeyUA, ctx.Header("User-Agent"))
			return next(ctx)
		}
	}
}
