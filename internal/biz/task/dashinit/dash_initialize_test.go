package dashinit

import (
	"path/filepath"
	"testing"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/keyvaluestore"
	"github.com/go-sphere/sphere/utils/secure"
)

func TestInitializeRecordsCompletionOnlyAfterAdminSeedSucceeds(t *testing.T) {
	ctx := t.Context()
	db, err := client.NewDataBaseClient(client.Config{
		Type: "sqlite3",
		Path: filepath.Join(t.TempDir(), "dash-init.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(ctx, `
		CREATE TRIGGER reject_admin_seed
		BEFORE INSERT ON admins
		BEGIN
			SELECT RAISE(FAIL, 'admin seed rejected');
		END
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	initialize := NewDashInitialize(dao.NewDao(db))
	if err := initialize.Start(ctx); err == nil {
		t.Fatal("expected admin seed failure")
	}
	assertInitState(t, db, 0, false)

	if _, err := db.ExecContext(ctx, "DROP TRIGGER reject_admin_seed"); err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}
	if err := initialize.Start(ctx); err != nil {
		t.Fatalf("retry initialize: %v", err)
	}
	assertInitState(t, db, 1, true)

	admin, err := db.Admin.Query().Only(ctx)
	if err != nil {
		t.Fatalf("load seeded admin: %v", err)
	}
	if admin.Username != defaultAdminUsername {
		t.Fatalf("seeded username = %q", admin.Username)
	}
	if !secure.IsPasswordMatch(defaultAdminPassword, admin.Password) {
		t.Fatal("seeded password is not the expected bcrypt hash")
	}
}

func assertInitState(t *testing.T, db *ent.Client, adminCount int, initialized bool) {
	t.Helper()
	ctx := t.Context()
	gotCount, err := db.Admin.Query().Count(ctx)
	if err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if gotCount != adminCount {
		t.Fatalf("admin count = %d, want %d", gotCount, adminCount)
	}
	gotInitialized, err := db.KeyValueStore.Query().
		Where(keyvaluestore.KeyEQ("did_init")).
		Exist(ctx)
	if err != nil {
		t.Fatalf("query initialization marker: %v", err)
	}
	if gotInitialized != initialized {
		t.Fatalf("did_init exists = %v, want %v", gotInitialized, initialized)
	}
}
