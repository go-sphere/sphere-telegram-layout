package main

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/biz/task/conncleaner"
	"github.com/go-sphere/sphere-telegram-layout/internal/biz/task/dashinit"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/api"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/bot"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/dash"
	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/server/service/file"
)

func newApplication(
	dash *dash.Web,
	api *api.Web,
	botApp *bot.Bot,
	file *file.Web,
	initialize *dashinit.DashInitialize,
	cleaner *conncleaner.ConnectCleaner,
) *boot.Application {
	// Cleaner is the first wave so it Starts (noop) before HTTP, and Stops
	// last — after dash/api/file have drained — instead of closing the DB
	// concurrently with in-flight requests.
	return boot.NewStagedApplication(
		[]task.Task{cleaner},
		[]task.Task{dash, api, botApp, file, initialize},
	)
}
