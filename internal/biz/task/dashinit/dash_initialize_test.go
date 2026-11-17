package dashinit

import (
	"path/filepath"
	"testing"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/dash"
	"github.com/go-sphere/sphere/utils/secure"
)

func TestInitializeSeedsConfiguredAdminWhenTableIsEmpty(t *testing.T) {
	ctx := t.Context()
	db := openDashInitDB(t)
	seed := dash.SeedUserConfig{Username: "seed-admin", Password: "ConfigSeed#9"}

	if err := NewDashInitialize(dao.NewDao(db), dash.Config{SeedUser: seed}).Start(ctx); err != nil {
		t.Fatalf("initialize empty table: %v", err)
	}
	assertSeededAdmin(t, db, seed)

	if err := NewDashInitialize(dao.NewDao(db), dash.Config{SeedUser: seed}).Start(ctx); err != nil {
		t.Fatalf("initialize with existing admin: %v", err)
	}
	if count := adminCount(t, db); count != 1 {
		t.Fatalf("admin count after second start = %d, want 1", count)
	}
}

func TestInitializeGeneratesPasswordWhenSeedPasswordIsEmpty(t *testing.T) {
	ctx := t.Context()
	db := openDashInitDB(t)
	seed := dash.SeedUserConfig{Username: " Seed-Admin "}

	if err := NewDashInitialize(dao.NewDao(db), dash.Config{SeedUser: seed}).Start(ctx); err != nil {
		t.Fatalf("initialize empty table: %v", err)
	}
	admin, err := db.Admin.Query().Only(ctx)
	if err != nil {
		t.Fatalf("load seeded admin: %v", err)
	}
	if admin.Username != "seed-admin" {
		t.Fatalf("seeded username = %q, want seed-admin", admin.Username)
	}
	if secure.IsPasswordMatch("", admin.Password) {
		t.Fatal("seeded admin must not accept an empty password")
	}
}

func TestInitializeReseedsAfterAllAdminsDeleted(t *testing.T) {
	ctx := t.Context()
	db := openDashInitDB(t)
	seed := dash.SeedUserConfig{Username: "recovery-admin", Password: "RecoverMe#1"}
	initialize := NewDashInitialize(dao.NewDao(db), dash.Config{SeedUser: seed})

	if err := initialize.Start(ctx); err != nil {
		t.Fatalf("first initialize: %v", err)
	}
	if _, err := db.Admin.Delete().Exec(ctx); err != nil {
		t.Fatalf("delete admins: %v", err)
	}
	if count := adminCount(t, db); count != 0 {
		t.Fatalf("admin count after delete = %d, want 0", count)
	}

	if err := initialize.Start(ctx); err != nil {
		t.Fatalf("re-seed after delete: %v", err)
	}
	assertSeededAdmin(t, db, seed)
}

func TestInitializeLeavesExistingAdminsAlone(t *testing.T) {
	ctx := t.Context()
	db := openDashInitDB(t)
	password, err := secure.CryptPassword("ExistingPass#1")
	if err != nil {
		t.Fatalf("crypt existing password: %v", err)
	}
	existing, err := db.Admin.Create().
		SetUsername("already-here").
		SetPassword(password).
		SetRoles([]string{"admin"}).
		Save(ctx)
	if err != nil {
		t.Fatalf("insert existing admin: %v", err)
	}

	seed := dash.SeedUserConfig{Username: "should-not-create", Password: "ShouldNot#1"}
	if err := NewDashInitialize(dao.NewDao(db), dash.Config{SeedUser: seed}).Start(ctx); err != nil {
		t.Fatalf("initialize with existing admin: %v", err)
	}
	if count := adminCount(t, db); count != 1 {
		t.Fatalf("admin count = %d, want 1", count)
	}
	got, err := db.Admin.Get(ctx, existing.ID)
	if err != nil {
		t.Fatalf("reload existing admin: %v", err)
	}
	if got.Username != "already-here" {
		t.Fatalf("username = %q, want already-here", got.Username)
	}
	if got.Password != existing.Password {
		t.Fatal("existing admin password was overwritten")
	}
}

func TestInitializeDoesNotRecordPartialSeedFailure(t *testing.T) {
	ctx := t.Context()
	db := openDashInitDB(t)
	if _, err := db.ExecContext(ctx, `
		CREATE TRIGGER reject_admin_seed
		BEFORE INSERT ON admins
		BEGIN
			SELECT RAISE(FAIL, 'admin seed rejected');
		END
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	seed := dash.SeedUserConfig{Username: "seed-admin", Password: "ConfigSeed#9"}
	initialize := NewDashInitialize(dao.NewDao(db), dash.Config{SeedUser: seed})
	if err := initialize.Start(ctx); err == nil {
		t.Fatal("expected admin seed failure")
	}
	if count := adminCount(t, db); count != 0 {
		t.Fatalf("admin count after failed seed = %d, want 0", count)
	}

	if _, err := db.ExecContext(ctx, "DROP TRIGGER reject_admin_seed"); err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}
	if err := initialize.Start(ctx); err != nil {
		t.Fatalf("retry initialize: %v", err)
	}
	assertSeededAdmin(t, db, seed)
}

func openDashInitDB(t *testing.T) *ent.Client {
	t.Helper()
	db, err := client.NewDataBaseClient(client.Config{
		Type: "sqlite3",
		Path: filepath.Join(t.TempDir(), "dash-init.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func adminCount(t *testing.T, db *ent.Client) int {
	t.Helper()
	count, err := db.Admin.Query().Count(t.Context())
	if err != nil {
		t.Fatalf("count admins: %v", err)
	}
	return count
}

func assertSeededAdmin(t *testing.T, db *ent.Client, seed dash.SeedUserConfig) {
	t.Helper()
	admin, err := db.Admin.Query().Only(t.Context())
	if err != nil {
		t.Fatalf("load seeded admin: %v", err)
	}
	if admin.Username != seed.Username {
		t.Fatalf("seeded username = %q, want %q", admin.Username, seed.Username)
	}
	if !secure.IsPasswordMatch(seed.Password, admin.Password) {
		t.Fatal("seeded password is not the configured bcrypt hash")
	}
}
