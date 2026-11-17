package dao

import (
	"context"
	"errors"
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

func TestWithTxPreservesOriginalError(t *testing.T) {
	origErr := errors.New("business failure")
	rollbackErr := errors.New("rollback hook failed")

	t.Run("joins rollback error with original error", func(t *testing.T) {
		db := testClient(t)
		_, err := WithTx(t.Context(), db, func(ctx context.Context, tx *ent.Client) (*int, error) {
			insertAdmin(t, ctx, tx)
			return nil, origErr
		}, WithTxRollbackHook(func(next ent.Rollbacker) ent.Rollbacker {
			return ent.RollbackFunc(func(ctx context.Context, tx *ent.Tx) error {
				_ = next.Rollback(ctx, tx)
				return rollbackErr
			})
		}))
		if !errors.Is(err, origErr) {
			t.Fatalf("WithTx() error = %v, want errors.Is(_, origErr)", err)
		}
		if !errors.Is(err, rollbackErr) {
			t.Fatalf("WithTx() error = %v, want errors.Is(_, rollbackErr)", err)
		}
	})

	t.Run("ignores ErrTxDone when context cancels before rollback", func(t *testing.T) {
		db := testClient(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		_, err := WithTx(ctx, db, func(ctx context.Context, tx *ent.Client) (*int, error) {
			cancel()
			return nil, origErr
		})
		if !errors.Is(err, origErr) {
			t.Fatalf("WithTx() error = %v, want origErr", err)
		}
	})
}

func TestWithTxExCommitsAndRollsBack(t *testing.T) {
	db := testClient(t)
	ctx := t.Context()

	if err := WithTxEx(ctx, db, func(ctx context.Context, tx *ent.Client) error {
		insertAdmin(t, ctx, tx)
		return nil
	}); err != nil {
		t.Fatalf("WithTxEx(commit) error = %v", err)
	}
	count, err := db.Admin.Query().Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("admin count after commit = %d (err=%v), want 1", count, err)
	}

	wantErr := errors.New("abort second insert")
	err = WithTxEx(ctx, db, func(ctx context.Context, tx *ent.Client) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithTxEx(rollback) error = %v, want %v", err, wantErr)
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
