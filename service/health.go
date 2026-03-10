package service

import (
	"context"

	"github.com/Royaltyprogram/aiops/dto/request"
	"github.com/Royaltyprogram/aiops/dto/response"
	"github.com/Royaltyprogram/aiops/pkg/buildinfo"
)

type HealthService struct {
	Options
}

func NewHealthService(opt Options) *HealthService {
	return &HealthService{opt}
}

func (u *HealthService) Health(ctx context.Context, req *request.HealthReq) (*response.HealthResp, error) {
	return &response.HealthResp{
		Reply: req.Message,
	}, nil
}

func (u *HealthService) Liveness(ctx context.Context) (*response.ProbeResp, error) {
	_ = ctx
	meta := buildinfo.Current()
	return &response.ProbeResp{
		Status:    "ok",
		Version:   meta.Version,
		Commit:    meta.Commit,
		BuildDate: meta.BuildDate,
	}, nil
}

func (u *HealthService) Readiness(ctx context.Context) (*response.ProbeResp, error) {
	if u.AnalyticsStore != nil {
		if err := u.AnalyticsStore.Ping(ctx); err != nil {
			return nil, err
		}
	}
	meta := buildinfo.Current()
	return &response.ProbeResp{
		Status:    "ready",
		Store:     "ok",
		Version:   meta.Version,
		Commit:    meta.Commit,
		BuildDate: meta.BuildDate,
	}, nil
}
