package bot

import (
	"context"
	"strings"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
	service "github.com/go-sphere/sphere-telegram-layout/internal/service/bot"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/telegram-bot/telegram"
)

type Config = telegram.Config

// placeholderToken is the example value shipped in config.json. It can never
// be a valid bot token, so it marks the bot as disabled until a real token
// is configured.
const placeholderToken = "YOUR_TELEGRAM_BOT_TOKEN"

type Bot struct {
	bot      *telegram.Bot
	service  *service.Service
	disabled bool
}

func NewApp(conf Config, botService *service.Service) (*Bot, error) {
	if strings.TrimSpace(conf.Token) == "" || conf.Token == placeholderToken {
		return &Bot{service: botService, disabled: true}, nil
	}
	app, err := telegram.NewApp(conf)
	if err != nil {
		return nil, err
	}
	return &Bot{bot: app, service: botService}, nil
}

func (b *Bot) Identifier() string {
	return "bot"
}

func (b *Bot) Start(ctx context.Context) error {
	if b.disabled {
		log.Warn("bot disabled: configure a real bot.token in config.json to enable it")
		return nil
	}
	b.bot.BindRoute(
		botv1.RegisterMenuServiceBotServer(b.service, &MenuServiceBotCodec{}, b.bot.SendMessage),
		botv1.GetExtraBotDataByMenuServiceOperation,
		botv1.GetAllBotMenuServiceOperations(),
	)
	return b.bot.Start(ctx)
}

func (b *Bot) Stop(ctx context.Context) error {
	if b.disabled {
		return nil
	}
	return b.bot.Close(ctx)
}
