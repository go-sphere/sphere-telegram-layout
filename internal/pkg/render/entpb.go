package render

import (
	"github.com/go-sphere/sphere-telegram-layout/api/entpb"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/render/entmap"
)

func (r *Render) AdminLite(value *ent.Admin) *entpb.Admin {
	return &entpb.Admin{
		Id:       value.ID,
		Nickname: value.Nickname,
		Avatar:   r.storage.GenerateURL(value.Avatar),
	}
}

func (r *Render) Admin(value *ent.Admin) *entpb.Admin {
	val, _ := entmap.ToProtoAdmin(value)
	if val == nil {
		return nil
	}
	val.Password = ""
	val.Avatar = r.storage.GenerateURL(value.Avatar)
	return val
}

func (r *Render) AdminSession(value *ent.AdminSession) *entpb.AdminSession {
	val, _ := entmap.ToProtoAdminSession(value)
	return val
}

func (r *Render) KeyValueStore(value *ent.KeyValueStore) *entpb.KeyValueStore {
	val, _ := entmap.ToProtoKeyValueStore(value)
	return val
}

func (r *Render) KeyValueStoreList(values []*ent.KeyValueStore) []*entpb.KeyValueStore {
	vals := make([]*entpb.KeyValueStore, 0, len(values))
	for _, v := range values {
		vals = append(vals, r.KeyValueStore(v))
	}
	return vals
}
