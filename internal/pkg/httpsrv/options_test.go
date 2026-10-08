package httpsrv

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
)

func TestNewEngineServerTimeouts(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		wantRead time.Duration
		wantIdle time.Duration
	}{
		{name: "defaults", wantRead: DefaultReadTimeout, wantIdle: DefaultIdleTimeout},
		{name: "configured", opts: Options{ReadTimeoutSeconds: 7, IdleTimeoutSeconds: 9}, wantRead: 7 * time.Second, wantIdle: 9 * time.Second},
		{name: "disabled", opts: Options{ReadTimeoutSeconds: -1, IdleTimeoutSeconds: -1}},
		{name: "upload defaults", opts: Options{}.WithUploadDefaults(), wantRead: DefaultUploadReadTimeout, wantIdle: DefaultIdleTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, srv := newEngine("127.0.0.1:0", tt.opts)
			if srv.ReadHeaderTimeout != readHeaderTimeout {
				t.Errorf("ReadHeaderTimeout = %v, want %v", srv.ReadHeaderTimeout, readHeaderTimeout)
			}
			if srv.ReadTimeout != tt.wantRead {
				t.Errorf("ReadTimeout = %v, want %v", srv.ReadTimeout, tt.wantRead)
			}
			if srv.IdleTimeout != tt.wantIdle {
				t.Errorf("IdleTimeout = %v, want %v", srv.IdleTimeout, tt.wantIdle)
			}
		})
	}
}

// postBody serves engine through srv on a real listener and POSTs a JSON
// object of size bytes to /echo, with a declared Content-Length when declared
// is true and chunked encoding otherwise. It reports the status, whether the
// handler ran, and whether the handler's read failed with *http.MaxBytesError.
func postBody(t *testing.T, opts Options, size int, declared bool) (status int, reached, capped bool) {
	t.Helper()
	engine, srv := newEngine("", opts)
	var readErr error
	engine.Group("").POST("/echo", func(ctx httpx.Context) error {
		reached = true
		var v map[string]string
		readErr = ctx.BindJSON(&v)
		if readErr != nil {
			return readErr
		}
		return ctx.NoContent(http.StatusNoContent)
	})
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	// A JSON object whose total length is exactly size bytes.
	body := `{"k":"` + strings.Repeat("a", size-len(`{"k":""}`)) + `"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/echo", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if !declared {
		// A length of 0 with a non-nil body makes the client use chunked
		// encoding, so the server cannot refuse the request up front.
		req.ContentLength = 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /echo: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	_, capped = errors.AsType[*http.MaxBytesError](readErr)
	return resp.StatusCode, reached, capped
}

func TestNewEngineCapsRequestBody(t *testing.T) {
	// A body at the cap is served as usual.
	if status, reached, capped := postBody(t, Options{MaxBodyBytes: 64}, 64, true); status != http.StatusNoContent || !reached || capped {
		t.Fatalf("body at the cap: status = %d, reached = %v, capped = %v; want 204, true, false", status, reached, capped)
	}
	// A declared length over the cap is refused before the route runs.
	if status, reached, capped := postBody(t, Options{MaxBodyBytes: 64}, 65, true); status != http.StatusRequestEntityTooLarge || reached || capped {
		t.Fatalf("declared body over the cap: status = %d, reached = %v, capped = %v; want 413, false, false", status, reached, capped)
	}
	// A chunked body cannot be refused up front: the read that passes the cap
	// fails inside the handler, which answers 413 through the error parser.
	if status, reached, capped := postBody(t, Options{MaxBodyBytes: 64}, 65, false); status != http.StatusRequestEntityTooLarge || !reached || !capped {
		t.Fatalf("chunked body over the cap: status = %d, reached = %v, capped = %v; want 413, true, true", status, reached, capped)
	}
	// The default cap applies to a server that did not configure one.
	if status, _, capped := postBody(t, Options{}, int(DefaultMaxBodyBytes)+1, false); status != http.StatusRequestEntityTooLarge || !capped {
		t.Fatalf("body over DefaultMaxBodyBytes: status = %d, capped = %v; want 413, true", status, capped)
	}
	// A negative value disables the cap.
	if status, reached, capped := postBody(t, Options{MaxBodyBytes: -1}, int(DefaultMaxBodyBytes)+1, true); status != http.StatusNoContent || !reached || capped {
		t.Fatalf("disabled cap: status = %d, reached = %v, capped = %v; want 204, true, false", status, reached, capped)
	}
}

func TestNewEngineTrustedProxies(t *testing.T) {
	clientIP := func(opts Options) string {
		t.Helper()
		engine, srv := newEngine("", opts)
		engine.Group("").GET("/ip", func(ctx httpx.Context) error {
			return ctx.Text(http.StatusOK, ctx.ClientIP())
		})
		ts := httptest.NewServer(srv.Handler)
		defer ts.Close()
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/ip", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /ip: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	if got := clientIP(Options{}); got != "127.0.0.1" {
		t.Errorf("without trusted proxies ClientIP = %q, want the peer 127.0.0.1", got)
	}
	if got := clientIP(Options{TrustedProxies: []string{"127.0.0.1/32"}}); got != "203.0.113.7" {
		t.Errorf("behind a trusted proxy ClientIP = %q, want the forwarded 203.0.113.7", got)
	}
}

func TestOptionsValidate(t *testing.T) {
	if err := (Options{TrustedProxies: []string{"10.0.0.0/8", "192.0.2.1"}}).Validate(); err != nil {
		t.Fatalf("Validate(valid) = %v", err)
	}
	if err := (Options{TrustedProxies: []string{"proxy.internal"}}).Validate(); err == nil {
		t.Fatal("Validate(hostname) = nil, want an error")
	}
}
