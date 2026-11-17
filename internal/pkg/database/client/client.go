package client

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	if config.Type == dialect.SQLite {
		if err := ensureSQLiteDir(config.Path); err != nil {
			return nil, err
		}
	}
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
		return nil, errors.Join(err, client.Close())
	}
	if config.Debug {
		client = client.Debug()
	}
	return client, nil
}

func ensureSQLiteDir(dsn string) error {
	pathPart, queryPart, _ := strings.Cut(dsn, "?")
	if query, err := url.ParseQuery(queryPart); err == nil && query.Get("mode") == "memory" {
		return nil
	}
	pathPart = strings.TrimPrefix(pathPart, "file:")
	if strings.HasPrefix(pathPart, "//localhost/") {
		pathPart = strings.TrimPrefix(pathPart, "//localhost")
	}
	if pathPart == "" || pathPart == ":memory:" {
		return nil
	}
	cleanPath := filepath.Clean(pathPart)
	dir := filepath.Dir(cleanPath)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
