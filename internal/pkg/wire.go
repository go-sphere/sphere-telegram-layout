package pkg

import (
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/client"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	dao.NewDao,
	client.NewDataBaseClient,
)
