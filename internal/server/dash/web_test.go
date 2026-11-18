package dash

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	dashv1 "github.com/go-sphere/sphere-telegram-layout/api/dash/v1"
	"github.com/go-sphere/sphere-telegram-layout/api/entpb"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/adminsession"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/httpsrv"
	servicedash "github.com/go-sphere/sphere-telegram-layout/internal/service/dash"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
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

	t.Run("admin without roles cannot reach management services", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		root := loginTokens(t, baseURL, testAdminUsername, testAdminPassword)
		status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/admin/create", map[string]any{
			"admin": map[string]any{"username": "NoRoles", "password": "NoRoles#123"},
		}, map[string]string{"Authorization": "Bearer " + root.AccessToken})
		if status != http.StatusOK {
			t.Fatalf("create admin status = %d, body=%s", status, body)
		}

		// The username is normalized on create, so login matches it exactly.
		limited := loginTokens(t, baseURL, "noroles", "NoRoles#123")
		bearer := map[string]string{"Authorization": "Bearer " + limited.AccessToken}
		for _, target := range []string{"/api/key-value-store/list", "/api/admin/list"} {
			if status, body := doJSONRequest(t, http.MethodGet, baseURL+target, nil, bearer); status != http.StatusForbidden {
				t.Errorf("GET %s status = %d, want %d, body=%s", target, status, http.StatusForbidden, body)
			}
		}
		if status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/cache/reset", map[string]any{}, bearer); status != http.StatusForbidden {
			t.Errorf("POST /api/cache/reset status = %d, want %d, body=%s", status, http.StatusForbidden, body)
		}
		if status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/upload/token", map[string]any{"filename": "test.txt"}, bearer); status != http.StatusForbidden {
			t.Errorf("POST /api/upload/token status = %d, want %d, body=%s", status, http.StatusForbidden, body)
		}
		// TestService stays on needAuthRoute while StorageService requires PermissionAdmin.
		if status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/test/body_path_test", map[string]any{"field_test1": "ok"}, bearer); status != http.StatusOK {
			t.Errorf("POST /api/test/body_path_test status = %d, want %d, body=%s", status, http.StatusOK, body)
		}
		if status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/upload/token", map[string]any{"filename": "test.txt"}, map[string]string{"Authorization": "Bearer " + root.AccessToken}); status != http.StatusOK {
			t.Errorf("root POST /api/upload/token status = %d, want %d, body=%s", status, http.StatusOK, body)
		}
	})

	t.Run("updating self preserves existing roles while updating another admin changes roles", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		root := loginTokens(t, baseURL, testAdminUsername, testAdminPassword)
		rootBearer := map[string]string{"Authorization": "Bearer " + root.AccessToken}

		createAdmin := func(username, password string, roles []string) int64 {
			t.Helper()
			status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/admin/create", map[string]any{
				"admin": map[string]any{"username": username, "password": password, "roles": roles},
			}, rootBearer)
			if status != http.StatusOK {
				t.Fatalf("create %s status = %d, body=%s", username, status, body)
			}
			var created struct {
				Data struct {
					Admin struct {
						ID int64 `json:"id"`
					} `json:"admin"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(body), &created); err != nil || created.Data.Admin.ID == 0 {
				t.Fatalf("parse created admin: %v, body=%s", err, body)
			}
			return created.Data.Admin.ID
		}

		editorID := createAdmin("editor", "Editor#123", []string{servicedash.PermissionAdmin})
		otherID := createAdmin("other", "Other#123", []string{})

		editor := loginTokens(t, baseURL, "editor", "Editor#123")
		editorBearer := map[string]string{"Authorization": "Bearer " + editor.AccessToken}

		for _, attemptedRoles := range [][]string{{servicedash.PermissionAll}, {}} {
			status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/admin/update", map[string]any{
				"admin": map[string]any{
					"id":       editorID,
					"username": "editor",
					"nickname": "Updated Editor",
					"roles":    attemptedRoles,
				},
			}, editorBearer)
			if status != http.StatusOK {
				t.Fatalf("self update status = %d, body=%s", status, body)
			}
			var updated struct {
				Data struct {
					Admin struct {
						Nickname string   `json:"nickname"`
						Roles    []string `json:"roles"`
					} `json:"admin"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(body), &updated); err != nil {
				t.Fatalf("parse self update response: %v, body=%s", err, body)
			}
			if updated.Data.Admin.Nickname != "Updated Editor" {
				t.Fatalf("self update nickname = %q, want %q", updated.Data.Admin.Nickname, "Updated Editor")
			}
			if !slices.Equal(updated.Data.Admin.Roles, []string{servicedash.PermissionAdmin}) {
				t.Fatalf("self update changed roles to %v (attempted %v), want [%s]", updated.Data.Admin.Roles, attemptedRoles, servicedash.PermissionAdmin)
			}
		}

		// Updating another admin's roles succeeds and takes effect on subsequent login.
		status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/admin/update", map[string]any{
			"admin": map[string]any{
				"id":       otherID,
				"username": "other",
				"roles":    []string{servicedash.PermissionAdmin},
			},
		}, rootBearer)
		if status != http.StatusOK {
			t.Fatalf("update other admin status = %d, body=%s", status, body)
		}
		var updatedOther struct {
			Data struct {
				Admin struct {
					Roles []string `json:"roles"`
				} `json:"admin"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(body), &updatedOther); err != nil {
			t.Fatalf("parse update other response: %v, body=%s", err, body)
		}
		if !slices.Equal(updatedOther.Data.Admin.Roles, []string{servicedash.PermissionAdmin}) {
			t.Fatalf("other admin roles = %v, want [%s]", updatedOther.Data.Admin.Roles, servicedash.PermissionAdmin)
		}
		otherTokens := loginTokens(t, baseURL, "other", "Other#123")
		if status, body := doJSONRequest(t, http.MethodGet, baseURL+"/api/admin/list", nil, map[string]string{
			"Authorization": "Bearer " + otherTokens.AccessToken,
		}); status != http.StatusOK {
			t.Fatalf("promoted admin list status = %d, want 200, body=%s", status, body)
		}
	})

	t.Run("password change revokes refresh sessions", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		tokens := loginTokens(t, baseURL, testAdminUsername, testAdminPassword)
		bearer := map[string]string{"Authorization": "Bearer " + tokens.AccessToken}
		status, body := doJSONRequest(t, http.MethodGet, baseURL+"/api/admin/list", nil, bearer)
		if status != http.StatusOK {
			t.Fatalf("list admins status = %d, body=%s", status, body)
		}
		var list struct {
			Data struct {
				Admins []struct {
					ID int64 `json:"id"`
				} `json:"admins"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(body), &list); err != nil || len(list.Data.Admins) == 0 {
			t.Fatalf("parse admin list: %v, body=%s", err, body)
		}
		status, body = doJSONRequest(t, http.MethodPost, baseURL+"/api/admin/update", map[string]any{
			"admin": map[string]any{"id": list.Data.Admins[0].ID, "username": testAdminUsername, "password": "Changed#123"},
		}, bearer)
		if status != http.StatusOK {
			t.Fatalf("update admin status = %d, body=%s", status, body)
		}
		status, body = doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/refresh", map[string]string{
			"refresh_token": tokens.RefreshToken,
		}, nil)
		if status == http.StatusOK {
			t.Fatalf("refresh after password change succeeded, body=%s", body)
		}
		// Omitting roles during password update must keep existing roles intact.
		newTokens := loginTokens(t, baseURL, testAdminUsername, "Changed#123")
		status, body = doJSONRequest(t, http.MethodGet, baseURL+"/api/admin/list", nil, map[string]string{
			"Authorization": "Bearer " + newTokens.AccessToken,
		})
		if status != http.StatusOK {
			t.Fatalf("admin list after password update status = %d, want 200, body=%s", status, body)
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

	t.Run("unusable refresh token is rejected with 401", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		refresher := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]]("test-refresh-jwt-secret")
		sign := func(sessionID int64, expires time.Time) string {
			t.Helper()
			token, err := refresher.GenerateToken(t.Context(), jwtauth.NewRBACClaims(sessionID, "session-key", nil, expires))
			if err != nil {
				t.Fatalf("sign refresh token: %v", err)
			}
			return token
		}
		for name, token := range map[string]string{
			"malformed":       "not-a-jwt",
			"expired":         sign(1, time.Now().Add(-time.Hour)),
			"unknown session": sign(999999, time.Now().Add(time.Hour)),
		} {
			status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/refresh", map[string]string{
				"refresh_token": token,
			}, nil)
			if status != http.StatusUnauthorized {
				t.Errorf("%s refresh token: status = %d, want %d, body=%s", name, status, http.StatusUnauthorized, body)
			}
		}
	})
}

