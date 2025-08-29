package scenario

import "cryptobot/internal/usecase/orderbook"

type EqualSplit struct{}

func (EqualSplit) Name() string {
	return "Сценарий #2 (равное распределение)"
}

func (EqualSplit) Run(in Inputs) Result {
	if len(in.OrderBooks) == 0 {
		return Result{}
	}

	parts := make([]string, 0, len(in.OrderBooks))
	for ex, ob := range in.OrderBooks {
		if ob == nil {
			continue
		}
		switch in.Direction {
		case Buy:
			if len(ob.Asks) > 0 {
				parts = append(parts, ex)
			}
		case Sell:
			if len(ob.Bids) > 0 {
				parts = append(parts, ex)
			}
		}
	}
	if len(parts) == 0 {
		return Result{}
	}

	split := in.Amount / float64(len(parts))
	res := Result{}

	for _, ex := range parts {
		ob := in.OrderBooks[ex]
		feeCfg := in.Fees[ex]
		if in.Direction == Buy {
			qty, avg, spent := orderbook.BuyQtyFromAsksWithFee(ob.Asks, split, feeCfg.FeePct)
			if qty <= 0 {
				res.Leftover += split
				continue
			}
			// мин фильтры на шаг
			if (feeCfg.MinQty > 0 && qty < feeCfg.MinQty) || (feeCfg.MinNotional > 0 && spent < feeCfg.MinNotional) {
				res.Leftover += split
				continue
			}
			res.Legs = append(res.Legs, Leg{Exchange: ex, Price: avg, Qty: qty, AmountUSDT: spent})
			res.TotalQty += qty
			res.TotalUSDT += spent
			if split > spent {
				res.Leftover += split - spent
			}
		} else {
			usdt, avg := orderbook.SellFromBidsWithFee(ob.Bids, split, feeCfg.FeePct)
			if usdt <= 0 {
				continue
			}
			if feeCfg.MinNotional > 0 && usdt < feeCfg.MinNotional {
				continue
			}
			res.Legs = append(res.Legs, Leg{Exchange: ex, Price: avg, Qty: split, AmountUSDT: usdt})
			res.TotalQty += split
			res.TotalUSDT += usdt
		}
	}

	if res.TotalQty > 0 {
		res.AveragePrice = res.TotalUSDT / res.TotalQty
	}
	return res
}
