package biz

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/biz/task/conncleaner"
	"github.com/go-sphere/sphere-telegram-layout/internal/biz/task/dashinit"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	dashinit.NewDashInitialize,
	conncleaner.NewConnectCleaner,
)
