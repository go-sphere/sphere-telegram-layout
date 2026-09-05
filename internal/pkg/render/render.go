package render

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere/storage"
)

type Render struct {
	db          *dao.Dao
	storage     storage.URLHandler
	hidePrivacy bool
}

func NewRender(db *dao.Dao, storage storage.URLHandler, hidePrivacy bool) *Render {
	return &Render{db: db, storage: storage, hidePrivacy: hidePrivacy}
}
