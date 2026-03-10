package cmd

import (
	"github.com/google/wire"

	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/crontab"
	"github.com/Royaltyprogram/aiops/routes"
	"github.com/Royaltyprogram/aiops/service"
)

var ProviderSet = wire.NewSet(
	wire.Struct(new(Options), "*"),
	NewApp,
	NewDefaultSlog,
)

type Options struct {
	Config         *configs.Config
	Http           *routes.HttpEngine
	Cron           *crontab.Client
	AnalyticsStore *service.AnalyticsStore
}
