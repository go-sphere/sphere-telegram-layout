package httpsrv

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/httpz"
)

const (
	readHeaderTimeout = 10 * time.Second

	// DefaultReadTimeout bounds reading one whole request, body included.
	DefaultReadTimeout = 30 * time.Second
	// DefaultIdleTimeout bounds how long a keep-alive connection waits for its
	// next request.
	DefaultIdleTimeout = 120 * time.Second
	// DefaultMaxBodyBytes caps a request body on servers that take JSON only.
	DefaultMaxBodyBytes int64 = 4 << 20

	// DefaultUploadReadTimeout and DefaultUploadMaxBodyBytes replace the
	// defaults above on servers that also accept file uploads; see
	// Options.WithUploadDefaults.
	DefaultUploadReadTimeout        = 5 * time.Minute
	DefaultUploadMaxBodyBytes int64 = 64 << 20
)

// Options tunes the *http.Server and engine NewServer builds. It is meant to
// be embedded in a server's HTTP config, so its fields sit next to address and
// cors in config.json. Every field is optional: 0 selects the default, and a
// negative value disables the limit.
type Options struct {
	// ReadTimeoutSeconds bounds reading one whole request, body included
	// (default DefaultReadTimeout). Responses, and so SSE streams, are not
	// bounded by it.
	ReadTimeoutSeconds int `json:"read_timeout_seconds" yaml:"read_timeout_seconds"`
	// IdleTimeoutSeconds bounds how long a keep-alive connection waits for its
	// next request (default DefaultIdleTimeout).
	IdleTimeoutSeconds int `json:"idle_timeout_seconds" yaml:"idle_timeout_seconds"`
	// MaxBodyBytes caps every request body (default DefaultMaxBodyBytes).
	// Reading past it fails with *http.MaxBytesError and the connection is
	// closed after the response.
	MaxBodyBytes int64 `json:"max_body_bytes" yaml:"max_body_bytes"`
	// TrustedProxies lists the reverse proxies, as IPs or CIDRs, whose
	// X-Forwarded-For header ClientIP honours. Empty ignores forwarding headers
	// and reports the peer address, so behind a proxy every client — and every
	// per-IP rate limit — collapses onto the proxy's address until it is set.
	TrustedProxies []string `json:"trusted_proxies" yaml:"trusted_proxies"`
}

// Validate reports an entry of TrustedProxies that is neither an IP nor a
// CIDR. NewServer panics on such an entry, so configuration loading calls
// Validate first.
func (o Options) Validate() error {
	if _, err := httpx.ParseCIDRs(o.TrustedProxies); err != nil {
		return fmt.Errorf("trusted_proxies: %w", err)
	}
	return nil
}

// WithUploadDefaults returns o with the upload defaults filled in for the
// limits left at 0, for a server whose routes include file uploads: the body
// cap applies to the whole server, uploads included.
func (o Options) WithUploadDefaults() Options {
	if o.ReadTimeoutSeconds == 0 {
		o.ReadTimeoutSeconds = int(DefaultUploadReadTimeout / time.Second)
	}
	if o.MaxBodyBytes == 0 {
		o.MaxBodyBytes = DefaultUploadMaxBodyBytes
	}
	return o
}

func (o Options) readTimeout() time.Duration {
	return seconds(o.ReadTimeoutSeconds, DefaultReadTimeout)
}

func (o Options) idleTimeout() time.Duration {
	return seconds(o.IdleTimeoutSeconds, DefaultIdleTimeout)
}

func (o Options) maxBodyBytes() int64 {
	switch {
	case o.MaxBodyBytes < 0:
		return 0
	case o.MaxBodyBytes == 0:
		return DefaultMaxBodyBytes
	default:
		return o.MaxBodyBytes
	}
}

// seconds converts a configured number of seconds; 0 selects def and a
// negative value yields 0, which net/http reads as "no timeout".
func seconds(n int, def time.Duration) time.Duration {
	switch {
	case n < 0:
		return 0
	case n == 0:
		return def
	default:
		return time.Duration(n) * time.Second
	}
}

// newEngine builds the stdx engine and the *http.Server it serves on, with
// the limits in opts applied. The body cap wraps the server's Handler, so it
// covers every connection but not Engine.Do, which calls the engine directly.
func newEngine(addr string, opts Options) (httpx.Engine, *http.Server) {
	httpServer := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       opts.readTimeout(),
		IdleTimeout:       opts.idleTimeout(),
	}
	engineOpts := []stdx.Option{
		stdx.WithServer(httpServer),
		stdx.WithErrorHandler(httpz.AbortWithJsonError),
	}
	if len(opts.TrustedProxies) > 0 {
		engineOpts = append(engineOpts, stdx.WithTrustedProxies(opts.TrustedProxies...))
	}
	engine := stdx.New(engineOpts...)
	if limit := opts.maxBodyBytes(); limit > 0 {
		httpServer.Handler = http.MaxBytesHandler(httpServer.Handler, limit)
	}
	return engine, httpServer
}
