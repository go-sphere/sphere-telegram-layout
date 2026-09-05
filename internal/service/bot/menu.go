package bot

import (
	"context"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
)

var _ botv1.MenuServiceBotServer = (*Service)(nil)

func (*Service) UpdateCount(_ context.Context, request *botv1.UpdateCountRequest) (*botv1.UpdateCountResponse, error) {
	return &botv1.UpdateCountResponse{Value: request.Value + request.Offset}, nil
}
