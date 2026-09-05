package client

import (
	"context"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/migrate"
	"github.com/go-sphere/sphere/infra/sqlite"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	sqlite.Register(dialect.SQLite)
}

type Config struct {
	Type  string `json:"type" yaml:"type"`
	Path  string `json:"path" yaml:"path"`
	Debug bool   `json:"debug" yaml:"debug"`
	// AutoMigrateDrop enables Ent WithDropColumn/WithDropIndex on startup.
	// Leave false in production: adding or removing fields would otherwise
	// drop columns and indexes, which can discard data.
	AutoMigrateDrop bool `json:"auto_migrate_drop" yaml:"auto_migrate_drop"`
}

func NewDataBaseClient(config Config) (*ent.Client, error) {
	client, err := ent.Open(config.Type, config.Path)
	if err != nil {
		return nil, err
	}
	var migrateOpts []schema.MigrateOption
	if config.AutoMigrateDrop {
		migrateOpts = append(migrateOpts,
			migrate.WithDropIndex(true),
			migrate.WithDropColumn(true),
		)
	}
	err = client.Schema.Create(context.Background(), migrateOpts...)
	if err != nil {
		return nil, err
	}
	if config.Debug {
		client = client.Debug()
	}
	return client, nil
}
