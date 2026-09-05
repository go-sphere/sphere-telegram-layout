package internal

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/biz"
	"github.com/go-sphere/sphere-telegram-layout/internal/config"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg"
	"github.com/go-sphere/sphere-telegram-layout/internal/server"
	"github.com/go-sphere/sphere-telegram-layout/internal/service"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/server/service/file"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/fileserver"
	"github.com/google/wire"
)

var cacheSet = wire.NewSet(
	memory.NewByteCache,
	wire.Bind(new(cache.ByteCache), new(*memory.ByteCache)),
)

var storageSet = wire.NewSet(
	file.NewLocalFileService, // Wrapper for local file storage to S3 adapter
	wire.Bind(new(storage.CDNStorage), new(*fileserver.FileServer)), // Bind the S3Adapter to the CDNStorage interface
)

var ProviderSet = wire.NewSet(
	// Sphere library components
	wire.NewSet(
		storageSet,
		cacheSet,
	),
	// Internal application components
	wire.NewSet(
		server.ProviderSet,
		service.ProviderSet,
		pkg.ProviderSet,
		biz.ProviderSet,
		config.ProviderSet,
	),
)
