package dash

import (
	"context"
	"time"

	"github.com/go-sphere/httpx"
	dashv1 "github.com/go-sphere/sphere-telegram-layout/api/dash/v1"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/admin"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/adminsession"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
	"github.com/go-sphere/sphere/utils/secure"
	"github.com/google/uuid"
)

var _ dashv1.AuthServiceHTTPServer = (*Service)(nil)

const (
	AuthTokenValidDuration    = time.Hour
	RefreshTokenValidDuration = time.Hour * 24
)

// AuthTokenCookieName is the cookie carrying the access token for browser
// clients. The auth middleware loader reads it; LoginWithPassword and
// RefreshToken write it, so both sides must agree on the name.
const AuthTokenCookieName = "auth_token"

// AuthContextKey is the type of the keys this package exchanges through the
// standard context.Context. Built-in string keys are prohibited (staticcheck
// SA1029): the string key space is shared by every package, so two packages
// using "auth_ip" would silently overwrite each other.
//
// All three keys are injected by internal/server/dash.NewSessionMetaData
// through httpx SetContext, not StateStore: ctx.Set writes a channel the
// service layer cannot see.
type AuthContextKey string

const (
	AuthContextKeyIP           AuthContextKey = "auth_ip"
	AuthContextKeyUA           AuthContextKey = "auth_ua"
	AuthContextKeyCookieSetter AuthContextKey = "auth_cookie_setter"
)

// CookieSetter writes a Set-Cookie header on the current response. The server
// layer injects one into the context because services only see
// context.Context, not httpx.Context.
type CookieSetter func(name, value string, maxAge int)

// setAuthCookie stores the access token in the auth cookie through the
// injected setter. A missing setter (direct service calls, tests) just skips
// the cookie; the token is still returned in the response body.
func setAuthCookie(ctx context.Context, value string, maxAge int) {
	if setter, ok := ctx.Value(AuthContextKeyCookieSetter).(CookieSetter); ok && setter != nil {
		setter(AuthTokenCookieName, value, maxAge)
	}
}

// errInvalidRefreshToken answers a refresh token that is malformed, expired,
// signed with another key, or names a session or admin that no longer exists:
// the client must log in again, which 401 tells it.
var errInvalidRefreshToken = httpx.NewUnauthorizedError("invalid refresh token")

