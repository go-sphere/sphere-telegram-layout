package main

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/biz/task/conncleaner"
	"github.com/go-sphere/sphere-telegram-layout/internal/biz/task/dashinit"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/bot"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/dash"
	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/core/task"
)

func newApplication(
	dash *dash.Web,
	botApp *bot.Bot,
	initialize *dashinit.DashInitialize,
	cleaner *conncleaner.ConnectCleaner,
) *boot.Application {
	// Cleaner is the first wave so it Starts (noop) before HTTP, and Stops
	// last — after dash/bot have drained — instead of closing the DB
	// concurrently with in-flight requests.
	return boot.NewStagedApplication(
		[]task.Task{cleaner},
		[]task.Task{dash, botApp, initialize},
	)
}
