package scenario

import (
	"sort"

	"cryptobot/internal/usecase/orderbook"
)

type BestSingle struct{}

func (BestSingle) Name() string {
	return "Сценарий #1 (Самая выгодная биржа)"
}

func (BestSingle) Run(in Inputs) Result {
	res := Result{Asset: in.Right}

	type cand struct {
		ex    string
		qty   float64
		avg   float64
		net   float64
		obQty float64
	}

	var cs []cand

	// Собираем кандидатов по всем биржам
	for ex, ob := range in.OrderBooks {
		if ob == nil {
			continue
		}
		switch in.Direction {
		case Buy:
			qty, avg, spent := orderbook.BuyQtyFromAsks(ob.Asks, in.Amount)
			if qty <= 0 || avg <= 0 || spent <= 0 {
				continue
			}
			cs = append(cs, cand{ex: ex, qty: qty, avg: avg, net: spent, obQty: qty})

		case Sell:
			received, avg := orderbook.SellFromBids(ob.Bids, in.Amount)
			if received <= 0 || avg <= 0 {
				continue
			}
			cs = append(cs, cand{ex: ex, qty: in.Amount, avg: avg, net: received, obQty: in.Amount})
		}
	}

	if len(cs) == 0 {
		return res
	}

	bestSlice := append([]cand(nil), cs...)
	if in.Direction == Buy {
		sort.Slice(bestSlice, func(i, j int) bool {
			if bestSlice[i].avg == bestSlice[j].avg {
				return bestSlice[i].obQty > bestSlice[j].obQty
			}
			return bestSlice[i].avg < bestSlice[j].avg
		})
	} else {
		sort.Slice(bestSlice, func(i, j int) bool {
			if bestSlice[i].avg == bestSlice[j].avg {
				return bestSlice[i].obQty > bestSlice[j].obQty
			}
			return bestSlice[i].avg > bestSlice[j].avg
		})
	}
	best := bestSlice[0]

	if in.Direction == Buy {
		sort.Slice(cs, func(i, j int) bool {
			if cs[i].avg == cs[j].avg {
				return cs[i].obQty > cs[j].obQty
			}
			return cs[i].avg < cs[j].avg
		})
	} else {
		sort.Slice(cs, func(i, j int) bool {
			if cs[i].avg == cs[j].avg {
				return cs[i].obQty > cs[j].obQty
			}
			return cs[i].avg > cs[j].avg
		})
	}
	for _, c := range cs {
		res.Legs = append(res.Legs, Leg{
			Exchange:   c.ex,
			Price:      c.avg,
			Qty:        c.qty,
			AmountUSDT: c.net,
		})
	}

	if in.Direction == Buy {
		res.TotalQty = best.qty
		res.TotalUSDT = best.net
		res.AveragePrice = best.avg
		res.Leftover = in.Amount - best.net
		res.Asset = in.Right
	} else {
		res.TotalQty = in.Amount
		res.TotalUSDT = best.net
		res.AveragePrice = best.avg
		// Для SELL — левая часть символа без "USDT"
		if len(in.Symbol) > 4 {
			res.Asset = in.Symbol[:len(in.Symbol)-4]
		} else {
			res.Asset = in.Symbol
		}
	}

	return res
}
