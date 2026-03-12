package service

import (
	"github.com/google/wire"

	"github.com/Royaltyprogram/aiops/configs"
)

var ProviderSet = wire.NewSet(
	wire.Struct(new(Options), "Config", "AnalyticsStore"),
	NewAnalyticsStore,
	NewAnalyticsService,
	NewHealthService,
)

type Options struct {
	Config            *configs.Config
	AnalyticsStore    *AnalyticsStore
	ReportMinSessions int
}
