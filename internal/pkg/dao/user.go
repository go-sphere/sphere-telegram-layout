package dao

import (
	"context"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/conv"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/user"
)

func (d *Dao) GetUsers(ctx context.Context, ids []int64) (map[int64]*ent.User, error) {
	users, err := d.User.Query().Where(user.IDIn(conv.UniqueSorted(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	userMap := make(map[int64]*ent.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	return userMap, nil
}