// TestWebAuthCookie covers the browser auth path: login and refresh write the
// auth_token cookie and the auth middleware accepts it on requests that carry
// no Authorization header.
func TestWebAuthCookie(t *testing.T) {
	baseURL, _, cleanup := setupTestWeb(t)
	defer cleanup()

	base, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url failed: %v", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar failed: %v", err)
	}
	httpClient := &http.Client{Timeout: time.Second * 5, Jar: jar}

	status, header, body := doJSONRequestWithClient(t, httpClient, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": testAdminUsername,
		"password": testAdminPassword,
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body=%s", status, body)
	}
	tokens := parseAuthTokens(t, body)

	authCookie := authCookieFromHeader(header)
	if authCookie == nil {
		t.Fatalf("login response missing %s cookie, headers=%v", servicedash.AuthTokenCookieName, header)
	}
	if authCookie.Value != tokens.AccessToken {
		t.Fatalf("cookie value does not match the access token, value_len=%d token_len=%d", len(authCookie.Value), len(tokens.AccessToken))
	}
	maxAge := int(servicedash.AuthTokenValidDuration.Seconds())
	if authCookie.Path != "/" || authCookie.MaxAge != maxAge || authCookie.SameSite != http.SameSiteLaxMode || !authCookie.HttpOnly || authCookie.Secure {
		t.Fatalf("cookie attrs = path %q max-age %d samesite %v http-only %v secure %v, want /, %d, Lax, true, false",
			authCookie.Path, authCookie.MaxAge, authCookie.SameSite, authCookie.HttpOnly, authCookie.Secure, maxAge)
	}

	// The jar replays the cookie; these requests send no Authorization header.
	status, _, body = doJSONRequestWithClient(t, httpClient, http.MethodGet, baseURL+"/api/admin/list", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("cookie-authenticated admin list status = %d, want 200, body=%s", status, body)
	}

	// Refresh rotates the access token and must rotate the cookie with it.
	status, _, body = doJSONRequestWithClient(t, httpClient, http.MethodPost, baseURL+"/api/auth/refresh", map[string]string{
		"refresh_token": tokens.RefreshToken,
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200, body=%s", status, body)
	}
	refreshed := parseAuthTokens(t, body)
	var rotated string
	for _, cookie := range jar.Cookies(base) {
		if cookie.Name == servicedash.AuthTokenCookieName {
			rotated = cookie.Value
		}
	}
	if rotated != refreshed.AccessToken {
		t.Fatalf("refresh did not rotate the auth cookie, value_len=%d token_len=%d", len(rotated), len(refreshed.AccessToken))
	}

	// A bogus cookie must not authenticate, mirroring the invalid-Bearer case.
	badJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar failed: %v", err)
	}
	badJar.SetCookies(base, []*http.Cookie{{Name: servicedash.AuthTokenCookieName, Value: "not-a-jwt", Path: "/"}})
	status, _, body = doJSONRequestWithClient(t, &http.Client{Timeout: time.Second * 5, Jar: badJar}, http.MethodGet, baseURL+"/api/admin/list", nil, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("invalid cookie status = %d, want 401, body=%s", status, body)
	}

	// Behind a TLS-terminating proxy the cookie must be marked Secure.
	status, header, body = doJSONRequestWithClient(t, httpClient, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": testAdminUsername,
		"password": testAdminPassword,
	}, map[string]string{"X-Forwarded-Proto": "https"})
	if status != http.StatusOK {
		t.Fatalf("https login status = %d, want 200, body=%s", status, body)
	}
	secureCookie := authCookieFromHeader(header)
	if secureCookie == nil || !secureCookie.Secure {
		t.Fatalf("cookie behind X-Forwarded-Proto=https is not Secure, cookie=%+v", secureCookie)
	}
}

