package app

import (
	"context"
	"fmt"
	"os"
	"time"

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
	err := boot.Run(conf, app,
		boot.WithLoggerBackend(backend),
		boot.AddAfterStop(func(context.Context) error { return backend.Close() }),
		boot.WithShutdownTimeout(5*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Boot error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Boot done")
}
