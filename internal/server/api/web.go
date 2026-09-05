package api

import (
	"context"

	"github.com/go-sphere/httpx"
	apiv1 "github.com/go-sphere/sphere-telegram-layout/api/api/v1"
	sharedv1 "github.com/go-sphere/sphere-telegram-layout/api/shared/v1"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/httpsrv"
	"github.com/go-sphere/sphere-telegram-layout/internal/service/api"
	"github.com/go-sphere/sphere-telegram-layout/internal/service/shared"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
	"github.com/go-sphere/sphere/server/middleware/auth"
	"github.com/go-sphere/sphere/storage"
)

type Web struct {
	config    Config
	engine    httpx.Engine
	service   *api.Service
	sharedSvc *shared.Service
}

func NewWebServer(conf Config, storage storage.CDNStorage, service *api.Service) *Web {
	return &Web{
		config:    conf,
		engine:    httpsrv.NewGinServer("api", conf.HTTP.Address),
		service:   service,
		sharedSvc: shared.NewService(storage, "user"),
	}
}

func (w *Web) Identifier() string {
	return "api"
}

func (w *Web) Start(ctx context.Context) error {
	jwtAuthorizer := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]](w.config.JWT)

	authMiddleware := auth.NewAuthMiddleware[int64, jwtauth.RBACClaims[int64]](
		jwtAuthorizer,
		auth.WithHeaderLoader(auth.AuthorizationHeader),
		auth.WithPrefixTransform(auth.AuthorizationPrefixBearer),
		auth.WithAbortOnError(true),
	)

	if err := httpsrv.UseCORS(w.engine, w.config.HTTP.Cors); err != nil {
		return err
	}

	w.service.Init(jwtAuthorizer)

	publicRoute := w.engine.Group("/")
	protectedRoute := w.engine.Group("/", authMiddleware)

	apiv1.RegisterAuthServiceHTTPServer(publicRoute, w.service)
	apiv1.RegisterSystemServiceHTTPServer(publicRoute, w.service)
	sharedv1.RegisterStorageServiceHTTPServer(protectedRoute, w.sharedSvc)
	apiv1.RegisterUserServiceHTTPServer(protectedRoute, w.service)

	return w.engine.Start()
}

func (w *Web) Stop(ctx context.Context) error {
	return w.engine.Stop(ctx)
}
