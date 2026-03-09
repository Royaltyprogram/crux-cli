package service

import (
	"context"

	"github.com/liushuangls/go-server-template/dto/request"
	"github.com/liushuangls/go-server-template/dto/response"
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
	return &response.ProbeResp{
		Status: "ok",
	}, nil
}

func (u *HealthService) Readiness(ctx context.Context) (*response.ProbeResp, error) {
	if u.AnalyticsStore != nil {
		if err := u.AnalyticsStore.Ping(ctx); err != nil {
			return nil, err
		}
	}
	return &response.ProbeResp{
		Status: "ready",
		Store:  "ok",
	}, nil
}
