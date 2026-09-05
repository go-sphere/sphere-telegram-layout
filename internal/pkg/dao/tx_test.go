package dao

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
)

func TestTransactionPanicRollsBackAndRepanics(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *ent.Client)
	}{
		{
			name: "WithTxEx",
			run: func(ctx context.Context, db *ent.Client) {
				_ = WithTxEx(ctx, db, func(ctx context.Context, tx *ent.Client) error {
					insertAdmin(t, ctx, tx)
					panic("boom")
				})
			},
		},
		{
			name: "WithTx",
			run: func(ctx context.Context, db *ent.Client) {
				_, _ = WithTx(ctx, db, func(ctx context.Context, tx *ent.Client) (*int, error) {
					insertAdmin(t, ctx, tx)
					panic("boom")
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testClient(t)
			ctx := t.Context()
			var panicValue any
			func() {
				defer func() { panicValue = recover() }()
				tt.run(ctx, db)
			}()
			if panicValue != "boom" {
				t.Fatalf("recovered panic = %v, want boom", panicValue)
			}

			count, err := db.Admin.Query().Count(ctx)
			if err != nil {
				t.Fatalf("count admins: %v", err)
			}
			if count != 0 {
				t.Fatalf("panic path committed %d admins, want 0", count)
			}
		})
	}
}

func testClient(t *testing.T) *ent.Client {
	t.Helper()
	db, err := client.NewDataBaseClient(client.Config{
		Type: "sqlite3",
		Path: filepath.Join(t.TempDir(), "tx.db"),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test db: %v", err)
		}
	})
	return db
}

func insertAdmin(t *testing.T, ctx context.Context, db *ent.Client) {
	t.Helper()
	if err := db.Admin.Create().
		SetUsername("panic-admin").
		SetPassword("x").
		SetRoles([]string{"all"}).
		Exec(ctx); err != nil {
		t.Fatalf("insert in transaction: %v", err)
	}
}
