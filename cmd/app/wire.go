//go:build wireinject

package main

import (
	"github.com/go-sphere/sphere-telegram-layout/internal"
	"github.com/go-sphere/sphere-telegram-layout/internal/config"
	"github.com/go-sphere/sphere/core/boot"
	"github.com/google/wire"
)

func NewApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(internal.ProviderSet, wire.NewSet(newApplication))
	return &boot.Application{}, nil
}
