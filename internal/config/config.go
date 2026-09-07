package config

import (
	"fmt"

	"github.com/go-sphere/confstore"
	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/confstore/provider/file"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/bot"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/dash"
	"github.com/go-sphere/sphere/log/zapx"
	spherefile "github.com/go-sphere/sphere/server/service/file"
	"github.com/go-sphere/sphere/utils/secure"
)

var BuildVersion = "dev"

type Config struct {
	Environments map[string]string                 `json:"environments" yaml:"environments"`
	Log          zapx.Config                       `json:"log" yaml:"log"`
	Database     client.Config                     `json:"database" yaml:"database"`
	Dash         dash.Config                       `json:"dash" yaml:"dash"`
	Local        spherefile.LocalFileServiceConfig `json:"local" yaml:"local"`
	Bot          bot.Config                        `json:"bot" yaml:"bot"`
}

func NewEmptyConfig() *Config {
	return &Config{
		Environments: map[string]string{},
		Log: zapx.Config{
			File: zapx.FileConfig{
				FileName:   "./var/log/sphere.log",
				MaxSize:    10,
				MaxBackups: 10,
				MaxAge:     10,
			},
			Console: zapx.ConsoleConfig{},
			Level:   "info",
		},
		Database: client.Config{
			Type:  "sqlite3",
			Path:  "file:./var/data.db?cache=shared&mode=rwc",
			Debug: false,
		},
		Dash: dash.Config{
			AuthJWT:    secure.RandString(32),
			RefreshJWT: secure.RandString(32),
			HTTP: dash.HTTPConfig{
				Address: "0.0.0.0:8800",
				Cors:    nil,
				Static:  "",
			},
		},
		Local: spherefile.LocalFileServiceConfig{
			RootDir:    "./var/file",
			PublicBase: "http://localhost:8800/files",
		},
		Bot: bot.Config{
			Token: "YOUR_TELEGRAM_BOT_TOKEN",
		},
	}
}

func NewConfig(path string) (*Config, error) {
	config, err := confstore.Load[Config](file.New(path), codec.JsonCodec())
	if err != nil {
		return nil, err
	}
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Dash.AuthJWT == "" || config.Dash.RefreshJWT == "" {
		return nil, fmt.Errorf("dash auth_jwt and refresh_jwt must be non-empty")
	}
	return config, nil
}
