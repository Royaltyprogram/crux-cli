//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/google/wire"

	cmd2 "github.com/Royaltyprogram/aiops/app"
	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/crontab"
	"github.com/Royaltyprogram/aiops/routes"
	"github.com/Royaltyprogram/aiops/service"
)

func app(ctx context.Context) (*cmd2.App, func(), error) {
	panic(wire.Build(
		configs.InitConfig,
		routes.ProviderSet,
		service.ProviderSet,
		crontab.ProviderSet,
		cmd2.ProviderSet,
	))
}
