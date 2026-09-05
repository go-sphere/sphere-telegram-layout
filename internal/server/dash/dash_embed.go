//go:build embed_dash

package dash

import (
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere-telegram-layout/assets/dash"
	"github.com/go-sphere/sphere/server/httpz"
)

func (w *Web) RegisterDashStatic(route httpx.Router) {
	if dashFs, err := httpz.Fs(w.config.HTTP.Static, &dash.Assets, dash.AssetsPath); err == nil && dashFs != nil {
		route.StaticFS("/", dashFs)
	}
}
