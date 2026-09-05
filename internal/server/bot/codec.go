package bot

import (
	"context"
	"fmt"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
	"github.com/go-sphere/telegram-bot/telegram"
)

var _ botv1.MenuServiceBotCodec = (*MenuServiceBotCodec)(nil)

type MenuServiceBotCodec struct{}

func (MenuServiceBotCodec) DecodeUpdateCountRequest(_ context.Context, request *telegram.Update) (*botv1.UpdateCountRequest, error) {
	return UnmarshalUpdateDataWithDefault(request, &botv1.UpdateCountRequest{})
}

func (MenuServiceBotCodec) EncodeUpdateCountResponse(_ context.Context, response *botv1.UpdateCountResponse) (*telegram.Message, error) {
	return &telegram.Message{
		Text: fmt.Sprintf("UpdateCount: %d", response.Value),
		Button: [][]telegram.Button{
			{
				NewButtonX("-1", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{Value: response.Value, Offset: -1}),
				NewButtonX("+1", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{Value: response.Value, Offset: 1}),
			},
			{
				NewButtonX("Reset", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{}),
			},
		},
	}, nil
}
