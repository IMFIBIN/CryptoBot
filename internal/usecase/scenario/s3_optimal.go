package scenario

import (
	"sort"
	"strconv"
)

type Optimal struct{}

func (Optimal) Name() string {
	return "Сценарий #3 (оптимальное распределение)"
}

// Простой жадный алгоритм: идём по лучшим уровням разных бирж, пока не исчерпаем бюджет/объём.
func (Optimal) Run(in Inputs) Result {
	res := Result{Asset: in.Right}

	type quote struct {
		ex string
		p  float64 // цена
		q  float64 // объём по этой цене
	}

	// Соберём ленты
	var quotes []quote
	for ex, ob := range in.OrderBooks {
		if ob == nil {
			continue
		}
		levels := ob.Asks
		if in.Direction == Sell {
			levels = ob.Bids
		}
		for _, lv := range levels {
			p, ok1 := parseFloat(lv.Price)
			q, ok2 := parseFloat(lv.Quantity)
			if !ok1 || !ok2 || p <= 0 || q <= 0 {
				continue
			}
			quotes = append(quotes, quote{ex: ex, p: p, q: q})
		}
	}
	if len(quotes) == 0 {
		return res
	}

	if in.Direction == Buy {
		sort.Slice(quotes, func(i, j int) bool { return quotes[i].p < quotes[j].p })
		budget := in.Amount
		for _, qu := range quotes {
			if budget <= 0 {
				break
			}
			f := in.Fees[qu.ex]
			// максимум gross на этот шаг:
			grossCap := f.InvertBuy(budget)
			// можно взять q по цене p, но не больше grossCap
			maxQtyByGross := grossCap / qu.p
			take := min(qu.q, maxQtyByGross)
			if take <= 0 {
				continue
			}
			gross := take * qu.p
			net, fee := f.ApplyBuy(gross)

			res.Legs = append(res.Legs, Leg{Exchange: qu.ex, Price: net / take, Qty: take, AmountUSDT: net, FeeUSDT: fee})
			res.TotalQty += take
			res.TotalUSDT += net
			budget -= net
		}
		res.AveragePrice = safeDiv(res.TotalUSDT, res.TotalQty)
		res.Asset = in.Right
		res.Leftover = max(0, budget)

	} else { // SELL
		sort.Slice(quotes, func(i, j int) bool { return quotes[i].p > quotes[j].p })
		remainQty := in.Amount
		for _, qu := range quotes {
			if remainQty <= 0 {
				break
			}
			take := min(qu.q, remainQty)
			if take <= 0 {
				continue
			}
			f := in.Fees[qu.ex]
			gross := take * qu.p
			net, fee := f.ApplySell(gross)

			res.Legs = append(res.Legs, Leg{Exchange: qu.ex, Price: net / take, Qty: take, AmountUSDT: net, FeeUSDT: fee})
			res.TotalQty += take
			res.TotalUSDT += net
			remainQty -= take
		}
		res.AveragePrice = safeDiv(res.TotalUSDT, res.TotalQty)
		res.Asset = in.Symbol[:len(in.Symbol)-4]
	}

	return res
}

func parseFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
