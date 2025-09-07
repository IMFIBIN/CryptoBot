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

type level struct{ Price, Qty float64 }
type book struct {
	Exchange string
	Asks     []level // asc by price
	Bids     []level // desc by price
}
type fetchDiag struct{ Exchange, Status string }

type RealFlow struct{ http *http.Client }

func New() *RealFlow { return &RealFlow{http: &http.Client{Timeout: 8 * time.Second}} }

func roundCents(x float64) float64 { return math.Round(x*100) / 100 }

// --- small HTTP helper with retry ---
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

func sortAsks(a []level) { sort.Slice(a, func(i, j int) bool { return a[i].Price < a[j].Price }) }
func sortBids(b []level) { sort.Slice(b, func(i, j int) bool { return b[i].Price > b[j].Price }) }

// ------------------ FETCHERS ------------------

// Binance
func (rf *RealFlow) fetchBinance(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	symbol := strings.ToUpper(coin) + "USDT"
	url := fmt.Sprintf("https://api.binance.com/api/v3/depth?limit=%d&symbol=%s", depth, symbol)
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
	inst := strings.ToUpper(coin) + "-USDT"
	url := fmt.Sprintf("https://www.okx.com/api/v5/market/books?instId=%s&sz=%d", inst, depth)
	var raw struct {
		Data []struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
		} `json:"data"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "okx"}, fetchDiag{"okx", err.Error()}
	}
	if len(raw.Data) == 0 {
		return book{Exchange: "okx"}, fetchDiag{"okx", "empty"}
	}
	var asks, bids []level
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
	sortAsks(asks)
	sortBids(bids)
	return book{Exchange: "okx", Asks: asks, Bids: bids}, fetchDiag{"okx", "ok"}
}

// Bybit
func (rf *RealFlow) fetchBybit(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	symbol := strings.ToUpper(coin) + "USDT"
	url := fmt.Sprintf("https://api.bybit.com/v5/market/orderbook?category=spot&symbol=%s&limit=%d", symbol, depth)
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
	return book{Exchange: "bybit", Asks: asks, Bids: bids}, fetchDiag{"bybit", "ok"}
}

// KuCoin
func (rf *RealFlow) fetchKucoin(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	symbol := strings.ToUpper(coin) + "-USDT"
	if depth > 100 {
		depth = 100
	}
	url := fmt.Sprintf("https://api.kucoin.com/api/v1/market/orderbook/level2_%d?symbol=%s", depth, symbol)
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
		return book{Exchange: "kucoin"}, fetchDiag{"kucoin", "non-200000"}
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
	return book{Exchange: "kucoin", Asks: asks, Bids: bids}, fetchDiag{"kucoin", "ok"}
}

// Gate
func (rf *RealFlow) fetchGate(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	pair := strings.ToUpper(coin) + "_USDT"
	url := fmt.Sprintf("https://api.gateio.ws/api/v4/spot/order_book?currency_pair=%s&limit=%d", pair, depth)
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
	return book{Exchange: "gate", Asks: asks, Bids: bids}, fetchDiag{"gate", "ok"}
}

// HTX (Huobi)
func (rf *RealFlow) fetchHTX(ctx context.Context, coin string, depth int) (book, fetchDiag) {
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
		return book{Exchange: "htx"}, fetchDiag{"htx", "status != ok"}
	}
	var asks, bids []level
	for _, a := range raw.Tick.Asks {
		if len(a) >= 2 && a[0] > 0 && a[1] > 0 {
			asks = append(asks, level{Price: a[0], Qty: a[1]})
		}
	}
	for _, b := range raw.Tick.Bids {
		if len(b) >= 2 && b[0] > 0 && b[1] > 0 {
			bids = append(bids, level{Price: b[0], Qty: b[1]})
		}
	}
	sortAsks(asks)
	sortBids(bids)
	return book{Exchange: "htx", Asks: asks, Bids: bids}, fetchDiag{"htx", "ok"}
}

// Bitget
func (rf *RealFlow) fetchBitget(ctx context.Context, coin string, depth int) (book, fetchDiag) {
	symbol := strings.ToUpper(coin) + "USDT"
	if depth > 100 {
		depth = 100
	}
	url := fmt.Sprintf("https://api.bitget.com/api/spot/v1/market/depth?symbol=%s&limit=%d", symbol, depth)
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
		return book{Exchange: "bitget"}, fetchDiag{"bitget", "non-00000"}
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
	return book{Exchange: "bitget", Asks: asks, Bids: bids}, fetchDiag{"bitget", "ok"}
}

// --- BUY greedy: spend USDT over asks ---
func greedyBuyUSD(books []book, amountUSDT float64) (perEx map[string]float64, vwap float64, spent float64) {
	type cur struct {
		ex  string
		i   int
		lvl level
	}
	// min-heap by ask price
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
				return
			}
			swap(i, s)
			i = s
		}
	}
	push := func(x cur) {
		h = append(h, x)
		for j := len(h) - 1; j > 0; {
			i := (j - 1) / 2
			if !(h[j].lvl.Price < h[i].lvl.Price) {
				break
			}
			h[i], h[j] = h[j], h[i]
			j = i
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
			got += qty
			cost = usdCap
			perEx[x.ex] += qty
			break
		}
	}
	if got > 0 {
		vwap = cost / got
	}
	return perEx, vwap, cost
}

// --- SELL greedy: sell coin qty over bids ---
func greedySellCoin(books []book, amountCoin float64) (perEx map[string]float64, vwap float64, proceeds float64, sold float64) {
	type cur struct {
		ex  string
		i   int
		lvl level
	}
	// max-heap by bid price
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
				return
			}
			swap(i, s)
			i = s
		}
	}
	push := func(x cur) {
		h = append(h, x)
		for j := len(h) - 1; j > 0; {
			i := (j - 1) / 2
			if !(h[j].lvl.Price > h[i].lvl.Price) {
				break
			}
			h[i], h[j] = h[j], h[i]
			j = i
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

	perEx = map[string]float64{}
	var usd float64
	remain := amountCoin

	for remain > 1e-12 {
		x, ok := pop()
		if !ok {
			break
		}
		q := x.lvl.Qty
		if q >= remain {
			usd += remain * x.lvl.Price
			perEx[x.ex] += remain
			sold += remain
			remain = 0
			break
		} else {
			usd += q * x.lvl.Price
			perEx[x.ex] += q
			sold += q
			remain -= q
			if b, ok := bookByEx[x.ex]; ok && x.i+1 < len(b.Bids) {
				push(cur{ex: x.ex, i: x.i + 1, lvl: b.Bids[x.i+1]})
			}
		}
	}
	if sold > 0 {
		vwap = usd / sold
	}
	return perEx, vwap, usd, sold
}

// helper: собрать книги по монете (монета всегда против USDT)
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
	for i := 0; i < 7; i++ {
		r := <-ch
		diags = append(diags, fmt.Sprintf("%s: %s", r.d.Exchange, r.d.Status))
		if len(r.b.Asks) > 0 || len(r.b.Bids) > 0 {
			books = append(books, r.b)
		}
	}
	return books, diags
}

// --------------- main Plan ---------------
func (rf *RealFlow) Plan(ctx context.Context, req httpapi.PlanRequest) (httpapi.PlanResponse, error) {
	depth := req.Depth
	if depth <= 0 || depth > 500 {
		depth = 100
	}

	isUSDT := func(s string) bool { return strings.EqualFold(s, "USDT") }
	sideBuy := !isUSDT(req.Base) && isUSDT(req.Quote)    // покупка монеты за USDT (amount в USDT)
	sideSell := isUSDT(req.Base) && !isUSDT(req.Quote)   // продажа монеты за USDT (amount в монете)
	sideRoute := !isUSDT(req.Base) && !isUSDT(req.Quote) // coinA -> USDT -> coinB

	const feeRate = 0.001 // 0.1%

	var legs []httpapi.PlanLeg
	var vwap, total, fees, unspent float64

	switch {
	case sideBuy:
		// книги по покупаемой монете
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
			usd := roundCents(qty * priceByEx[b.Exchange])
			fee := roundCents(usd * feeRate)
			fees += fee
			legs = append(legs, httpapi.PlanLeg{Exchange: b.Exchange, Amount: qty, Price: priceByEx[b.Exchange], Fee: fee})
		}
		fees = roundCents(fees)
		unspent = roundCents(amountUSDT - total)
		if unspent < 0 {
			unspent = 0
		}

	case sideSell:
		// книги по продаваемой монете (req.Quote)
		books, diags := rf.fetchAll(ctx, req.Quote, depth)
		if len(books) == 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("no order books (%s)", strings.Join(diags, "; "))
		}
		amountCoin := req.Amount
		if amountCoin <= 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("amount must be > 0 for sell")
		}

		perEx, v, usd, sold := greedySellCoin(books, amountCoin)
		vwap = roundCents(v)
		total = roundCents(usd)
		unspent = roundCents(amountCoin - sold)
		if unspent < 0 {
			unspent = 0
		}

		priceByEx := map[string]float64{}
		for _, b := range books {
			if len(b.Bids) > 0 {
				priceByEx[b.Exchange] = b.Bids[0].Price
			}
		}
		for _, b := range books {
			qty := perEx[b.Exchange]
			if qty <= 0 {
				continue
			}
			usd := roundCents(qty * priceByEx[b.Exchange])
			fee := roundCents(usd * feeRate)
			fees += fee
			legs = append(legs, httpapi.PlanLeg{Exchange: b.Exchange, Amount: qty, Price: priceByEx[b.Exchange], Fee: fee})
		}
		fees = roundCents(fees)

	case sideRoute:
		// 1) продаём quote -> USDT
		booksSell, diags1 := rf.fetchAll(ctx, req.Quote, depth)
		// 2) покупаем base за USDT
		booksBuy, diags2 := rf.fetchAll(ctx, req.Base, depth)
		if len(booksSell) == 0 || len(booksBuy) == 0 {
			return httpapi.PlanResponse{}, fmt.Errorf("no order books (sell:%s | buy:%s)", strings.Join(diags1, "; "), strings.Join(diags2, "; "))
		}

		amountCoin := req.Amount
		perExS, _, usdProceeds, sold := greedySellCoin(booksSell, amountCoin)

		// комиссии первой стадии
		priceBid := map[string]float64{}
		for _, b := range booksSell {
			if len(b.Bids) > 0 {
				priceBid[b.Exchange] = b.Bids[0].Price
			}
		}
		for _, b := range booksSell {
			q := perExS[b.Exchange]
			if q <= 0 {
				continue
			}
			usd := roundCents(q * priceBid[b.Exchange])
			fee := roundCents(usd * feeRate)
			fees += fee
			legs = append(legs, httpapi.PlanLeg{Exchange: b.Exchange, Amount: q, Price: priceBid[b.Exchange], Fee: fee})
		}

		// 2) тратим полученные USDT на покупку base
		perExB, vB, usdSpent := greedyBuyUSD(booksBuy, usdProceeds)
		priceAsk := map[string]float64{}
		for _, b := range booksBuy {
			if len(b.Asks) > 0 {
				priceAsk[b.Exchange] = b.Asks[0].Price
			}
		}
		for _, b := range booksBuy {
			q := perExB[b.Exchange]
			if q <= 0 {
				continue
			}
			usd := roundCents(q * priceAsk[b.Exchange])
			fee := roundCents(usd * feeRate)
			fees += fee
			legs = append(legs, httpapi.PlanLeg{Exchange: b.Exchange, Amount: q, Price: priceAsk[b.Exchange], Fee: fee})
		}

		fees = roundCents(fees)
		total = roundCents(usdSpent)                 // потраченный USDT на покупку base
		unspent = roundCents(usdProceeds - usdSpent) // остаток USDT
		if unspent < 0 {
			unspent = 0
		}

		// эффективная цена base в единицах quote: сколько quote за 1 base
		var gotBase float64
		if vB > 0 {
			gotBase = usdSpent / vB
		}
		if gotBase > 0 {
			vwap = sold / gotBase
		} else {
			vwap = 0
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
		TotalFees:   fees,
		Unspent:     unspent,
		Legs:        legs,
		GeneratedAt: time.Now().Format("15:04 02.01.2006"),
	}
	return resp, nil
}
