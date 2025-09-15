package realflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"cryptobot/internal/transport/httpapi"
)

const userAgent = "CryptoBot/1.0 (+local)"

// уровень стакана
type level struct {
	Price float64
	Qty   float64
}

// книга по бирже
type book struct {
	Exchange string
	Asks     []level // ask: цена ↑
	Bids     []level // bid: цена ↓
}

// диагностика фетча
type fetchDiag struct {
	Exchange string
	Status   string
}

type RealFlow struct {
	http *http.Client
}

func New() *RealFlow {
	return &RealFlow{http: &http.Client{Timeout: 8 * time.Second}}
}

func roundCents(x float64) float64 { return math.Round(x*100) / 100 }

// сортировки
func sortAsks(xs []level) { sort.Slice(xs, func(i, j int) bool { return xs[i].Price < xs[j].Price }) }
func sortBids(xs []level) { sort.Slice(xs, func(i, j int) bool { return xs[i].Price > xs[j].Price }) }

// небольшой HTTP-хелпер с ретраями
func httpGetJSON[T any](ctx context.Context, c *http.Client, url string, out *T) error {
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := c.Do(req)
		if err != nil {
			last = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			last = fmt.Errorf("http %d", resp.StatusCode)
			_ = resp.Body.Close()
			time.Sleep(200 * time.Millisecond)
			continue
		}
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(out)
		_ = resp.Body.Close()
		if err != nil {
			last = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		return nil
	}
	return last
}

// BINANCE
func (rf *RealFlow) fetchBinance(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	d := depth
	if d <= 0 || d > 5000 {
		d = 5000
	}
	symbol := strings.ToUpper(coin) + "USDT"
	url := fmt.Sprintf("https://api.binance.com/api/v3/depth?limit=%d&symbol=%s", d, symbol)
	var raw struct {
		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "binance"}, fetchDiag{"binance", err.Error()}
	}
	var asks, bids []level
	for _, a := range raw.Asks {
		if len(a) >= 2 {
			p, _ := strconv.ParseFloat(a[0], 64)
			q, _ := strconv.ParseFloat(a[1], 64)
			if p > 0 && q > 0 {
				asks = append(asks, level{Price: p, Qty: q})
			}
		}
	}
	for _, b := range raw.Bids {
		if len(b) >= 2 {
			p, _ := strconv.ParseFloat(b[0], 64)
			q, _ := strconv.ParseFloat(b[1], 64)
			if p > 0 && q > 0 {
				bids = append(bids, level{Price: p, Qty: q})
			}
		}
	}
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return book{Exchange: "binance"}, fetchDiag{"binance", "empty"}
	}
	return book{Exchange: "binance", Asks: asks, Bids: bids}, fetchDiag{"binance", "ok"}
}