// TestWebLogoutClearsAuthCookie covers server-side logout: it clears the
// HttpOnly auth_token cookie, revokes the refresh session (401 on subsequent
// refresh), and succeeds even when refresh_token is omitted.
func TestWebLogoutClearsAuthCookie(t *testing.T) {
	baseURL, _, cleanup := setupTestWeb(t)
	defer cleanup()

	base, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url failed: %v", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar failed: %v", err)
	}
	httpClient := &http.Client{Timeout: time.Second * 5, Jar: jar}

	status, _, body := doJSONRequestWithClient(t, httpClient, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": testAdminUsername,
		"password": testAdminPassword,
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body=%s", status, body)
	}
	tokens := parseAuthTokens(t, body)

	status, _, body = doJSONRequestWithClient(t, httpClient, http.MethodGet, baseURL+"/api/admin/list", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("cookie-authenticated admin list status = %d, want 200, body=%s", status, body)
	}

	status, header, body := doJSONRequestWithClient(t, httpClient, http.MethodPost, baseURL+"/api/auth/logout", map[string]string{
		"refresh_token": tokens.RefreshToken,
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("logout status = %d, want 200, body=%s", status, body)
	}
	if cleared := authCookieFromHeader(header); cleared == nil || cleared.Value != "" || cleared.MaxAge > 0 {
		t.Fatalf("logout did not clear the auth cookie, cookie=%+v", cleared)
	}
	for _, cookie := range jar.Cookies(base) {
		if cookie.Name == servicedash.AuthTokenCookieName {
			t.Fatalf("auth cookie survived logout: %+v", cookie)
		}
	}
	status, _, body = doJSONRequestWithClient(t, httpClient, http.MethodGet, baseURL+"/api/admin/list", nil, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("post-logout admin list status = %d, want 401, body=%s", status, body)
	}

	// Logout revokes the refresh session as well, so the token handed in above
	// cannot mint another access token (401 ADMIN_SESSION_ERROR_REVOKED).
	status, _, body = doJSONRequestWithClient(t, httpClient, http.MethodPost, baseURL+"/api/auth/refresh", map[string]string{
		"refresh_token": tokens.RefreshToken,
	}, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("refresh after logout status = %d, want 401, body=%s", status, body)
	}

	// No refresh token at all: the cookie still has to go.
	status, _, body = doJSONRequestWithClient(t, httpClient, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": testAdminUsername,
		"password": testAdminPassword,
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("re-login status = %d, want 200, body=%s", status, body)
	}
	status, header, body = doJSONRequestWithClient(t, httpClient, http.MethodPost, baseURL+"/api/auth/logout", map[string]string{}, nil)
	if status != http.StatusOK {
		t.Fatalf("token-less logout status = %d, want 200, body=%s", status, body)
	}
	if cleared := authCookieFromHeader(header); cleared == nil || cleared.Value != "" || cleared.MaxAge > 0 {
		t.Fatalf("token-less logout did not clear the auth cookie, cookie=%+v", cleared)
	}
}

