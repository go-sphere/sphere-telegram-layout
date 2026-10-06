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

func initAdminIfNeed(ctx context.Context, client *ent.Client, seed dash.SeedUserConfig) error {
	count, err := client.Admin.Query().Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if seed.Username == "" || seed.Password == "" {
		return fmt.Errorf("dash seed_user username and password must be set when no admin exists")
	}
	password, err := secure.CryptPassword(seed.Password)
	if err != nil {
		return err
	}
	if err := client.Admin.Create().
		SetUsername(seed.Username).
		SetPassword(password).
		SetRoles([]string{"all"}).
		Exec(ctx); err != nil {
		return err
	}
	log.Warn("seeded dashboard admin from config; change this password before exposing the service",
		log.String("username", seed.Username),
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
