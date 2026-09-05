package api

import (
	"context"

	apiv1 "github.com/go-sphere/sphere-telegram-layout/api/api/v1"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/auth"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/user"
	"github.com/go-sphere/sphere/utils/secure"
)

var _ apiv1.AuthServiceHTTPServer = (*Service)(nil)

func (s *Service) RegisterWithPassword(ctx context.Context, request *apiv1.RegisterWithPasswordRequest) (*apiv1.RegisterWithPasswordResponse, error) {
	username := auth.NormalizeUsername(request.Username)
	password, err := secure.CryptPassword(request.Password)
	if err != nil {
		return nil, err
	}
	registered, err := dao.WithTx[ent.User](ctx, s.db.Client, func(ctx context.Context, client *ent.Client) (*ent.User, error) {
		created, createErr := client.User.Create().
			SetUsername(username).
			SetPassword(password).
			Save(ctx)
		if ent.IsConstraintError(createErr) {
			return nil, apiv1.AuthError_AUTH_ERROR_USERNAME_TAKEN
		}
		return created, createErr
	})
	if err != nil {
		return nil, err
	}
	token, err := s.issueUserToken(ctx, registered)
	if err != nil {
		return nil, err
	}
	return &apiv1.RegisterWithPasswordResponse{Token: token, User: s.render.User(registered)}, nil
}

func (s *Service) LoginWithPassword(ctx context.Context, request *apiv1.LoginWithPasswordRequest) (*apiv1.LoginWithPasswordResponse, error) {
	username := auth.NormalizeUsername(request.Username)
	registered, err := s.db.User.Query().Where(user.UsernameEQ(username)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, apiv1.AuthError_AUTH_ERROR_INVALID_CREDENTIALS
	}
	if err != nil {
		return nil, err
	}
	if !secure.IsPasswordMatch(request.Password, registered.Password) {
		return nil, apiv1.AuthError_AUTH_ERROR_INVALID_CREDENTIALS
	}
	token, err := s.issueUserToken(ctx, registered)
	if err != nil {
		return nil, err
	}
	return &apiv1.LoginWithPasswordResponse{Token: token, User: s.render.User(registered)}, nil
}

// issueUserToken is the extension seam for additional identity providers:
// provider-specific code resolves a local User, then reuses token issuance.
func (s *Service) issueUserToken(ctx context.Context, registered *ent.User) (string, error) {
	return s.authorizer.GenerateToken(ctx, auth.RenderClaims(registered, auth.AppTokenValidDuration))
}
