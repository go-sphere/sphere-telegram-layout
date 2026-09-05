package server

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/server/api"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/bot"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/dash"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/file"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	api.NewWebServer,
	dash.NewWebServer,
	file.NewWebServer,
	bot.NewApp,
)
