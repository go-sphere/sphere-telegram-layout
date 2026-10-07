package dashinit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/keyvaluestore"
	"github.com/go-sphere/sphere-telegram-layout/internal/server/dash"
	servicedash "github.com/go-sphere/sphere-telegram-layout/internal/service/dash"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/utils/secure"
)

type DashInitialize struct {
	db   *dao.Dao
	seed dash.SeedUserConfig
}

func NewDashInitialize(db *dao.Dao, conf dash.Config) *DashInitialize {
	return &DashInitialize{db: db, seed: conf.SeedUser}
}

// initAdminIfNeed creates the seed admin with the "all" role while the admin
// table is empty. The username is normalized like login does; an empty seed
// password is replaced by a random one that is logged once.
func initAdminIfNeed(ctx context.Context, client *ent.Client, seed dash.SeedUserConfig) error {
	count, err := client.Admin.Query().Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	username := servicedash.NormalizeUsername(seed.Username)
	if username == "" {
		return fmt.Errorf("dash seed_user username must be set when no admin exists")
	}
	plain := seed.Password
	generated := plain == ""
	if generated {
		plain = secure.RandString(16)
	}
	password, err := secure.CryptPassword(plain)
	if err != nil {
		return err
	}
	if err := client.Admin.Create().
		SetUsername(username).
		SetPassword(password).
		SetRoles([]string{"all"}).
		Exec(ctx); err != nil {
		return err
	}
	if generated {
		// Logged once: the plain password is not stored anywhere else.
		log.Warn("seeded dashboard admin with a generated password; change it after the first login",
			log.String("username", username),
			log.String("password", plain),
		)
		return nil
	}
	log.Warn("seeded dashboard admin from config; change this password before exposing the service",
		log.String("username", username),
	)
	return nil
}

func (i *DashInitialize) Identifier() string {
	return "initialize"
}

func (i *DashInitialize) Start(ctx context.Context) error {
	key := "did_init"
	return dao.WithTxEx(ctx, i.db.Client, func(ctx context.Context, client *ent.Client) error {
		exist, err := client.KeyValueStore.Query().Where(keyvaluestore.KeyEQ(key)).Exist(ctx)
		if err != nil {
			return err
		}
		if exist {
			return nil
		}
		if err := initAdminIfNeed(ctx, client, i.seed); err != nil {
			return err
		}
		_, err = client.KeyValueStore.Create().
			SetKey(key).
			SetValue([]byte(strconv.Itoa(int(time.Now().Unix())))).
			Save(ctx)
		return err
	})
}

func (i *DashInitialize) Stop(ctx context.Context) error {
	return nil
}
