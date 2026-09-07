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
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	servicedash "github.com/go-sphere/sphere-telegram-layout/internal/service/dash"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/server/httpz"
	spherefile "github.com/go-sphere/sphere/server/service/file"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/fileserver"
	"github.com/go-sphere/sphere/utils/secure"
)

const (
	testAdminUsername = "admin"
	testAdminPassword = "aA1234567"
)

func TestWebAuthAndAdminEndpoints(t *testing.T) {
	t.Run("default credentials should return token", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
			"username": testAdminUsername,
			"password": testAdminPassword,
		}, nil)
		if status != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body=%s", status, body)
		}

		tokens := parseAuthTokens(t, body)
		if tokens.AccessToken == "" || tokens.RefreshToken == "" {
			t.Fatalf("expected non-empty access_token and refresh_token, body=%s", body)
		}
	})

	t.Run("wrong credentials should not return token", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
			"username": "wrong-user",
			"password": "wrong-password",
		}, nil)
		if status != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d, body=%s", status, http.StatusUnauthorized, body)
		}
	})

	t.Run("refresh rotates tokens and bearer accesses admin list", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		loginStatus, loginBody := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
			"username": testAdminUsername,
			"password": testAdminPassword,
		}, nil)
		if loginStatus != http.StatusOK {
			t.Fatalf("login status = %d, want %d, body=%s", loginStatus, http.StatusOK, loginBody)
		}
		tokens := parseAuthTokens(t, loginBody)
		if tokens.AccessToken == "" || tokens.RefreshToken == "" {
			t.Fatalf("expected login tokens, body=%s", loginBody)
		}

		refreshStatus, refreshBody := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/refresh", map[string]string{
			"refresh_token": tokens.RefreshToken,
		}, nil)
		if refreshStatus != http.StatusOK {
			t.Fatalf("refresh status = %d, want %d, body=%s", refreshStatus, http.StatusOK, refreshBody)
		}
		refreshed := parseAuthTokens(t, refreshBody)
		if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
			t.Fatalf("expected refreshed tokens, body=%s", refreshBody)
		}
		if refreshed.AccessToken == tokens.AccessToken || refreshed.RefreshToken == tokens.RefreshToken {
			t.Fatalf("refresh did not rotate tokens, body=%s", refreshBody)
		}

		status, body := doJSONRequest(t, http.MethodGet, baseURL+"/api/admin/list", nil, map[string]string{
			"Authorization": "Bearer " + refreshed.AccessToken,
		})
		if status != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body=%s", status, body)
		}

		count := parseAdminCount(t, body)
		if count == 0 {
			t.Fatalf("expected non-empty admin list, body=%s", body)
		}
	})

	t.Run("legacy pure-admin login path is gone", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/login", map[string]string{
			"username": testAdminUsername,
			"password": testAdminPassword,
		}, nil)
		if status == http.StatusOK {
			t.Fatalf("legacy /api/login still succeeded, body=%s", body)
		}
	})

	t.Run("invalid token should not get admin list", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		status, body := doJSONRequest(t, http.MethodGet, baseURL+"/api/admin/list", nil, map[string]string{
			"Authorization": "Bearer invalid-token",
		})
		if status != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d, body=%s", status, http.StatusUnauthorized, body)
		}
	})
}

func TestWebServer_TokenUploadDownloadFlow(t *testing.T) {
	_, fileServer, cleanup := setupTestWeb(t)
	defer cleanup()

	content := []byte("sphere-telegram-layout upload/download e2e")

	// Step 1: get upload token.
	authData, err := fileServer.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		Dir:      "user",
		FileName: "test.txt",
	})
	if err != nil {
		t.Fatalf("GenerateUploadAuth() error = %v", err)
	}
	if authData.Authorization.Value == "" {
		t.Fatal("GenerateUploadAuth() returned empty upload token url")
	}

	// Step 2: upload file with token url.
	uploadReq, err := http.NewRequest(http.MethodPut, authData.Authorization.Value, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("http.NewRequest(PUT) error = %v", err)
	}
	uploadResp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		t.Fatalf("upload request error = %v", err)
	}
	defer func() {
		_ = uploadResp.Body.Close()
	}()
	if uploadResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(uploadResp.Body)
		t.Fatalf("upload status = %d, body = %s", uploadResp.StatusCode, string(body))
	}
	uploadBody, err := io.ReadAll(uploadResp.Body)
	if err != nil {
		t.Fatalf("read upload body error = %v", err)
	}
	var uploadResult httpz.DataResponse[fileserver.UploadResult]
	if err = json.Unmarshal(uploadBody, &uploadResult); err != nil {
		t.Fatalf("decode upload response error = %v, body = %s", err, string(uploadBody))
	}
	if uploadResult.Data.Key != authData.File.Key {
		t.Fatalf("upload response key = %q, want %q", uploadResult.Data.Key, authData.File.Key)
	}
	if uploadResult.Data.URL != authData.File.URL {
		t.Fatalf("upload response url = %q, want %q", uploadResult.Data.URL, authData.File.URL)
	}

	// Step 3: download uploaded file.
	downloadResp, err := http.Get(authData.File.URL)
	if err != nil {
		t.Fatalf("download request error = %v", err)
	}
	defer func() {
		_ = downloadResp.Body.Close()
	}()
	if downloadResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(downloadResp.Body)
		t.Fatalf("download status = %d, url = %s, key = %s, body = %s", downloadResp.StatusCode, authData.File.URL, authData.File.Key, string(body))
	}
	downloaded, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(download) error = %v", err)
	}
	if !bytes.Equal(downloaded, content) {
		t.Fatalf("download content mismatch: got = %q, want = %q", string(downloaded), string(content))
	}
}

func setupTestWeb(t *testing.T) (string, *fileserver.FileServer, func()) {
	t.Helper()

	addr := randomLocalAddress(t)
	db := newMemoryDB(t)
	insertDefaultAdmin(t, db)

	baseURL := "http://" + addr
	fileServer, err := spherefile.NewLocalFileService(spherefile.LocalFileServiceConfig{
		RootDir:    t.TempDir(),
		PublicBase: baseURL + "/files",
	})
	if err != nil {
		t.Fatalf("create test storage: %v", err)
	}

	service := servicedash.NewService(dao.NewDao(db), memory.NewByteCache(), fileServer)
	web := NewWebServer(Config{
		AuthJWT:    "test-auth-jwt-secret",
		RefreshJWT: "test-refresh-jwt-secret",
		HTTP: HTTPConfig{
			Address: addr,
		},
	}, fileServer, service)

	startErr := make(chan error, 1)
	go func() {
		startErr <- web.Start(t.Context())
	}()

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
	return baseURL, fileServer, cleanup
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
		resp, err := httpClient.Get(baseURL + "/")
		if err == nil {
			_ = resp.Body.Close()
			return
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
		Save(t.Context())
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

	req, err := http.NewRequestWithContext(t.Context(), method, target, body)
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

type authTokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

func parseAuthTokens(t *testing.T, body string) authTokenData {
	t.Helper()

	var resp struct {
		Data authTokenData `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode auth response: %v, body=%s", err, body)
	}
	if strings.Contains(body, `"accessToken"`) || strings.Contains(body, `"refreshToken"`) {
		t.Fatalf("auth response still uses camelCase token fields, body=%s", body)
	}
	return resp.Data
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
