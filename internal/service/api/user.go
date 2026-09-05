package api

import (
	"context"

	apiv1 "github.com/go-sphere/sphere-telegram-layout/api/api/v1"
)

var _ apiv1.UserServiceHTTPServer = (*Service)(nil)

func (s *Service) GetCurrentUser(ctx context.Context, request *apiv1.GetCurrentUserRequest) (*apiv1.GetCurrentUserResponse, error) {
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	me, err := s.db.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &apiv1.GetCurrentUserResponse{
		User: s.render.User(me),
	}, nil
}
