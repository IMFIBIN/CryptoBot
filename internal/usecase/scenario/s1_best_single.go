package scenario

import (
	"sort"

	"cryptobot/internal/usecase/orderbook"
)

type BestSingle struct{}

func (BestSingle) Name() string { return "Сценарий #1 (лучшая биржа)" }

func (BestSingle) Run(in Inputs) Result {
	res := Result{Asset: in.Right}
	type cand struct {
		ex    string
		qty   float64
		avg   float64
		net   float64
		fee   float64
		obQty float64 // для покрытия
	}
	var cs []cand

	for ex, ob := range in.OrderBooks {
		f, ok := in.Fees[ex]
		if !ok || ob == nil {
			continue
		}
		switch in.Direction {
		case Buy:
			qty, avg, spentNet, fee := orderbook.BuyQtyFromAsksWithFee(ob.Asks, in.Amount, f)
			if qty <= 0 {
				continue
			}
			cs = append(cs, cand{ex: ex, qty: qty, avg: avg, net: spentNet, fee: fee, obQty: qty})

		case Sell:
			receivedNet, avg, fee := orderbook.SellFromBidsWithFee(ob.Bids, in.Amount, f)
			if receivedNet <= 0 || avg <= 0 {
				continue
			}
			// qty == in.Amount (если хватило ликвидности)
			cs = append(cs, cand{ex: ex, qty: in.Amount, avg: avg, net: receivedNet, fee: fee, obQty: in.Amount})
		}
	}

	if len(cs) == 0 {
		return res
	}

	// выбрать лучшую биржу
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
	best := cs[0]

	if in.Direction == Buy {
		res.Legs = []Leg{{Exchange: best.ex, Price: best.avg, Qty: best.qty, AmountUSDT: best.net, FeeUSDT: best.fee}}
		res.TotalQty = best.qty
		res.TotalUSDT = best.net
		res.AveragePrice = best.avg
		res.Leftover = in.Amount - best.net
		res.Asset = in.Right
	} else {
		res.Legs = []Leg{{Exchange: best.ex, Price: best.avg, Qty: in.Amount, AmountUSDT: best.net, FeeUSDT: best.fee}}
		res.TotalQty = in.Amount
		res.TotalUSDT = best.net
		res.AveragePrice = best.avg
		res.Asset = in.Symbol[:len(in.Symbol)-4]
	}

	return res
}
