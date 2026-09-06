//go:build spheretools
// +build spheretools

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/go-sphere/sphere-telegram-layout/internal/config"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/docs"
	"github.com/go-sphere/sphere/core/boot"
)

func main() {
	conf := boot.DefaultConfigParser(config.BuildVersion, config.NewConfig)
	err := boot.Run(conf, func(c *config.Config) (*boot.Application, error) {
		return boot.NewApplication(docs.NewWebServer(c.Docs)), nil
	}, boot.WithShutdownTimeout(5*time.Second))
	if err != nil {
		fmt.Printf("Boot error: %v", err)
		os.Exit(1)
	}
	fmt.Println("Boot done")
}
