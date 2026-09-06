package httpsrv

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/ginx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/server/middleware/cors"
	"github.com/go-sphere/sphere/server/middleware/logger"
)

const readHeaderTimeout = 10 * time.Second

var errNoTestRequester = errors.New("httpsrv: wrapped engine does not support in-process requests")

// Server is an httpx.Engine that owns the net/http.Server so Stop can use
// httpz.StopServer (Shutdown, then Close if the context expires) without
// changing httpx adapters.
type Server struct {
	httpx.Engine
	httpServer *http.Server
}

// NewGinServer initializes and returns a new HTTP server engine configured with the specified address and middlewares.
func NewGinServer(name, addr string) httpx.Engine {
	httpServer := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: readHeaderTimeout,
	}
	app := ginx.New(
		ginx.WithEngine(gin.New()),
		ginx.WithServer(httpServer),
		ginx.WithHTTPXErrorHandler(httpz.AbortWithJsonError),
	)
	lg := log.With(log.WithAttrs(map[string]any{"module": name}), log.DisableCaller())
	app.Use(logger.Log(lg), logger.RecoveryLog(lg, true))
	return &Server{Engine: app, httpServer: httpServer}
}

// Stop shuts down the listener with httpz.StopServer, then marks the
// adapter closed so a later Start returns httpx.ErrEngineClosed.
func (s *Server) Stop(ctx context.Context) error {
	err := httpz.StopServer(ctx, s.httpServer)
	_ = s.Engine.Stop(context.Background())
	return err
}

// Do forwards in-process test requests to the wrapped engine.
func (s *Server) Do(req *http.Request) (*http.Response, error) {
	tr, ok := httpx.AsTestRequester(s.Engine)
	if !ok {
		return nil, errNoTestRequester
	}
	return tr.Do(req)
}

// UseCORS attaches CORS middleware when origins is non-empty.
func UseCORS(engine httpx.Engine, origins []string) error {
	if len(origins) == 0 {
		return nil
	}
	mw, err := cors.NewCORS(cors.WithAllowOrigins(origins...))
	if err != nil {
		return err
	}
	engine.Use(mw)
	return nil
}
