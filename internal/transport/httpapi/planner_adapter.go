package httpapi

import (
	"context"
	"strings"

	"cryptobot/internal/usecase/planner"
)

// PlannerAdapter — тонкий адаптер: маппит httpapi.Plan* <-> planner.* и вызывает use-case.
type PlannerAdapter struct {
	Svc *planner.Service
}

// Гарантируем совместимость с ожидаемым интерфейсом httpapi.Server (Plan(ctx, PlanRequest) ...).
func (a *PlannerAdapter) Plan(ctx context.Context, req PlanRequest) (PlanResponse, error) {
	in := planner.Request{
		Base:     strings.ToUpper(strings.TrimSpace(req.Base)),
		Quote:    strings.ToUpper(strings.TrimSpace(req.Quote)),
		Amount:   req.Amount,
		Scenario: req.Scenario,
	}
	out, err := a.Svc.Plan(ctx, in)
	if err != nil {
		return PlanResponse{}, err
	}
	legs := make([]PlanLeg, 0, len(out.Legs))
	for _, l := range out.Legs {
		legs = append(legs, PlanLeg{
			Exchange: l.Exchange,
			Amount:   l.Amount,
			Price:    l.Price,
		})
	}
	return PlanResponse{
		Scenario:    out.Scenario,
		Base:        out.Base,
		Quote:       out.Quote,
		VWAP:        out.VWAP,
		TotalCost:   out.TotalCost,
		Unspent:     out.Unspent,
		Generated:   out.Generated,
		Legs:        legs,
		Diagnostics: out.Diagnostics,
		GeneratedAt: out.GeneratedAt,
	}, nil
}
