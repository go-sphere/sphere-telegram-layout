package dao

import (
	"context"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/conv"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/admin"
)

func (d *Dao) GetAdmins(ctx context.Context, ids []int64) (map[int64]*ent.Admin, error) {
	admins, err := d.Client.Admin.Query().Where(admin.IDIn(conv.UniqueSorted(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	adminMap := make(map[int64]*ent.Admin, len(admins))
	for _, a := range admins {
		adminMap[a.ID] = a
	}
	return adminMap, nil
}
