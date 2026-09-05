package dash

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	servicedash "github.com/go-sphere/sphere-telegram-layout/internal/service/dash"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/utils/secure"
)

const (
	testAdminUsername = "admin"
	testAdminPassword = "aA1234567"
)

func TestWebAuthAndAdminEndpoints(t *testing.T) {
	t.Run("default credentials should return token", func(t *testing.T) {
		baseURL, cleanup := setupTestWeb(t)
		defer cleanup()

		status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/login", map[string]string{
			"username": testAdminUsername,
			"password": testAdminPassword,
		}, nil)
		if status != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body=%s", status, body)
		}

		token := parseLoginToken(t, body)
		if token == "" {
			t.Fatalf("expected non-empty accessToken, body=%s", body)
		}
	})

	t.Run("wrong credentials should not return token", func(t *testing.T) {
		baseURL, cleanup := setupTestWeb(t)
		defer cleanup()

		status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/login", map[string]string{
			"username": "wrong-user",
			"password": "wrong-password",
		}, nil)
		if status != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d, body=%s", status, http.StatusUnauthorized, body)
		}
	})

	t.Run("valid token should get admin list", func(t *testing.T) {
		baseURL, cleanup := setupTestWeb(t)
		defer cleanup()

		loginStatus, loginBody := doJSONRequest(t, http.MethodPost, baseURL+"/api/login", map[string]string{
			"username": testAdminUsername,
			"password": testAdminPassword,
		}, nil)
		if loginStatus != http.StatusOK {
			t.Fatalf("login status = %d, want %d, body=%s", loginStatus, http.StatusOK, loginBody)
		}
		token := parseLoginToken(t, loginBody)
		if token == "" {
			t.Fatalf("expected login token, body=%s", loginBody)
		}

		status, body := doJSONRequest(t, http.MethodGet, baseURL+"/api/admin/list", nil, map[string]string{
			"Authorization": "Bearer " + token,
		})
		if status != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body=%s", status, body)
		}

		count := parseAdminCount(t, body)
		if count == 0 {
			t.Fatalf("expected non-empty admin list, body=%s", body)
		}
	})

	t.Run("invalid token should not get admin list", func(t *testing.T) {
		baseURL, cleanup := setupTestWeb(t)
		defer cleanup()

		status, body := doJSONRequest(t, http.MethodGet, baseURL+"/api/admin/list", nil, map[string]string{
			"Authorization": "Bearer invalid-token",
		})
		if status != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d, body=%s", status, http.StatusUnauthorized, body)
		}
	})
}

func setupTestWeb(t *testing.T) (string, func()) {
	t.Helper()

	addr := randomLocalAddress(t)
	db := newMemoryDB(t)
	insertDefaultAdmin(t, db)

	testStorage := &noopStorage{}
	service := servicedash.NewService(dao.NewDao(db), memory.NewByteCache(), testStorage)
	web := NewWebServer(Config{
		AuthJWT:    "test-auth-jwt-secret",
		RefreshJWT: "test-refresh-jwt-secret",
		HTTP: HTTPConfig{
			Address: addr,
		},
	}, testStorage, service)

	startErr := make(chan error, 1)
	go func() {
		startErr <- web.Start(context.Background())
	}()

	baseURL := "http://" + addr
	waitServerReady(t, baseURL, startErr)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		_ = web.Stop(ctx)
		_ = db.Close()
		select {
		case err := <-startErr:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Fatalf("web server exited with error: %v", err)
			}
		case <-time.After(time.Second):
		}
	}
	return baseURL, cleanup
}

func waitServerReady(t *testing.T, baseURL string, startErr <-chan error) {
	t.Helper()

	httpClient := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(time.Second * 5)
	for time.Now().Before(deadline) {
		select {
		case err := <-startErr:
			t.Fatalf("web server start failed: %v", err)
		default:
		}
		resp, err := httpClient.Get(baseURL + "/api/get-async-routes")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(time.Millisecond * 50)
	}
	t.Fatalf("web server did not become ready in time")
}

func randomLocalAddress(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on random port failed: %v", err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().String()
}

func newMemoryDB(t *testing.T) *ent.Client {
	t.Helper()

	conf := client.Config{
		Type: "sqlite3",
		Path: fmt.Sprintf("file:dash-web-test-%d?mode=memory&cache=shared", time.Now().UnixNano()),
	}
	db, err := client.NewDataBaseClient(conf)
	if err != nil {
		t.Fatalf("create test database failed: %v", err)
	}
	return db
}

func insertDefaultAdmin(t *testing.T, db *ent.Client) {
	t.Helper()

	password, err := secure.CryptPassword(testAdminPassword)
	if err != nil {
		t.Fatalf("crypt password failed: %v", err)
	}
	_, err = db.Admin.Create().
		SetUsername(testAdminUsername).
		SetPassword(password).
		SetRoles([]string{"all"}).
		Save(context.Background())
	if err != nil {
		t.Fatalf("insert admin failed: %v", err)
	}
}

func doJSONRequest(t *testing.T, method, target string, payload any, headers map[string]string) (int, string) {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload failed: %v", err)
		}
		body = bytes.NewBuffer(raw)
	}

	req, err := http.NewRequest(method, target, body)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := (&http.Client{Timeout: time.Second * 5}).Do(req)
	if err != nil {
		t.Fatalf("do request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return resp.StatusCode, string(raw)
}

func parseLoginToken(t *testing.T, body string) string {
	t.Helper()

	var resp struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode login response: %v, body=%s", err, body)
	}
	return resp.Data.AccessToken
}

func parseAdminCount(t *testing.T, body string) int {
	t.Helper()

	var resp struct {
		Data struct {
			Admins []any `json:"admins"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode admin list response: %v, body=%s", err, body)
	}
	return len(resp.Data.Admins)
}

type noopStorage struct{}

func (n *noopStorage) GenerateURL(key string, _ ...url.Values) string { return key }

func (n *noopStorage) GenerateURLs(keys []string, _ ...url.Values) []string { return keys }

func (n *noopStorage) ExtractKeyFromURL(uri string) string { return uri }

func (n *noopStorage) ExtractKeyFromURLWithMode(uri string, _ bool) (string, error) { return uri, nil }

func (n *noopStorage) GenerateUploadAuth(_ context.Context, req storage.UploadAuthRequest) (storage.UploadAuthResult, error) {
	return storage.UploadAuthResult{
		Authorization: storage.UploadAuthorization{
			Type:   storage.UploadAuthorizationTypeToken,
			Value:  "test-upload-token",
			Method: http.MethodPost,
		},
		File: storage.UploadFileInfo{
			Key: req.FileName,
			URL: req.FileName,
		},
	}, nil
}

func (n *noopStorage) UploadFile(_ context.Context, _ io.Reader, key string) (string, error) {
	return key, nil
}

func (n *noopStorage) UploadLocalFile(_ context.Context, _ string, key string) (string, error) {
	return key, nil
}

func (n *noopStorage) IsFileExists(_ context.Context, _ string) (bool, error) { return false, nil }

func (n *noopStorage) DownloadFile(_ context.Context, _ string) (storage.DownloadResult, error) {
	return storage.DownloadResult{}, errors.New("not implemented")
}

func (n *noopStorage) DeleteFile(_ context.Context, _ string) error { return nil }

func (n *noopStorage) MoveFile(_ context.Context, _, _ string, _ bool) error { return nil }

func (n *noopStorage) CopyFile(_ context.Context, _, _ string, _ bool) error { return nil }
