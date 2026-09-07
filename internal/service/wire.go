package service

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/service/bot"
	"github.com/go-sphere/sphere-telegram-layout/internal/service/dash"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	dash.NewService,
	bot.NewService,
)
