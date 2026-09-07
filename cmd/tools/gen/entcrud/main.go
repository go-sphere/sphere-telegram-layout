package main

import (
	"log"

	entgen "github.com/go-sphere/entc-extensions/entcrud"
	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/sphere-telegram-layout/api/entpb"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/admin"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/adminsession"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/keyvaluestore"
)

func main() {
	config := conf.NewFilesConf(
		"./internal/pkg/render/entbind",
		"entbind",
		conf.NewEntity(
			ent.Admin{},
			entpb.Admin{},
			[]any{ent.AdminCreate{}, ent.AdminUpdateOne{}},
			conf.WithIgnoreFields(admin.FieldCreatedAt, admin.FieldUpdatedAt),
		),
		conf.NewEntity(
			ent.AdminSession{},
			entpb.AdminSession{},
			[]any{ent.AdminSessionCreate{}, ent.AdminSessionUpdateOne{}},
			conf.WithIgnoreFields(adminsession.FieldCreatedAt, adminsession.FieldUpdatedAt),
		),
		conf.NewEntity(
			ent.KeyValueStore{},
			entpb.KeyValueStore{},
			[]any{ent.KeyValueStoreCreate{}, ent.KeyValueStoreUpdateOne{}, ent.KeyValueStoreUpsertOne{}},
			conf.WithIgnoreFields(keyvaluestore.FieldCreatedAt, keyvaluestore.FieldUpdatedAt),
		),
	)
	if err := entgen.BindFiles(config); err != nil {
		log.Fatal(err)
	}
}
