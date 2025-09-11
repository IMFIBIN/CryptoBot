package scenario

import (
	"sort"
	"strconv"
)

type Optimal struct{}

func (Optimal) Name() string {
	return "Сценарий #3 (Лучшее распределение средств)"
}

func (Optimal) Run(in Inputs) Result {
	res := Result{Asset: in.Right}

	type legAgg struct {
		qty  float64
		usdt float64
	}

	legsByEx := map[string]*legAgg{}

	add := func(ex string, price, qty float64) {
		if qty <= 0 || price <= 0 {
			return
		}
		l := legsByEx[ex]
		if l == nil {
			l = &legAgg{}
			legsByEx[ex] = l
		}
		l.qty += qty
		l.usdt += price * qty
		res.TotalQty += qty
		res.TotalUSDT += price * qty
	}

	switch in.Direction {
	case Buy:
		// Собираем все аски всех бирж в единый массив (ex, price, qty)
		type level struct {
			ex    string
			price float64
			qty   float64
		}
		var all []level
		for ex, ob := range in.OrderBooks {
			if ob == nil {
				continue
			}
			for _, a := range ob.Asks {
				p, err1 := strconv.ParseFloat(a.Price, 64)
				q, err2 := strconv.ParseFloat(a.Quantity, 64)
				if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
					continue
				}
				all = append(all, level{ex: ex, price: p, qty: q})
			}
		}
		sort.Slice(all, func(i, j int) bool { return all[i].price < all[j].price })

		remainBudget := in.Amount
		for _, lv := range all {
			if remainBudget <= 0 {
				break
			}
			maxCost := lv.price * lv.qty
			if maxCost <= remainBudget {
				add(lv.ex, lv.price, lv.qty)
				remainBudget -= maxCost
			} else {
				q := remainBudget / lv.price
				if q > 0 {
					add(lv.ex, lv.price, q)
					remainBudget = 0
				}
				break
			}
		}
		res.Leftover = in.Amount - res.TotalUSDT
		res.Asset = in.Right

	case Sell:
		type level struct {
			ex    string
			price float64
			qty   float64
		}
		var all []level
		for ex, ob := range in.OrderBooks {
			if ob == nil {
				continue
			}
			for _, b := range ob.Bids {
				p, err1 := strconv.ParseFloat(b.Price, 64)
				q, err2 := strconv.ParseFloat(b.Quantity, 64)
				if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
					continue
				}
				all = append(all, level{ex: ex, price: p, qty: q})
			}
		}

		sort.Slice(all, func(i, j int) bool { return all[i].price > all[j].price })

		remainQty := in.Amount
		for _, lv := range all {
			if remainQty <= 0 {
				break
			}
			if lv.qty <= remainQty {
				add(lv.ex, lv.price, lv.qty)
				remainQty -= lv.qty
			} else {
				add(lv.ex, lv.price, remainQty)
				remainQty = 0
				break
			}
		}
		// Для SELL Asset — левая часть символа (без суффикса USDT)
		if len(in.Symbol) > 4 {
			res.Asset = in.Symbol[:len(in.Symbol)-4]
		} else {
			res.Asset = in.Symbol
		}
	default:
		return res
	}

	// Цена в ноге — средневзвешенная (usdt/qty)
	type exName struct{ name string }
	var keys []exName
	for ex := range legsByEx {
		keys = append(keys, exName{name: ex})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].name < keys[j].name })

	for _, k := range keys {
		l := legsByEx[k.name]
		if l == nil || l.qty <= 0 {
			continue
		}
		avg := l.usdt / l.qty
		res.Legs = append(res.Legs, Leg{
			Exchange:   k.name,
			Price:      avg,
			Qty:        l.qty,
			AmountUSDT: l.usdt,
		})
	}

	if res.TotalQty > 0 {
		res.AveragePrice = res.TotalUSDT / res.TotalQty
	}
	return res
}
