package client

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoMigrateDoesNotDropColumnsByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration.db")
	config := Config{Type: "sqlite3", Path: path}
	db, err := NewDataBaseClient(config)
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	if _, err := db.ExecContext(t.Context(), "ALTER TABLE admins ADD COLUMN legacy_value TEXT"); err != nil {
		_ = db.Close()
		t.Fatalf("add legacy column: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	db, err = NewDataBaseClient(config)
	if err != nil {
		t.Fatalf("migrate with safe defaults: %v", err)
	}
	if !hasColumn(t, db, "admins", "legacy_value") {
		_ = db.Close()
		t.Fatal("safe auto-migration dropped a column")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	config.AutoMigrateDrop = true
	db, err = NewDataBaseClient(config)
	if err != nil {
		t.Fatalf("migrate with destructive option: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	if hasColumn(t, db, "admins", "legacy_value") {
		t.Fatal("destructive auto-migration did not drop the legacy column")
	}
}

func TestNewDataBaseClientCreatesSQLiteDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "db")
	dsn := "file:" + filepath.Join(dir, "app.db") + "?cache=shared&mode=rwc"
	db, err := NewDataBaseClient(Config{Type: "sqlite3", Path: dsn})
	if err != nil {
		t.Fatalf("NewDataBaseClient() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("expected directory %s to be created, err=%v", dir, err)
	}
}

func TestNewDataBaseClientClosesOnSchemaError(t *testing.T) {
	db, err := NewDataBaseClient(Config{Type: "sqlite3", Path: t.TempDir()})
	if err == nil {
		_ = db.Close()
		t.Fatal("expected Schema.Create error when sqlite path is a directory")
	}
}

func hasColumn(t *testing.T, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, table, column string) bool {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), "PRAGMA table_info("+table+")")
	if err != nil {
		t.Fatalf("inspect table: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close table info rows: %v", err)
		}
	}()
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info: %v", err)
	}
	return false
}
