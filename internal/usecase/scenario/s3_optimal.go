package scenario

import "cryptobot/internal/usecase/orderbook"

type Optimal struct{}

func (Optimal) Name() string {
	return "Сценарий #3 (оптимальное распределение)"
}

func (Optimal) Run(in Inputs) Result {
	res := Result{}
	switch in.Direction {
	case Buy:
		levels := orderbook.CombinedAsks(in.OrderBooks)
		remain := in.Amount
		for _, lv := range levels {
			if remain <= 0 {
				break
			}
			feeCfg := in.Fees[lv.Exchange]
			unit := lv.Price * (1 + feeCfg.FeePct)
			if unit <= 0 {
				continue
			}
			maxSpend := unit * lv.Qty
			if remain >= maxSpend {
				// мин фильтры на лег
				if (feeCfg.MinQty > 0 && lv.Qty < feeCfg.MinQty) || (feeCfg.MinNotional > 0 && maxSpend < feeCfg.MinNotional) {
					continue
				}
				res.Legs = append(res.Legs, Leg{Exchange: lv.Exchange, Price: unit, Qty: lv.Qty, AmountUSDT: maxSpend})
				res.TotalQty += lv.Qty
				res.TotalUSDT += maxSpend
				remain -= maxSpend
			} else {
				q := remain / unit
				// min filters на частичный
				notional := q * unit
				if (feeCfg.MinQty > 0 && q < feeCfg.MinQty) || (feeCfg.MinNotional > 0 && notional < feeCfg.MinNotional) {
					break
				}
				res.Legs = append(res.Legs, Leg{Exchange: lv.Exchange, Price: unit, Qty: q, AmountUSDT: notional})
				res.TotalQty += q
				res.TotalUSDT += notional
				remain = 0
				break
			}
		}
		res.Leftover = remain
	case Sell:
		levels := orderbook.CombinedBids(in.OrderBooks)
		remain := in.Amount
		for _, lv := range levels {
			if remain <= 0 {
				break
			}
			feeCfg := in.Fees[lv.Exchange]
			unitNet := lv.Price * (1 - feeCfg.FeePct)
			if unitNet <= 0 {
				continue
			}
			if remain >= lv.Qty {
				usdt := lv.Qty * unitNet
				if feeCfg.MinNotional > 0 && usdt < feeCfg.MinNotional {
					continue
				}
				res.Legs = append(res.Legs, Leg{Exchange: lv.Exchange, Price: unitNet, Qty: lv.Qty, AmountUSDT: usdt})
				res.TotalQty += lv.Qty
				res.TotalUSDT += usdt
				remain -= lv.Qty
			} else {
				usdt := remain * unitNet
				if feeCfg.MinNotional > 0 && usdt < feeCfg.MinNotional {
					break
				}
				res.Legs = append(res.Legs, Leg{Exchange: lv.Exchange, Price: unitNet, Qty: remain, AmountUSDT: usdt})
				res.TotalQty += remain
				res.TotalUSDT += usdt
				remain = 0
				break
			}
		}
		res.Leftover = remain
	}

	if res.TotalQty > 0 {
		res.AveragePrice = res.TotalUSDT / res.TotalQty
	}
	return res
}
