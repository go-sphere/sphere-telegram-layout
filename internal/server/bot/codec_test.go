package bot

import (
	"strings"
	"testing"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
)

func TestEncodeUpdateCountResponse(t *testing.T) {
	message, err := (MenuServiceBotCodec{}).EncodeUpdateCountResponse(t.Context(), &botv1.UpdateCountResponse{Value: 4})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if message.Text != "UpdateCount: 4" || len(message.Button) != 2 || len(message.Button[0]) != 2 {
		t.Fatalf("unexpected message: %+v", message)
	}
	if !strings.Contains(message.Button[0][0].CallbackData, "start") {
		t.Fatalf("callback data does not contain the route: %q", message.Button[0][0].CallbackData)
	}
}