// OKX
func (rf *RealFlow) fetchOKX(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	d := depth
	if d <= 0 || d > 400 {
		d = 400
	}
	inst := strings.ToUpper(coin) + "-USDT"
	url := fmt.Sprintf("https://www.okx.com/api/v5/market/books?instId=%s&sz=%d", inst, d)
	var raw struct {
		Data []struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
		} `json:"data"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "okx"}, fetchDiag{"okx", err.Error()}
	}
	var asks, bids []level
	if len(raw.Data) > 0 {
		for _, a := range raw.Data[0].Asks {
			if len(a) >= 2 {
				p, _ := strconv.ParseFloat(a[0], 64)
				q, _ := strconv.ParseFloat(a[1], 64)
				if p > 0 && q > 0 {
					asks = append(asks, level{Price: p, Qty: q})
				}
			}
		}
		for _, b := range raw.Data[0].Bids {
			if len(b) >= 2 {
				p, _ := strconv.ParseFloat(b[0], 64)
				q, _ := strconv.ParseFloat(b[1], 64)
				if p > 0 && q > 0 {
					bids = append(bids, level{Price: p, Qty: q})
				}
			}
		}
	}
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return book{Exchange: "okx"}, fetchDiag{"okx", "empty"}
	}
	return book{Exchange: "okx", Asks: asks, Bids: bids}, fetchDiag{"okx", "ok"}
}

// BYBIT
func (rf *RealFlow) fetchBybit(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	d := depth
	if d <= 0 || d > 200 {
		d = 200
	}
	symbol := strings.ToUpper(coin) + "USDT"
	url := fmt.Sprintf("https://api.bybit.com/v5/market/orderbook?category=spot&symbol=%s&limit=%d", symbol, d)
	var raw struct {
		Result struct {
			A [][]string `json:"a"`
			B [][]string `json:"b"`
		} `json:"result"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "bybit"}, fetchDiag{"bybit", err.Error()}
	}
	var asks, bids []level
	for _, a := range raw.Result.A {
		if len(a) >= 2 {
			p, _ := strconv.ParseFloat(a[0], 64)
			q, _ := strconv.ParseFloat(a[1], 64)
			if p > 0 && q > 0 {
				asks = append(asks, level{Price: p, Qty: q})
			}
		}
	}
	for _, b := range raw.Result.B {
		if len(b) >= 2 {
			p, _ := strconv.ParseFloat(b[0], 64)
			q, _ := strconv.ParseFloat(b[1], 64)
			if p > 0 && q > 0 {
				bids = append(bids, level{Price: p, Qty: q})
			}
		}
	}
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return book{Exchange: "bybit"}, fetchDiag{"bybit", "empty"}
	}
	return book{Exchange: "bybit", Asks: asks, Bids: bids}, fetchDiag{"bybit", "ok"}
}

// KUCOIN
func (rf *RealFlow) fetchKucoin(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	d := depth
	if d <= 0 || d > 100 {
		d = 100
	}
	symbol := strings.ToUpper(coin) + "-USDT"
	url := fmt.Sprintf("https://api.kucoin.com/api/v1/market/orderbook/level2_%d?symbol=%s", d, symbol)
	var raw struct {
		Code string `json:"code"`
		Data struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
		} `json:"data"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "kucoin"}, fetchDiag{"kucoin", err.Error()}
	}
	if raw.Code != "200000" {
		return book{Exchange: "kucoin"}, fetchDiag{"kucoin", "bad code " + raw.Code}
	}
	var asks, bids []level
	for _, a := range raw.Data.Asks {
		if len(a) >= 2 {
			p, _ := strconv.ParseFloat(a[0], 64)
			q, _ := strconv.ParseFloat(a[1], 64)
			if p > 0 && q > 0 {
				asks = append(asks, level{Price: p, Qty: q})
			}
		}
	}
	for _, b := range raw.Data.Bids {
		if len(b) >= 2 {
			p, _ := strconv.ParseFloat(b[0], 64)
			q, _ := strconv.ParseFloat(b[1], 64)
			if p > 0 && q > 0 {
				bids = append(bids, level{Price: p, Qty: q})
			}
		}
	}
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return book{Exchange: "kucoin"}, fetchDiag{"kucoin", "empty"}
	}
	return book{Exchange: "kucoin", Asks: asks, Bids: bids}, fetchDiag{"kucoin", "ok"}
}

// GATE
func (rf *RealFlow) fetchGate(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	d := depth
	if d <= 0 || d > 200 {
		d = 200
	}
	pair := strings.ToUpper(coin) + "_USDT"
	url := fmt.Sprintf("https://api.gateio.ws/api/v4/spot/order_book?currency_pair=%s&limit=%d", pair, d)
	var raw struct {
		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "gate"}, fetchDiag{"gate", err.Error()}
	}
	var asks, bids []level
	for _, a := range raw.Asks {
		if len(a) >= 2 {
			p, _ := strconv.ParseFloat(a[0], 64)
			q, _ := strconv.ParseFloat(a[1], 64)
			if p > 0 && q > 0 {
				asks = append(asks, level{Price: p, Qty: q})
			}
		}
	}
	for _, b := range raw.Bids {
		if len(b) >= 2 {
			p, _ := strconv.ParseFloat(b[0], 64)
			q, _ := strconv.ParseFloat(b[1], 64)
			if p > 0 && q > 0 {
				bids = append(bids, level{Price: p, Qty: q})
			}
		}
	}
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return book{Exchange: "gate"}, fetchDiag{"gate", "empty"}
	}
	return book{Exchange: "gate", Asks: asks, Bids: bids}, fetchDiag{"gate", "ok"}
}

// HTX (Huobi)
func (rf *RealFlow) fetchHTX(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	d := depth
	if d <= 0 || d > 200 {
		d = 200
	}
	symbol := strings.ToLower(coin) + "usdt"
	url := fmt.Sprintf("https://api.huobi.pro/market/depth?symbol=%s&type=step0", symbol)
	var raw struct {
		Status string `json:"status"`
		Tick   struct {
			Asks [][]float64 `json:"asks"`
			Bids [][]float64 `json:"bids"`
		} `json:"tick"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "htx"}, fetchDiag{"htx", err.Error()}
	}
	if raw.Status != "ok" {
		return book{Exchange: "htx"}, fetchDiag{"htx", "status " + raw.Status}
	}
	var asks, bids []level
	for i, a := range raw.Tick.Asks {
		if i >= d {
			break
		}
		if len(a) >= 2 {
			p, q := a[0], a[1]
			if p > 0 && q > 0 {
				asks = append(asks, level{Price: p, Qty: q})
			}
		}
	}
	for i, b := range raw.Tick.Bids {
		if i >= d {
			break
		}
		if len(b) >= 2 {
			p, q := b[0], b[1]
			if p > 0 && q > 0 {
				bids = append(bids, level{Price: p, Qty: q})
			}
		}
	}
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return book{Exchange: "htx"}, fetchDiag{"htx", "empty"}
	}
	return book{Exchange: "htx", Asks: asks, Bids: bids}, fetchDiag{"htx", "ok"}
}