func authCookieFromHeader(header http.Header) *http.Cookie {
	for _, cookie := range (&http.Response{Header: header}).Cookies() {
		if cookie.Name == servicedash.AuthTokenCookieName {
			return cookie
		}
	}
	return nil
}

// TestWebLogin_PersistsSessionMetadata guards the hand-off between
// NewSessionMetaData and the service: the IP and User-Agent must travel
// through the standard context, or the stored session ends up without them.
func TestWebLogin_PersistsSessionMetadata(t *testing.T) {
	baseURL, _, db, cleanup := setupTestWebWithDB(t)
	defer cleanup()

	const userAgent = "sphere-telegram-layout-test/1.0"
	status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": testAdminUsername,
		"password": testAdminPassword,
	}, map[string]string{
		"User-Agent": userAgent,
	})
	if status != http.StatusOK {
		t.Fatalf("login status = %d, want %d, body=%s", status, http.StatusOK, body)
	}

	session, err := db.AdminSession.Query().Only(t.Context())
	if err != nil {
		t.Fatalf("query admin session: %v", err)
	}
	if session.IPAddress != "127.0.0.1" {
		t.Fatalf("session ip_address = %q, want %q", session.IPAddress, "127.0.0.1")
	}
	if session.DeviceInfo != userAgent {
		t.Fatalf("session device_info = %q, want %q", session.DeviceInfo, userAgent)
	}
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

func TestWebRejectsOversizedBodyWith413(t *testing.T) {
	const limit = 1 << 10
	baseURL, _, _, cleanup := setupTestWebWithOptions(t, httpsrv.Options{MaxBodyBytes: limit})
	defer cleanup()

	status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": strings.Repeat("a", limit),
		"password": testAdminPassword,
	}, nil)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d, body=%s", status, http.StatusRequestEntityTooLarge, body)
	}
}

func setupTestWeb(t *testing.T) (string, *fileserver.FileServer, func()) {
	t.Helper()

	baseURL, fileServer, _, cleanup := setupTestWebWithDB(t)
	return baseURL, fileServer, cleanup
}

func setupTestWebWithDB(t *testing.T) (string, *fileserver.FileServer, *ent.Client, func()) {
	t.Helper()

	return setupTestWebWithOptions(t, httpsrv.Options{})
}

