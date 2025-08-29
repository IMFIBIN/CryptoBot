package scenario

import "cryptobot/internal/usecase/orderbook"

type BestSingle struct{}

func (BestSingle) Name() string { return "Сценарий #1 (лучшая биржа)" }

func (BestSingle) Run(in Inputs) Result {
	var best Result

	switch in.Direction {
	case Buy:
		for ex, ob := range in.OrderBooks {
			if ob == nil || len(ob.Asks) == 0 {
				continue
			}
			fee := in.Fees[ex].FeePct
			qty, avg, spent := orderbook.BuyQtyFromAsksWithFee(ob.Asks, in.Amount, fee)
			if qty <= 0 {
				continue
			}
			// фильтры минимумов
			cfg := in.Fees[ex]
			if (cfg.MinQty > 0 && qty < cfg.MinQty) || (cfg.MinNotional > 0 && spent < cfg.MinNotional) {
				continue
			}
			r := Result{
				Legs:         []Leg{{Exchange: ex, Price: avg, Qty: qty, AmountUSDT: spent}},
				TotalQty:     qty,
				TotalUSDT:    spent,
				AveragePrice: avg,
				Leftover:     in.Amount - spent,
				Asset:        in.Right,
			}
			if best.TotalQty == 0 || r.AveragePrice < best.AveragePrice {
				best = r
			}
		}
	case Sell:
		for ex, ob := range in.OrderBooks {
			if ob == nil || len(ob.Bids) == 0 {
				continue
			}
			fee := in.Fees[ex].FeePct
			usdt, avg := orderbook.SellFromBidsWithFee(ob.Bids, in.Amount, fee)
			if usdt <= 0 {
				continue
			}
			cfg := in.Fees[ex]
			if cfg.MinNotional > 0 && usdt < cfg.MinNotional {
				continue
			}
			r := Result{
				Legs:         []Leg{{Exchange: ex, Price: avg, Qty: in.Amount, AmountUSDT: usdt}},
				TotalQty:     in.Amount,
				TotalUSDT:    usdt,
				AveragePrice: avg,
				Leftover:     0,
				Asset:        in.Symbol[:len(in.Symbol)-4],
			}
			if best.TotalUSDT == 0 || r.TotalUSDT > best.TotalUSDT {
				best = r
			}
		}
	}

	return best
}