type AdminToken struct {
	Admin        *ent.Admin
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

func (s *Service) createAdminToken(ctx context.Context, client *ent.Client, administrator *ent.Admin) (*AdminToken, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	authClaims := jwtauth.NewRBACClaims(administrator.ID, administrator.Username, administrator.Roles, time.Now().Add(AuthTokenValidDuration))
	authClaims.ID = newUUID.String()
	token, err := s.authorizer.GenerateToken(ctx, authClaims)
	if err != nil {
		return nil, err
	}

	sessionExpires := time.Now().Add(RefreshTokenValidDuration)
	session := client.AdminSession.Create().
		SetUID(administrator.ID).
		SetSessionKey(newUUID.String()).
		SetExpires(sessionExpires.Unix())
	if ip, ok := ctx.Value(AuthContextKeyIP).(string); ok {
		session = session.SetIPAddress(ip)
	}
	if ua, ok := ctx.Value(AuthContextKeyUA).(string); ok {
		session = session.SetDeviceInfo(ua)
	}
	adminSession, err := session.Save(ctx)
	if err != nil {
		return nil, err
	}

	refreshClaims := jwtauth.NewRBACClaims(adminSession.ID, adminSession.SessionKey, nil, sessionExpires)
	refresh, err := s.authRefresher.GenerateToken(ctx, refreshClaims)
	if err != nil {
		return nil, err
	}

	var expiresAt int64
	if authClaims.ExpiresAt != nil {
		expiresAt = authClaims.ExpiresAt.Unix()
	}

	return &AdminToken{
		Admin:        administrator,
		AccessToken:  token,
		RefreshToken: refresh,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *Service) LoginWithPassword(ctx context.Context, request *dashv1.LoginWithPasswordRequest) (*dashv1.LoginWithPasswordResponse, error) {
	token, err := dao.WithTx[AdminToken](ctx, s.db.Client, func(ctx context.Context, client *ent.Client) (*AdminToken, error) {
		administrator, err := client.Admin.Query().Where(admin.UsernameEQ(NormalizeUsername(request.Username))).Only(ctx)
		if err != nil {
			return nil, dashv1.AuthError_AUTH_ERROR_INVALID_CREDENTIALS // 隐藏错误信息
		}
		if !secure.IsPasswordMatch(request.Password, administrator.Password) {
			return nil, dashv1.AuthError_AUTH_ERROR_INVALID_CREDENTIALS
		}
		return s.createAdminToken(ctx, client, administrator)
	})
	if err != nil {
		return nil, err
	}
	setAuthCookie(ctx, token.AccessToken, int(AuthTokenValidDuration.Seconds()))
	return &dashv1.LoginWithPasswordResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
	}, nil
}

// Logout makes the browser credential actually go away: it deletes the auth
// cookie and revokes the refresh session the request names.
//
// The cookie is HttpOnly, so the server deleting it is the only way it can be
// deleted — a client-side "logout" that just forgets about the token leaves a
// valid credential in the browser for the rest of its lifetime. The access
// token is a stateless JWT and cannot be revoked early; deleting the cookie
// (the only copy the browser holds) is what makes this endpoint meaningful.
//
// Idempotent, and deliberately tolerant of a missing refresh token: an empty or
// unparsable token is a normal input (a client that lost its token but still
// holds the cookie must be able to clear it), and a session row that is already
// gone counts as logged out.
func (s *Service) Logout(ctx context.Context, request *dashv1.LogoutRequest) (*dashv1.LogoutResponse, error) {
	// Clear the cookie before anything that can fail. Deleting it is the point
	// of the call, and every later step here is best-effort.
	setAuthCookie(ctx, "", -1)
	claims, err := s.authRefresher.ParseToken(ctx, request.RefreshToken)
	if err != nil {
		return &dashv1.LogoutResponse{}, nil
	}
	if err := s.db.AdminSession.UpdateOneID(claims.UID).SetIsRevoked(true).Exec(ctx); err != nil && !ent.IsNotFound(err) {
		return nil, err
	}
	return &dashv1.LogoutResponse{}, nil
}

func (s *Service) RefreshToken(ctx context.Context, request *dashv1.RefreshTokenRequest) (*dashv1.RefreshTokenResponse, error) {
	claims, err := s.authRefresher.ParseToken(ctx, request.RefreshToken)
	if err != nil {
		return nil, errInvalidRefreshToken
	}
	var expiredSessionID int64
	token, err := dao.WithTx[AdminToken](ctx, s.db.Client, func(ctx context.Context, client *ent.Client) (*AdminToken, error) {
		session, err := client.AdminSession.Get(ctx, claims.UID)
		if ent.IsNotFound(err) {
			return nil, errInvalidRefreshToken
		}
		if err != nil {
			return nil, err
		}
		if session.IsRevoked {
			return nil, dashv1.AdminSessionError_ADMIN_SESSION_ERROR_REVOKED
		}
		if session.Expires < time.Now().Unix() {
			expiredSessionID = session.ID
			return nil, dashv1.AdminSessionError_ADMIN_SESSION_ERROR_EXPIRED
		}
		if session.SessionKey != claims.Subject {
			return nil, dashv1.AdminSessionError_ADMIN_SESSION_ERROR_KEY_NOT_MATCH
		}
		administrator, err := client.Admin.Get(ctx, session.UID)
		if ent.IsNotFound(err) {
			return nil, errInvalidRefreshToken
		}
		if err != nil {
			return nil, err
		}
		// Revoke conditionally so that only one of several concurrent refreshes
		// with the same token wins; the others see zero affected rows.
		revoked, err := client.AdminSession.Update().
			Where(adminsession.ID(session.ID), adminsession.IsRevoked(false)).
			SetIsRevoked(true).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		if revoked == 0 {
			return nil, dashv1.AdminSessionError_ADMIN_SESSION_ERROR_REVOKED
		}
		return s.createAdminToken(ctx, client, administrator)
	})
	if err != nil {
		if expiredSessionID != 0 {
			if revokeErr := s.db.AdminSession.UpdateOneID(expiredSessionID).SetIsRevoked(true).Exec(ctx); revokeErr != nil {
				return nil, dashv1.AdminSessionError_ADMIN_SESSION_ERROR_EXPIRED.Join(revokeErr)
			}
		}
		return nil, err
	}
	setAuthCookie(ctx, token.AccessToken, int(AuthTokenValidDuration.Seconds()))
	return &dashv1.RefreshTokenResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
	}, nil
}
