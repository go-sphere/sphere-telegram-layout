package app

import (
	"fmt"
	"os"

	"github.com/go-sphere/sphere-telegram-layout/internal/config"
	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/zapx"
)

func Execute(app func(*config.Config) (*boot.Application, error)) {
	conf := boot.DefaultConfigParser(config.BuildVersion, config.NewConfig)
	backend := zapx.NewBackend(conf.Log, log.WithAttrs(map[string]any{
		"version": config.BuildVersion,
	}))
	err := boot.Run(conf, app, boot.WithLoggerBackend(backend))
	if err != nil {
		fmt.Printf("Boot error: %v", err)
		os.Exit(1)
	}
	fmt.Println("Boot done")
}
