package httpsrv

import (
	"bytes"
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

// postBody serves engine through srv on a real listener, POSTs size bytes of
// JSON to /echo, and reports the status and whether the handler's read failed
// with *http.MaxBytesError.
func postBody(t *testing.T, opts Options, size int) (status int, capped bool) {
	t.Helper()
	engine, srv := newEngine("", opts)
	var readErr error
	engine.Group("").POST("/echo", func(ctx httpx.Context) error {
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
	resp, err := http.Post(ts.URL+"/echo", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /echo: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	_, capped = errors.AsType[*http.MaxBytesError](readErr)
	return resp.StatusCode, capped
}

func TestNewEngineCapsRequestBody(t *testing.T) {
	if status, capped := postBody(t, Options{MaxBodyBytes: 64}, 64); status != http.StatusNoContent || capped {
		t.Fatalf("body at the cap: status = %d, capped = %v; want 204, false", status, capped)
	}
	if status, capped := postBody(t, Options{MaxBodyBytes: 64}, 65); status == http.StatusNoContent || !capped {
		t.Fatalf("body over the cap: status = %d, capped = %v; want an error from *http.MaxBytesError", status, capped)
	}
	if _, capped := postBody(t, Options{}, int(DefaultMaxBodyBytes)+1); !capped {
		t.Fatal("body over DefaultMaxBodyBytes was read in full; want the default cap")
	}
	if status, capped := postBody(t, Options{MaxBodyBytes: -1}, int(DefaultMaxBodyBytes)+1); status != http.StatusNoContent || capped {
		t.Fatalf("disabled cap: status = %d, capped = %v; want 204, false", status, capped)
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
