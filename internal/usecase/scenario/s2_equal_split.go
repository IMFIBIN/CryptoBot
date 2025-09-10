package scenario

import (
	"sort"

	"cryptobot/internal/usecase/orderbook"
)

type EqualSplit struct{}

func (EqualSplit) Name() string {
	return "Сценарий #2 (равное распределение)"
}

func (EqualSplit) Run(in Inputs) Result {
	res := Result{Asset: in.Right}

	parts := make([]string, 0, len(in.OrderBooks))
	for ex := range in.OrderBooks {
		parts = append(parts, ex)
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return res
	}

	split := in.Amount / float64(len(parts))
	for _, ex := range parts {
		ob := in.OrderBooks[ex]
		f, ok := in.Fees[ex]
		if !ok || ob == nil {
			continue
		}
		if in.Direction == Buy {
			qty, avg, spentNet, fee := orderbook.BuyQtyFromAsksWithFee(ob.Asks, split, f)
			if qty <= 0 {
				res.Leftover += split
				continue
			}
			res.Legs = append(res.Legs, Leg{Exchange: ex, Price: avg, Qty: qty, AmountUSDT: spentNet, FeeUSDT: fee})
			res.TotalQty += qty
			res.TotalUSDT += spentNet
			if split > spentNet {
				res.Leftover += split - spentNet
			}
		} else {
			receivedNet, avg, fee := orderbook.SellFromBidsWithFee(ob.Bids, split, f)
			if receivedNet <= 0 {
				continue
			}
			res.Legs = append(res.Legs, Leg{Exchange: ex, Price: avg, Qty: split, AmountUSDT: receivedNet, FeeUSDT: fee})
			res.TotalQty += split
			res.TotalUSDT += receivedNet
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
