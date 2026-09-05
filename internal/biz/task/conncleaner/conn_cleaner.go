package conncleaner

import (
	"context"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere/cache"
	"golang.org/x/sync/errgroup"
)

type ConnectCleaner struct {
	db    *dao.Dao
	cache cache.ByteCache
}

func NewConnectCleaner(db *dao.Dao, cache cache.ByteCache) *ConnectCleaner {
	return &ConnectCleaner{
		db:    db,
		cache: cache,
	}
}

func (c *ConnectCleaner) Identifier() string {
	return "connect_cleaner"
}

func (c *ConnectCleaner) Start(ctx context.Context) error {
	return nil
}

func (c *ConnectCleaner) Stop(ctx context.Context) error {
	group, _ := errgroup.WithContext(ctx)
	group.Go(c.db.Close)
	group.Go(c.cache.Close)
	return group.Wait()
}
