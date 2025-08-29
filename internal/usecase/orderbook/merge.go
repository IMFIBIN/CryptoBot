package orderbook

import (
	"sort"
	"strconv"

	"cryptobot/internal/domain"
)

type Level struct {
	Exchange string
	Price    float64
	Qty      float64
}

// Слияние всех asks (для BUY) — в порядке возрастания цены.
func CombinedAsks(books map[string]*domain.OrderBook) []Level {
	var out []Level
	for ex, ob := range books {
		if ob == nil {
			continue
		}
		for _, a := range ob.Asks {
			p, err1 := strconv.ParseFloat(a.Price, 64)
			q, err2 := strconv.ParseFloat(a.Quantity, 64)
			if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
				continue
			}
			out = append(out, Level{Exchange: ex, Price: p, Qty: q})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

// Слияние всех bids (для SELL) — в порядке убывания цены.
func CombinedBids(books map[string]*domain.OrderBook) []Level {
	var out []Level
	for ex, ob := range books {
		if ob == nil {
			continue
		}
		for _, b := range ob.Bids {
			p, err1 := strconv.ParseFloat(b.Price, 64)
			q, err2 := strconv.ParseFloat(b.Quantity, 64)
			if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
				continue
			}
			out = append(out, Level{Exchange: ex, Price: p, Qty: q})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Price > out[j].Price })
	return out
}
