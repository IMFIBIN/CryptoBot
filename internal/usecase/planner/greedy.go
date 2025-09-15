package planner

import "math"

// greedyBuyUSD — покупка BASE на бюджет в USDT, агрегируя лучшие аски со всех бирж.
func greedyBuyUSD(books []Book, amountUSDT float64) (perEx map[string]float64, vwap float64, spent float64) {
	type cur struct {
		ex  string
		i   int
		lvl Level
	}
	// min-heap по ask
	h := make([]cur, 0, len(books))
	bookByEx := map[string]Book{}
	for _, b := range books {
		bookByEx[b.Exchange] = b
		if len(b.Asks) > 0 {
			h = append(h, cur{ex: b.Exchange, i: 0, lvl: b.Asks[0]})
		}
	}
	less := func(a, b cur) bool { return a.lvl.Price < b.lvl.Price }
	swap := func(i, j int) { h[i], h[j] = h[j], h[i] }
	down := func(i int) {
		for {
			l := 2*i + 1
			r := l + 1
			s := i
			if l < len(h) && less(h[l], h[s]) {
				s = l
			}
			if r < len(h) && less(h[r], h[s]) {
				s = r
			}
			if s == i {
				break
			}
			swap(i, s)
			i = s
		}
	}
	for i := len(h)/2 - 1; i >= 0; i-- {
		down(i)
	}

	perEx = map[string]float64{}
	var got, cost float64
	for amountUSDT > 1e-9 && len(h) > 0 {
		top := h[0]
		price, qty := top.lvl.Price, top.lvl.Qty
		if price <= 0 {
			// пропускаем некорректный уровень
			h[0].i++
			if h[0].i < len(bookByEx[top.ex].Asks) {
				h[0].lvl = bookByEx[top.ex].Asks[h[0].i]
				down(0)
			} else {
				h[0] = h[len(h)-1]
				h = h[:len(h)-1]
				if len(h) > 0 {
					down(0)
				}
			}
			continue
		}
		afford := amountUSDT / price
		take := math.Min(qty, afford)
		if take <= 1e-12 {
			break
		}
		perEx[top.ex] += take
		got += take
		cost += take * price
		amountUSDT -= take * price

		h[0].i++
		if h[0].i < len(bookByEx[top.ex].Asks) {
			h[0].lvl = bookByEx[top.ex].Asks[h[0].i]
			down(0)
		} else {
			h[0] = h[len(h)-1]
			h = h[:len(h)-1]
			if len(h) > 0 {
				down(0)
			}
		}
	}
	if got > 0 {
		vwap = cost / got // USDT за 1 BASE
	}
	return perEx, vwap, cost
}

// greedySellCoin — продажа монеты за USDT, агрегируя лучшие биды.
func greedySellCoin(books []Book, amountCoin float64) (perEx map[string]float64, vwap float64, proceeds float64, sold float64) {
	type cur struct {
		ex  string
		i   int
		lvl Level
	}
	// max-heap по bid
	h := make([]cur, 0, len(books))
	bookByEx := map[string]Book{}
	for _, b := range books {
		bookByEx[b.Exchange] = b
		if len(b.Bids) > 0 {
			h = append(h, cur{ex: b.Exchange, i: 0, lvl: b.Bids[0]})
		}
	}
	less := func(a, b cur) bool { return a.lvl.Price > b.lvl.Price }
	swap := func(i, j int) { h[i], h[j] = h[j], h[i] }
	down := func(i int) {
		for {
			l := 2*i + 1
			r := l + 1
			s := i
			if l < len(h) && less(h[l], h[s]) {
				s = l
			}
			if r < len(h) && less(h[r], h[s]) {
				s = r
			}
			if s == i {
				break
			}
			swap(i, s)
			i = s
		}
	}
	for i := len(h)/2 - 1; i >= 0; i-- {
		down(i)
	}

	perEx = map[string]float64{}
	var gotUSDT float64
	var used float64
	for amountCoin > 1e-12 && len(h) > 0 {
		top := h[0]
		price, qty := top.lvl.Price, top.lvl.Qty
		take := math.Min(qty, amountCoin)
		if take <= 1e-12 {
			break
		}
		perEx[top.ex] += take
		used += take
		gotUSDT += take * price
		amountCoin -= take

		h[0].i++
		if h[0].i < len(bookByEx[top.ex].Bids) {
			h[0].lvl = bookByEx[top.ex].Bids[h[0].i]
			down(0)
		} else {
			h[0] = h[len(h)-1]
			h = h[:len(h)-1]
			if len(h) > 0 {
				down(0)
			}
		}
	}
	if used > 0 {
		vwap = gotUSDT / used // USDT за 1 COIN
	}
	return perEx, vwap, gotUSDT, used
}
