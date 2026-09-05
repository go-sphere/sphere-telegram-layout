package api

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/render"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/server/auth/authorizer"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
	"github.com/go-sphere/sphere/storage"
)

type TokenAuthorizer = authorizer.TokenAuthorizer[int64, jwtauth.RBACClaims[int64]]

type Service struct {
	authorizer.ContextUtils[int64]

	db     *dao.Dao
	render *render.Render

	cache      cache.ByteCache
	storage    storage.CDNStorage
	authorizer TokenAuthorizer
}

func NewService(db *dao.Dao, cache cache.ByteCache, store storage.CDNStorage) *Service {
	return &Service{
		db:      db,
		cache:   cache,
		render:  render.NewRender(db, store, true),
		storage: store,
	}
}

func (s *Service) Init(authorizer TokenAuthorizer) {
	s.authorizer = authorizer
}