func setupTestWebWithOptions(t *testing.T, opts httpsrv.Options) (string, *fileserver.FileServer, *ent.Client, func()) {
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
			Options: opts,
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
	return baseURL, fileServer, db, cleanup
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

func loginTokens(t *testing.T, baseURL, username, password string) authTokenData {
	t.Helper()
	status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("login %q status = %d, body=%s", username, status, body)
	}
	return parseAuthTokens(t, body)
}

func doJSONRequest(t *testing.T, method, target string, payload any, headers map[string]string) (int, string) {
	t.Helper()

	status, _, body := doJSONRequestWithClient(t, &http.Client{Timeout: time.Second * 5}, method, target, payload, headers)
	return status, body
}

func doJSONRequestWithClient(t *testing.T, client *http.Client, method, target string, payload any, headers map[string]string) (int, http.Header, string) {
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

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return resp.StatusCode, resp.Header, string(raw)
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

func TestWebValidationPaginationRateLimitAndExpiredSession(t *testing.T) {
	t.Run("create and update admin validation rejects invalid password and whitespace username", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		root := loginTokens(t, baseURL, testAdminUsername, testAdminPassword)
		bearer := map[string]string{"Authorization": "Bearer " + root.AccessToken}

		for _, tc := range []struct {
			name string
			path string
			body map[string]any
		}{
			{
				name: "create empty password",
				path: "/api/admin/create",
				body: map[string]any{"admin": map[string]any{"username": "u1", "password": ""}},
			},
			{
				name: "create >72 byte password",
				path: "/api/admin/create",
				body: map[string]any{"admin": map[string]any{"username": "u1", "password": strings.Repeat("a", 73)}},
			},
			{
				name: "create whitespace username",
				path: "/api/admin/create",
				body: map[string]any{"admin": map[string]any{"username": "   ", "password": "ValidPassword#1"}},
			},
			{
				name: "update >72 byte password",
				path: "/api/admin/update",
				body: map[string]any{"admin": map[string]any{"id": 1, "username": testAdminUsername, "password": strings.Repeat("a", 73)}},
			},
			{
				name: "update whitespace username",
				path: "/api/admin/update",
				body: map[string]any{"admin": map[string]any{"id": 1, "username": "   "}},
			},
		} {
			status, body := doJSONRequest(t, http.MethodPost, baseURL+tc.path, tc.body, bearer)
			if status != http.StatusBadRequest {
				t.Errorf("%s: status = %d, want 400, body=%s", tc.name, status, body)
			}
		}

		for _, path := range []string{"/api/key-value-store/create", "/api/key-value-store/update"} {
			status, body := doJSONRequest(t, http.MethodPost, baseURL+path, map[string]any{}, bearer)
			if status != http.StatusBadRequest {
				t.Errorf("POST %s empty body: status = %d, want 400, body=%s", path, status, body)
			}
		}
	})

	t.Run("out-of-range page and session key scrubbing", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		root := loginTokens(t, baseURL, testAdminUsername, testAdminPassword)
		bearer := map[string]string{"Authorization": "Bearer " + root.AccessToken}

		for _, path := range []string{
			fmt.Sprintf("/api/admin/list?page=%d&page_size=9999", math.MaxInt64),
			fmt.Sprintf("/api/admin-session/list?page=%d&page_size=9999", math.MaxInt64),
			fmt.Sprintf("/api/key-value-store/list?page=%d&page_size=9999", math.MaxInt64),
		} {
			status, body := doJSONRequest(t, http.MethodGet, baseURL+path, nil, bearer)
			if status != http.StatusOK {
				t.Errorf("GET %s status = %d, want 200, body=%s", path, status, body)
			}
		}

		status, body := doJSONRequest(t, http.MethodGet, baseURL+"/api/admin-session/list?page=0&page_size=10", nil, bearer)
		if status != http.StatusOK {
			t.Fatalf("list sessions status = %d, body=%s", status, body)
		}
		if !strings.Contains(body, `"admin_sessions":[{`) {
			t.Fatalf("expected non-empty admin_sessions list, body=%s", body)
		}
		if strings.Contains(body, `"session_key"`) {
			t.Fatalf("session list leaked session_key: %s", body)
		}
	})

	t.Run("refresh endpoint is rate limited", func(t *testing.T) {
		baseURL, _, cleanup := setupTestWeb(t)
		defer cleanup()

		limited := false
		for range 10 {
			status, _ := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/refresh", map[string]string{
				"refresh_token": "not-a-jwt",
			}, nil)
			if status == http.StatusTooManyRequests {
				limited = true
				break
			}
		}
		if !limited {
			t.Fatal("POST /api/auth/refresh was not rate limited")
		}
	})

	t.Run("expired session revocation persists in DB", func(t *testing.T) {
		baseURL, _, db, cleanup := setupTestWebWithDB(t)
		defer cleanup()

		tokens := loginTokens(t, baseURL, testAdminUsername, testAdminPassword)
		sess, err := db.AdminSession.Query().Only(t.Context())
		if err != nil {
			t.Fatalf("query session: %v", err)
		}
		if _, err := db.ExecContext(
			t.Context(),
			"UPDATE "+adminsession.Table+" SET "+adminsession.FieldExpires+" = ? WHERE "+adminsession.FieldID+" = ?",
			time.Now().Add(-time.Minute).Unix(),
			sess.ID,
		); err != nil {
			t.Fatalf("expire session in DB: %v", err)
		}

		status, body := doJSONRequest(t, http.MethodPost, baseURL+"/api/auth/refresh", map[string]string{
			"refresh_token": tokens.RefreshToken,
		}, nil)
		if status != http.StatusUnauthorized {
			t.Fatalf("expired refresh status = %d, want 401, body=%s", status, body)
		}

		reloaded, err := db.AdminSession.Get(t.Context(), sess.ID)
		if err != nil {
			t.Fatalf("reload session: %v", err)
		}
		if !reloaded.IsRevoked {
			t.Fatal("expired session was not persisted as revoked")
		}
	})

	t.Run("update admin normalizes avatar key via storage", func(t *testing.T) {
		db := newMemoryDB(t)
		defer func() { _ = db.Close() }()
		insertDefaultAdmin(t, db)
		adm, err := db.Admin.Query().Only(t.Context())
		if err != nil {
			t.Fatalf("query admin: %v", err)
		}
		svc := servicedash.NewService(dao.NewDao(db), memory.NewByteCache(), &prefixStripStorage{prefix: "https://cdn.example/"})
		resp, err := svc.UpdateAdmin(t.Context(), &dashv1.UpdateAdminRequest{
			Admin: &entpb.Admin{
				Id:       adm.ID,
				Username: " Admin ",
				Avatar:   "https://cdn.example/avatars/a.png",
			},
		})
		if err != nil {
			t.Fatalf("UpdateAdmin() error = %v", err)
		}
		if resp.Admin.Avatar != "https://cdn.example/avatars/a.png" {
			t.Fatalf("rendered avatar = %q", resp.Admin.Avatar)
		}
		reloaded, err := db.Admin.Get(t.Context(), adm.ID)
		if err != nil {
			t.Fatalf("reload admin: %v", err)
		}
		if reloaded.Avatar != "avatars/a.png" {
			t.Fatalf("stored avatar key = %q, want avatars/a.png", reloaded.Avatar)
		}
	})
}

