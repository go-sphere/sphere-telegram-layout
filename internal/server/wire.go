package server

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/server/bot"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/dash"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	dash.NewWebServer,
	bot.NewApp,
)
