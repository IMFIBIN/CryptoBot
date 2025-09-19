package httpapi

import (
	"context"
	"fmt"
	"strings"

	"cryptobot/internal/usecase/planner"
)

// PlannerAdapter — тонкий адаптер: маппит httpapi.Plan* <-> planner.* и вызывает use-case.
type PlannerAdapter struct {
	Svc *planner.Service
}

// Гарантируем совместимость с ожидаемым интерфейсом httpapi.Server (Plan(ctx, PlanRequest) ...).
func (a *PlannerAdapter) Plan(ctx context.Context, req PlanRequest) (PlanResponse, error) {
	if a == nil || a.Svc == nil {
		return PlanResponse{}, fmt.Errorf("service is not initialized")
	}

	// 1) Нормализуем пару из запроса и дальше используем ИМЕННО ЕЁ
	base := strings.ToUpper(strings.TrimSpace(req.Base))
	quote := strings.ToUpper(strings.TrimSpace(req.Quote))
	if base == "" || quote == "" || base == quote {
		return PlanResponse{}, fmt.Errorf("bad pair: %q/%q", base, quote)
	}

	in := planner.Request{
		Base:     base,
		Quote:    quote,
		Amount:   req.Amount,
		Scenario: strings.TrimSpace(req.Scenario),
	}

	out, err := a.Svc.Plan(ctx, in)
	if err != nil {
		return PlanResponse{}, err
	}

	// 2) Маппинг ножек
	legs := make([]PlanLeg, 0, len(out.Legs))
	for _, l := range out.Legs {
		legs = append(legs, PlanLeg{
			Exchange: l.Exchange,
			Amount:   l.Amount,
			Price:    l.Price,
		})
	}

	// 3) Возвращаем РОВНО пару из запроса
	return PlanResponse{
		Scenario:    out.Scenario,
		Base:        base,  // <- фактическая BASE пользователя
		Quote:       quote, // <- фактическая QUOTE пользователя
		VWAP:        out.VWAP,
		TotalCost:   out.TotalCost,
		Unspent:     out.Unspent,
		Generated:   out.Generated,
		Legs:        legs,
		Diagnostics: out.Diagnostics,
		GeneratedAt: out.GeneratedAt,
	}, nil
}
