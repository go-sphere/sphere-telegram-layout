package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/user"
	serviceapi "github.com/go-sphere/sphere-telegram-layout/internal/service/api"
	"github.com/go-sphere/sphere/cache/memory"
	spherefile "github.com/go-sphere/sphere/server/service/file"
	"github.com/go-sphere/sphere/utils/secure"
)

func TestPasswordAuthenticationFlow(t *testing.T) {
	addr, baseURL := reserveAPIAddress(t)
	db := newAPITestDatabase(t)
	store, err := spherefile.NewLocalFileService(spherefile.LocalFileServiceConfig{
		RootDir:    t.TempDir(),
		PublicBase: baseURL + "/files",
	})
	if err != nil {
		t.Fatalf("create test storage: %v", err)
	}
	service := serviceapi.NewService(dao.NewDao(db), memory.NewByteCache(), store)
	web := NewWebServer(Config{
		JWT: "api-test-secret",
		HTTP: HTTPConfig{
			Address: addr,
		},
	}, store, service)

	startErr := make(chan error, 1)
	go func() {
		startErr <- web.Start(t.Context())
	}()
	waitForAPI(t, baseURL+"/api/status")
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = web.Stop(stopCtx)
		select {
		case err := <-startErr:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("server stopped with error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})

	const password = "correct-horse-battery-staple"
	status, body := apiJSONRequest(t, http.MethodPost, baseURL+"/api/auth/register", map[string]string{
		"username": " Alice ",
		"password": password,
	}, "")
	if status != http.StatusOK {
		t.Fatalf("register status = %d, want %d, body=%s", status, http.StatusOK, body)
	}
	registration := decodePasswordAuth(t, body)
	if registration.Data.Token == "" || registration.Data.User.Username != "alice" {
		t.Fatalf("unexpected register response: %+v", registration.Data)
	}
	if strings.Contains(body, password) || strings.Contains(strings.ToLower(body), "password") {
		t.Fatalf("register response leaked password data: %s", body)
	}

	registered, err := db.User.Query().Where(user.UsernameEQ("alice")).Only(t.Context())
	if err != nil {
		t.Fatalf("query registered user: %v", err)
	}
	if registered.Password == password || !secure.IsPasswordMatch(password, registered.Password) {
		t.Fatal("registered password was not stored as a matching hash")
	}

	status, _ = apiJSONRequest(t, http.MethodPost, baseURL+"/api/auth/register", map[string]string{
		"username": "ALICE",
		"password": password,
	}, "")
	if status != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want %d", status, http.StatusConflict)
	}

	status, _ = apiJSONRequest(t, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": "alice",
		"password": "wrong-password",
	}, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want %d", status, http.StatusUnauthorized)
	}

	status, body = apiJSONRequest(t, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": " ALICE ",
		"password": password,
	}, "")
	if status != http.StatusOK {
		t.Fatalf("login status = %d, want %d, body=%s", status, http.StatusOK, body)
	}
	login := decodePasswordAuth(t, body)
	if login.Data.Token == "" {
		t.Fatalf("login returned an empty token: %s", body)
	}

	status, _ = apiJSONRequest(t, http.MethodGet, baseURL+"/api/user/me", nil, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated me status = %d, want %d", status, http.StatusUnauthorized)
	}
	status, body = apiJSONRequest(t, http.MethodGet, baseURL+"/api/user/me", nil, login.Data.Token)
	if status != http.StatusOK || !strings.Contains(body, `"username":"alice"`) {
		t.Fatalf("authenticated me status = %d, body=%s", status, body)
	}
}

type passwordAuthEnvelope struct {
	Data struct {
		Token string `json:"token"`
		User  struct {
			Username string `json:"username"`
		} `json:"user"`
	} `json:"data"`
}

func decodePasswordAuth(t *testing.T, body string) passwordAuthEnvelope {
	t.Helper()
	var response passwordAuthEnvelope
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("decode auth response: %v, body=%s", err, body)
	}
	return response
}

func newAPITestDatabase(t *testing.T) *ent.Client {
	t.Helper()
	db, err := client.NewDataBaseClient(client.Config{
		Type: "sqlite3",
		Path: fmt.Sprintf("file:api-web-test-%d?mode=memory&cache=shared", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	return db
}

func reserveAPIAddress(t *testing.T) (string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved address: %v", err)
	}
	return addr, "http://" + addr
}

func waitForAPI(t *testing.T, target string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(target)
		if err == nil {
			_ = response.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("API did not become ready at %s", target)
}

func apiJSONRequest(t *testing.T, method, target string, payload any, token string) (int, string) {
	t.Helper()
	var requestBody io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		requestBody = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, target, requestBody)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return response.StatusCode, string(raw)
}
