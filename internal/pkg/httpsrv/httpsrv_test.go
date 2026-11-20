package httpsrv

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/logbuffer"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/server/middleware/requestid"
)

func registerAcceptanceRoutes(r httpx.Router) {
	r.GET("/ping", httpz.WithJson(func(ctx httpx.Context) (string, error) {
		return "pong", nil
	}))
	r.GET("/boom", func(ctx httpx.Context) error {
		return httpx.NewNotFoundError("resource missing")
	})
	r.GET("/users/:id", httpz.WithJson(func(ctx httpx.Context) (map[string]string, error) {
		return map[string]string{"id": ctx.Param("id")}, nil
	}))
}

func doRequest(t *testing.T, engine httpx.Engine, target string) (int, map[string]any) {
	t.Helper()
	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		t.Fatalf("engine %T does not support in-process test requests", engine)
	}
	resp, err := tr.Do(httptest.NewRequest(http.MethodGet, target, nil))
	if err != nil {
		t.Fatalf("request %s failed: %v", target, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("response %s is not JSON: %v; body=%q", target, err, body)
	}
	return resp.StatusCode, payload
}

func TestServerServesJSONRoutes(t *testing.T) {
	engine := NewServer("test", "127.0.0.1:0", Options{})
	registerAcceptanceRoutes(engine.Group(""))

	status, payload := doRequest(t, engine, "http://example.com/ping")
	if status != http.StatusOK {
		t.Fatalf("/ping status = %d, want 200", status)
	}
	if payload["data"] != "pong" {
		t.Fatalf("/ping payload = %v, want pong", payload)
	}

	status, payload = doRequest(t, engine, "http://example.com/users/42")
	if status != http.StatusOK {
		t.Fatalf("/users/:id status = %d, want 200", status)
	}
	data, _ := payload["data"].(map[string]any)
	if data["id"] != "42" {
		t.Fatalf("/users/:id payload = %v, want id=42", payload)
	}

	status, payload = doRequest(t, engine, "http://example.com/boom")
	if status != http.StatusNotFound {
		t.Fatalf("error route status = %d, want 404", status)
	}
	if payload["success"] != false || payload["message"] != "resource missing" {
		t.Fatalf("error route payload = %v, want sphere error shape", payload)
	}
}

func TestServerPanicIsRecovered(t *testing.T) {
	buf := logbuffer.New(32)
	log.InitWithBackends(buf)
	t.Cleanup(func() {
		log.InitWithBackends(log.NewStdioBackend())
	})

	engine := NewServer("test", "127.0.0.1:0", Options{})
	engine.Group("").GET("/panic", func(httpx.Context) error {
		panic("boom from handler")
	})

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		t.Fatalf("engine %T does not support in-process test requests", engine)
	}
	resp, err := tr.Do(httptest.NewRequest(http.MethodGet, "http://example.com/panic", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}

	entries, _ := buf.History(0, 8, log.LevelDebug)
	found := false
	for _, entry := range entries {
		if entry.Level == "error" && entry.Message == "[Recovery from panic]" && entry.Attrs["error"] == "boom from handler" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("panic recovery log missing from buffer: %+v", entries)
	}
}

func TestUseCORSAttachesHeaders(t *testing.T) {
	engine := NewServer("test", "127.0.0.1:0", Options{})
	if err := UseCORS(engine, []string{"https://example.com"}); err != nil {
		t.Fatalf("UseCORS() error = %v", err)
	}
	registerAcceptanceRoutes(engine.Group(""))

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		t.Fatalf("engine %T does not support in-process test requests", engine)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/ping", nil)
	req.Header.Set("Origin", "https://example.com")
	resp, err := tr.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want https://example.com", got)
	}
}

func TestServerStopForceClosesHungRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	started := make(chan struct{})
	engine := NewServer("test", addr, Options{})
	engine.Group("").POST("/hang", func(ctx httpx.Context) error {
		close(started)
		<-ctx.Context().Done()
		return ctx.Context().Err()
	})

	errCh := make(chan error, 1)
	go func() { errCh <- engine.Start() }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !engine.IsRunning() {
		time.Sleep(10 * time.Millisecond)
	}
	if !engine.IsRunning() {
		t.Fatal("engine did not start")
	}

	clientDone := make(chan error, 1)
	go func() {
		resp, err := http.Post("http://"+addr+"/hang", "application/json", nil)
		if err != nil {
			clientDone <- err
			return
		}
		_ = resp.Body.Close()
		clientDone <- nil
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()
	if err := engine.Stop(ctx); err != nil {
		t.Fatalf("Stop = %v, want nil after force close", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Stop took %v, want timeout then Close", elapsed)
	}
	select {
	case err := <-clientDone:
		if err == nil {
			t.Fatal("hung client should error after force close")
		}
	case <-time.After(time.Second):
		t.Fatal("hung client still blocked after Stop")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start after Stop = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
}

func TestServerAssignsRequestID(t *testing.T) {
	buf := logbuffer.New(32)
	log.InitWithBackends(buf)
	t.Cleanup(func() {
		log.InitWithBackends(log.NewStdioBackend())
	})
	// No route is registered: the engine-level stack must cover a 404 too.
	send := func(opts Options, id string) string {
		t.Helper()
		engine := NewServer("test", "127.0.0.1:0", opts)
		tr, ok := httpx.AsTestRequester(engine)
		if !ok {
			t.Fatalf("engine %T does not support in-process test requests", engine)
		}
		req := httptest.NewRequest(http.MethodGet, "http://example.com/missing", nil)
		req.Header.Set(requestid.DefaultHeader, id)
		resp, err := tr.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		return resp.Header.Get(requestid.DefaultHeader)
	}

	generated := send(Options{}, "client-id.1")
	if generated == "" || generated == "client-id.1" {
		t.Fatalf("default: response header = %q, want a generated ID", generated)
	}
	trusted := Options{TrustRequestID: true}
	if got := send(trusted, "client-id.1"); got != "client-id.1" {
		t.Fatalf("TrustRequestID: response header = %q, want the caller's ID kept", got)
	}
	if got := send(trusted, "not a valid id"); got == "" || got == "not a valid id" {
		t.Fatalf("TrustRequestID with a malformed ID: response header = %q, want a generated ID", got)
	}

	entries, _ := buf.History(0, 8, log.LevelDebug)
	for _, entry := range entries {
		if entry.Message == "/missing" && entry.Attrs["request_id"] == generated {
			return
		}
	}
	t.Fatalf("access log entry with request_id %q missing from buffer: %+v", generated, entries)
}
