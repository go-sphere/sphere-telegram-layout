package dashinit

import (
	"context"
	"strconv"
	"time"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/keyvaluestore"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/utils/secure"
)

const (
	defaultAdminUsername = "admin"
	defaultAdminPassword = "aA1234567"
)

type DashInitialize struct {
	db *dao.Dao
}

func NewDashInitialize(db *dao.Dao) *DashInitialize {
	return &DashInitialize{db: db}
}

func initAdminIfNeed(ctx context.Context, client *ent.Client) error {
	count, err := client.Admin.Query().Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	password, err := secure.CryptPassword(defaultAdminPassword)
	if err != nil {
		return err
	}
	if err := client.Admin.Create().
		SetUsername(defaultAdminUsername).
		SetPassword(password).
		SetRoles([]string{"all"}).
		Exec(ctx); err != nil {
		return err
	}
	log.Warn("seeded default dashboard admin; change this password before exposing the service",
		log.String("username", defaultAdminUsername),
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
		if err := initAdminIfNeed(ctx, client); err != nil {
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