// BITGET
func (rf *RealFlow) fetchBitget(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	d := depth
	if d <= 0 || d > 100 {
		d = 100
	}
	symbol := strings.ToUpper(coin) + "USDT"
	url := fmt.Sprintf("https://api.bitget.com/api/spot/v1/market/depth?symbol=%s&limit=%d", symbol, d)
	var raw struct {
		Code string `json:"code"`
		Data struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
		} `json:"data"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "bitget"}, fetchDiag{"bitget", err.Error()}
	}
	if raw.Code != "00000" {
		return book{Exchange: "bitget"}, fetchDiag{"bitget", "bad code " + raw.Code}
	}
	var asks, bids []level
	for _, a := range raw.Data.Asks {
		if len(a) >= 2 {
			p, _ := strconv.ParseFloat(a[0], 64)
			q, _ := strconv.ParseFloat(a[1], 64)
			if p > 0 && q > 0 {
				asks = append(asks, level{Price: p, Qty: q})
			}
		}
	}
	for _, b := range raw.Data.Bids {
		if len(b) >= 2 {
			p, _ := strconv.ParseFloat(b[0], 64)
			q, _ := strconv.ParseFloat(b[1], 64)
			if p > 0 && q > 0 {
				bids = append(bids, level{Price: p, Qty: q})
			}
		}
	}
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return book{Exchange: "bitget"}, fetchDiag{"bitget", "empty"}
	}
	return book{Exchange: "bitget", Asks: asks, Bids: bids}, fetchDiag{"bitget", "ok"}
}

// собрать книги по монете (монета всегда против USDT)
func (rf *RealFlow) fetchAll(ctx context.Context, coin string, depth int) ([]book, []string) {
	type res struct {
		b book
		d fetchDiag
	}
	ch := make(chan res, 7)
	go func() { b, d := rf.fetchBinance(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchOKX(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchBybit(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchKucoin(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchGate(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchHTX(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchBitget(ctx, coin, depth); ch <- res{b, d} }()

	var books []book
	diags := make([]string, 0, 7)
deadline:
	for i := 0; i < 7; i++ {
		select {
		case r := <-ch:
			if len(r.b.Asks) > 0 || len(r.b.Bids) > 0 {
				books = append(books, r.b)
			}
			diags = append(diags, r.d.Exchange+": "+r.d.Status)
		case <-time.After(3 * time.Second):
			diags = append(diags, "timeout")
			break deadline
		}
	}
	// фиксированный порядок: по лучшему аску
	sort.SliceStable(books, func(i, j int) bool {
		var ai, aj = 1e18, 1e18
		if len(books[i].Asks) > 0 {
			ai = books[i].Asks[0].Price
		}
		if len(books[j].Asks) > 0 {
			aj = books[j].Asks[0].Price
		}
		return ai < aj
	})
	return books, diags
}

// ------------------ GREEDY ------------------

// покупка BASE за USDT с наилучшим VWAP
func greedyBuyUSD(books []book, amountUSDT float64) (perEx map[string]float64, vwap float64, spent float64) {
	type cur struct {
		ex  string
		i   int
		lvl level
	}
	// min-heap по ask
	h := make([]cur, 0, len(books))
	bookByEx := map[string]book{}
	for _, b := range books {
		bookByEx[b.Exchange] = b
		if len(b.Asks) > 0 {
			h = append(h, cur{ex: b.Exchange, i: 0, lvl: b.Asks[0]})
		}
	}
	less := func(i, j int) bool { return h[i].lvl.Price < h[j].lvl.Price }
	swap := func(i, j int) { h[i], h[j] = h[j], h[i] }
	down := func(i, n int) {
		for {
			l, r := 2*i+1, 2*i+2
			s := i
			if l < n && less(l, s) {
				s = l
			}
			if r < n && less(r, s) {
				s = r
			}
			if s == i {
				break
			}
			swap(i, s)
			i = s
		}
	}
	up := func(i int) {
		for {
			p := (i - 1) / 2
			if i == 0 || !less(i, p) {
				break
			}
			swap(i, p)
			i = p
		}
	}
	pop := func() (cur, bool) {
		if len(h) == 0 {
			return cur{}, false
		}
		x := h[0]
		n := len(h) - 1
		h[0] = h[n]
		h = h[:n]
		down(0, len(h))
		return x, true
	}
	push := func(x cur) {
		h = append(h, x)
		up(len(h) - 1)
	}

	perEx = map[string]float64{}
	var got, cost float64
	usdCap := amountUSDT

	for cost < usdCap {
		x, ok := pop()
		if !ok {
			break
		}
		maxUSD := x.lvl.Price * x.lvl.Qty
		if cost+maxUSD <= usdCap {
			got += x.lvl.Qty
			cost += maxUSD
			perEx[x.ex] += x.lvl.Qty
			if b, ok := bookByEx[x.ex]; ok && x.i+1 < len(b.Asks) {
				push(cur{ex: x.ex, i: x.i + 1, lvl: b.Asks[x.i+1]})
			}
		} else {
			leftUSD := usdCap - cost
			qty := leftUSD / x.lvl.Price
			if qty > 0 {
				got += qty
				cost += qty * x.lvl.Price
				perEx[x.ex] += qty
			}
			break
		}
	}
	if got > 0 {
		vwap = cost / got
	}
	return perEx, vwap, cost
}

// продажа монеты за USDT с наилучшим VWAP
func greedySellCoin(books []book, amountCoin float64) (perEx map[string]float64, vwap float64, proceeds float64, sold float64) {
	type cur struct {
		ex  string
		i   int
		lvl level
	}
	// max-heap по bid
	h := make([]cur, 0, len(books))
	bookByEx := map[string]book{}
	for _, b := range books {
		bookByEx[b.Exchange] = b
		if len(b.Bids) > 0 {
			h = append(h, cur{ex: b.Exchange, i: 0, lvl: b.Bids[0]})
		}
	}
	less := func(i, j int) bool { return h[i].lvl.Price > h[j].lvl.Price }
	swap := func(i, j int) { h[i], h[j] = h[j], h[i] }
	down := func(i, n int) {
		for {
			l, r := 2*i+1, 2*i+2
			s := i
			if l < n && less(l, s) {
				s = l
			}
			if r < n && less(r, s) {
				s = r
			}
			if s == i {
				break
			}
			swap(i, s)
			i = s
		}
	}
	up := func(i int) {
		for {
			p := (i - 1) / 2
			if i == 0 || !less(i, p) {
				break
			}
			swap(i, p)
			i = p
		}
	}
	pop := func() (cur, bool) {
		if len(h) == 0 {
			return cur{}, false
		}
		x := h[0]
		n := len(h) - 1
		h[0] = h[n]
		h = h[:n]
		down(0, len(h))
		return x, true
	}
	push := func(x cur) {
		h = append(h, x)
		up(len(h) - 1)
	}

	perEx = map[string]float64{}
	var usd, coin float64
	capCoin := amountCoin

	for coin < capCoin {
		x, ok := pop()
		if !ok {
			break
		}
		if coin+x.lvl.Qty <= capCoin {
			coin += x.lvl.Qty
			usd += x.lvl.Qty * x.lvl.Price
			perEx[x.ex] += x.lvl.Qty
			if b, ok := bookByEx[x.ex]; ok && x.i+1 < len(b.Bids) {
				push(cur{ex: x.ex, i: x.i + 1, lvl: b.Bids[x.i+1]})
			}
		} else {
			left := capCoin - coin
			if left > 0 {
				coin += left
				usd += left * x.lvl.Price
				perEx[x.ex] += left
			}
			break
		}
	}
	if coin > 0 {
		vwap = usd / coin
	}
	return perEx, vwap, usd, coin
}

// ------------------ MAIN PLAN ------------------

func (rf *RealFlow) Plan(ctx context.Context, req httpapi.PlanRequest) (httpapi.PlanResponse, error) {
	if strings.TrimSpace(req.Base) == "" || strings.TrimSpace(req.Quote) == "" {
		return httpapi.PlanResponse{}, fmt.Errorf("unsupported pair selection")
	}
	if req.Amount <= 0 {
		return httpapi.PlanResponse{}, fmt.Errorf("amount must be > 0")
	}

	depth := 0

	isUSDT := func(s string) bool { return strings.EqualFold(s, "USDT") }
	sideBuy := !isUSDT(req.Base) && isUSDT(req.Quote)
	sideSell := isUSDT(req.Base) && !isUSDT(req.Quote)
	sideRoute := !isUSDT(req.Base) && !isUSDT(req.Quote)

	var legs []httpapi.PlanLeg
	var vwap, total, unspent float64

	switch {
	case sideBuy:
		books, diags := rf.fetchAll(ctx, req.Base, depth)
		if len(books) == 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("no order books (%s)", strings.Join(diags, "; "))
		}
		amountUSDT := math.Floor(req.Amount + 1e-9)
		if amountUSDT < 1 {
			amountUSDT = 1
		}

		perEx, v, cost := greedyBuyUSD(books, amountUSDT)
		vwap = roundCents(v)
		total = roundCents(cost)

		priceByEx := map[string]float64{}
		for _, b := range books {
			if len(b.Asks) > 0 {
				priceByEx[b.Exchange] = b.Asks[0].Price
			}
		}
		for _, b := range books {
			qty := perEx[b.Exchange]
			if qty <= 0 {
				continue
			}
			legs = append(legs, httpapi.PlanLeg{
				Exchange: b.Exchange,
				Amount:   qty,
				Price:    priceByEx[b.Exchange],
			})
		}
		unspent = roundCents(amountUSDT - total)
		if unspent < 0 {
			unspent = 0
		}

	case sideSell:
		books, diags := rf.fetchAll(ctx, req.Quote, depth)
		if len(books) == 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("no order books (%s)", strings.Join(diags, "; "))
		}
		amountCoin := req.Amount
		if amountCoin <= 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("amount must be > 0")
		}
		perExS, v, usdProceeds, sold := greedySellCoin(books, amountCoin)
		vwap = roundCents(v)

		priceBid := map[string]float64{}
		for _, b := range books {
			if len(b.Bids) > 0 {
				priceBid[b.Exchange] = b.Bids[0].Price
			}
		}
		for _, b := range books {
			q := perExS[b.Exchange]
			if q <= 0 {
				continue
			}
			legs = append(legs, httpapi.PlanLeg{
				Exchange: b.Exchange,
				Amount:   q,
				Price:    priceBid[b.Exchange],
			})
		}

		total = roundCents(usdProceeds)
		unspent = roundCents(amountCoin - sold)
		if unspent < 0 {
			unspent = 0
		}

	case sideRoute:
		booksBase, diags1 := rf.fetchAll(ctx, req.Base, depth)
		if len(booksBase) == 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("no order books on BASE leg (%s)", strings.Join(diags1, "; "))
		}
		amountBase := req.Amount
		perExSell, _, usdProceeds, soldBase := greedySellCoin(booksBase, amountBase)

		// Собираем ножки по продаже (цены — лучшие bid)
		priceBid := map[string]float64{}
		for _, b := range booksBase {
			if len(b.Bids) > 0 {
				priceBid[b.Exchange] = b.Bids[0].Price
			}
		}
		for _, b := range booksBase {
			q := perExSell[b.Exchange]
			if q > 0 {
				legs = append(legs, httpapi.PlanLeg{
					Exchange: b.Exchange,
					Amount:   q,
					Price:    priceBid[b.Exchange],
				})
			}
		}

		if usdProceeds <= 0 || soldBase <= 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("insufficient depth on BASE->USDT leg")
		}

		booksQuote, diags2 := rf.fetchAll(ctx, req.Quote, depth)
		if len(booksQuote) == 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("no order books on USDT->QUOTE leg (%s)", strings.Join(diags2, "; "))
		}

		perExBuy, _, usdSpent := greedyBuyUSD(booksQuote, usdProceeds)

		var gotQuote float64
		priceAsk := map[string]float64{}
		for _, b := range booksQuote {
			if len(b.Asks) > 0 {
				priceAsk[b.Exchange] = b.Asks[0].Price
			}
		}
		for _, b := range booksQuote {
			q := perExBuy[b.Exchange]
			if q > 0 {
				gotQuote += q
				legs = append(legs, httpapi.PlanLeg{
					Exchange: b.Exchange,
					Amount:   q,
					Price:    priceAsk[b.Exchange],
				})
			}
		}

		if gotQuote <= 0 || usdSpent <= 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("insufficient depth on USDT->QUOTE leg")
		}

		vwap = roundCents(soldBase / gotQuote)
		total = roundCents(soldBase)
		unspent = roundCents(req.Amount - soldBase)
		if unspent < 0 {
			unspent = 0
		}

	default:
		return httpapi.PlanResponse{}, fmt.Errorf("unsupported pair selection")
	}

	resp := httpapi.PlanResponse{
		Base:        strings.ToUpper(req.Base),
		Quote:       strings.ToUpper(req.Quote),
		Amount:      req.Amount,
		Scenario:    req.Scenario,
		VWAP:        vwap,
		TotalCost:   total,
		Unspent:     unspent,
		Legs:        legs,
		GeneratedAt: time.Now().Format("15:04 02.01.2006"),
	}
	return resp, nil
}