type prefixStripStorage struct {
	prefix string
}

func (p *prefixStripStorage) GenerateURL(key string, _ ...url.Values) string {
	if key == "" {
		return ""
	}
	return p.prefix + key
}

func (p *prefixStripStorage) GenerateURLs(keys []string, values ...url.Values) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = p.GenerateURL(k, values...)
	}
	return out
}

func (p *prefixStripStorage) ExtractKeyFromURL(uri string) string {
	return strings.TrimPrefix(uri, p.prefix)
}

func (p *prefixStripStorage) ExtractKeyFromURLWithMode(uri string, _ bool) (string, error) {
	return p.ExtractKeyFromURL(uri), nil
}

func (p *prefixStripStorage) GenerateUploadAuth(context.Context, storage.UploadAuthRequest) (storage.UploadAuthResult, error) {
	return storage.UploadAuthResult{}, nil
}

func (p *prefixStripStorage) UploadFile(context.Context, io.Reader, string) (string, error) {
	return "", nil
}

func (p *prefixStripStorage) UploadLocalFile(context.Context, string, string) (string, error) {
	return "", nil
}

func (p *prefixStripStorage) IsFileExists(context.Context, string) (bool, error) { return false, nil }

func (p *prefixStripStorage) DownloadFile(context.Context, string) (storage.DownloadResult, error) {
	return storage.DownloadResult{}, errors.New("not implemented")
}

func (p *prefixStripStorage) DeleteFile(context.Context, string) error             { return nil }
func (p *prefixStripStorage) MoveFile(context.Context, string, string, bool) error { return nil }
func (p *prefixStripStorage) CopyFile(context.Context, string, string, bool) error { return nil }
