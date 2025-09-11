package scenario

import (
	"sort"

	"cryptobot/internal/usecase/orderbook"
)

type EqualSplit struct{}

func (EqualSplit) Name() string {
	return "Сценарий #2 (Равное распределение средств по биржам)"
}

func (EqualSplit) Run(in Inputs) Result {
	res := Result{Asset: in.Right}

	type exOB struct {
		ex string
	}
	var xs []exOB
	for ex, ob := range in.OrderBooks {
		if ob != nil {
			xs = append(xs, exOB{ex: ex})
		}
	}
	if len(xs) == 0 {
		return res
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].ex < xs[j].ex })

	split := in.Amount / float64(len(xs))

	switch in.Direction {
	case Buy:
		for _, it := range xs {
			ob := in.OrderBooks[it.ex]
			if ob == nil {
				continue
			}
			qty, avg, spent := orderbook.BuyQtyFromAsks(ob.Asks, split)
			if qty <= 0 || avg <= 0 || spent <= 0 {
				continue
			}
			res.Legs = append(res.Legs, Leg{
				Exchange:   it.ex,
				Price:      avg,
				Qty:        qty,
				AmountUSDT: spent,
			})
			res.TotalQty += qty
			res.TotalUSDT += spent
		}

	case Sell:
		for _, it := range xs {
			ob := in.OrderBooks[it.ex]
			if ob == nil {
				continue
			}
			received, avg := orderbook.SellFromBids(ob.Bids, split)
			if received <= 0 || avg <= 0 {
				continue
			}
			res.Legs = append(res.Legs, Leg{
				Exchange:   it.ex,
				Price:      avg,
				Qty:        split,
				AmountUSDT: received,
			})
			res.TotalQty += split
			res.TotalUSDT += received
		}
	}

	if res.TotalQty > 0 {
		res.AveragePrice = res.TotalUSDT / res.TotalQty
	}
	if in.Direction == Sell {
		res.Asset = in.Symbol[:len(in.Symbol)-4]
	}
	return res
}
