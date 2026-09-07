package bot

import (
	"context"
	"testing"

	botservice "github.com/go-sphere/sphere-telegram-layout/internal/service/bot"
)

func TestNewAppSkipsBotWithoutToken(t *testing.T) {
	for _, token := range []string{"", "YOUR_TELEGRAM_BOT_TOKEN", "  "} {
		app, err := NewApp(Config{Token: token}, botservice.NewService())
		if err != nil {
			t.Fatalf("NewApp(%q) error = %v", token, err)
		}
		if !app.disabled {
			t.Fatalf("NewApp(%q) expected disabled bot", token)
		}
		if err := app.Start(context.Background()); err != nil {
			t.Fatalf("disabled bot Start() error = %v", err)
		}
		if err := app.Stop(context.Background()); err != nil {
			t.Fatalf("disabled bot Stop() error = %v", err)
		}
	}
}
