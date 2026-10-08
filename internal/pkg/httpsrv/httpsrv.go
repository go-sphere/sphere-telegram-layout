package httpsrv

import (
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/middleware/cors"
	"github.com/go-sphere/sphere/server/middleware/logger"
)

// UseCORS attaches CORS middleware when origins is non-empty. Like the access
// log it is registered on the engine: a preflight for an unmatched path still
// needs the headers.
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

// NewServer initializes and returns a new HTTP server engine configured with the specified address and middlewares.
//
// opts sets the timeouts, request body cap and trusted proxies; see Options.
// NewServer panics on an invalid trusted proxy, which Options.Validate reports.
//
// The engine is the stdx adapter over plain net/http: it owns the *http.Server
// and implements httpx.TestRequester directly, so no wrapper is needed for
// in-process tests or for Stop. Engine.Stop drains with Shutdown and
// force-closes when the caller's context expires (httpx.Close, the same
// sequence httpz.StopServer performs).
func NewServer(name, addr string, opts Options) httpx.Engine {
	engine, _ := newEngine(addr, opts)
	lg := log.With(log.WithAttrs(map[string]any{"module": name}), log.DisableCaller())
	// Engine scope rather than a group's: engine middleware also covers the
	// paths no route matched, and the access log and panic recovery must cover
	// 404s too.
	engine.Use(logger.Log(lg), logger.RecoveryLog(lg, true))
	return engine
}
