package dash

import (
	"context"
	"time"

	"github.com/go-sphere/httpx"
	dashv1 "github.com/go-sphere/sphere-telegram-layout/api/dash/v1"
	sharedv1 "github.com/go-sphere/sphere-telegram-layout/api/shared/v1"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/httpsrv"
	"github.com/go-sphere/sphere-telegram-layout/internal/service/dash"
	"github.com/go-sphere/sphere-telegram-layout/internal/service/shared"
	"github.com/go-sphere/sphere/server/auth/acl"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/server/middleware/auth"
	"github.com/go-sphere/sphere/server/middleware/ratelimiter"
	"github.com/go-sphere/sphere/server/middleware/selector"
	"github.com/go-sphere/sphere/storage"
)

type Web struct {
	config    Config
	acl       *acl.ACL
	engine    httpx.Engine
	service   *dash.Service
	sharedSvc *shared.Service
}

func NewWebServer(conf Config, storage storage.CDNStorage, service *dash.Service) *Web {
	return &Web{
		config:    conf,
		acl:       acl.NewACL(),
		engine:    httpsrv.NewGinServer("dash", conf.HTTP.Address),
		service:   service,
		sharedSvc: shared.NewService(storage, "dash"),
	}
}

func (w *Web) Identifier() string {
	return "dash"
}

func (w *Web) Start(ctx context.Context) error {
	jwtAuthorizer := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]](w.config.AuthJWT)
	jwtRefresher := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]](w.config.RefreshJWT)

	authMiddleware := auth.NewAuthMiddleware[int64, jwtauth.RBACClaims[int64]](
		jwtAuthorizer,
		auth.WithHeaderLoader(auth.AuthorizationHeader),
		auth.WithPrefixTransform(auth.AuthorizationPrefixBearer),
		auth.WithAbortOnError(true),
	)

	if err := httpsrv.UseCORS(w.engine, w.config.HTTP.Cors); err != nil {
		return err
	}

	// dashboard 静态资源
	// 1. 不设置 `embed_dash` 编译选项，使用默认的静态资源, 在配置中设置静态资源的绝对路径
	// 2. 设置 `embed_dash` 编译选项，使用内置的静态资源, 静态资源位置在 `assets/dash/dashboard` 目录下
	// 3. 由使用其他服务反代，设置API允许其跨域访问, 其中w.config.DashCors是一个配置项，用于配置允许跨域访问的域名,例如：https://dash.example.com
	w.RegisterDashStatic(w.engine.Group("/dash"))

	api := w.engine.Group("/")
	needAuthRoute := api.Group("/", authMiddleware)
	w.service.Init(jwtAuthorizer, jwtRefresher)

	initDefaultRolesACL(w.acl)

	sharedv1.RegisterStorageServiceHTTPServer(needAuthRoute, w.sharedSvc)
	sharedv1.RegisterTestServiceHTTPServer(api, w.sharedSvc)

	authRoute := api.Group("/", NewSessionMetaData())
	// 根据元数据限定中间件作用范围
	rateLimiter := ratelimiter.NewRateLimiterByClientIP(time.Second, 5, time.Hour)
	authRoute.Use(
		selector.NewSelectorMiddleware(
			selector.MatchFunc(
				httpz.MatchOperation(
					authRoute.BasePath(),
					dashv1.EndpointsAuthService[:],
					dashv1.OperationAuthServiceLoginWithPassword,
				),
			),
			rateLimiter,
		)...,
	)
	RegisterPureRute(authRoute)
	dashv1.RegisterAuthServiceHTTPServer(authRoute, w.service)

	adminRoute := needAuthRoute.Group("/", w.withPermission(dash.PermissionAdmin))
	dashv1.RegisterAdminServiceHTTPServer(adminRoute, w.service)
	dashv1.RegisterAdminSessionServiceHTTPServer(adminRoute, w.service)

	systemRoute := needAuthRoute.Group("/")
	dashv1.RegisterSystemServiceHTTPServer(systemRoute, w.service)
	dashv1.RegisterKeyValueStoreServiceHTTPServer(systemRoute, w.service)

	return w.engine.Start()
}

func (w *Web) Stop(ctx context.Context) error {
	return w.engine.Stop(ctx)
}

func (w *Web) withPermission(resource string) httpx.Middleware {
	return auth.NewPermissionMiddleware[int64](resource, w.acl)
}

func initDefaultRolesACL(acl *acl.ACL) {
	roles := []string{
		dash.PermissionAdmin,
	}
	for _, r := range roles {
		acl.Allow(dash.PermissionAll, r)
		acl.Allow(r, r)
	}
}
