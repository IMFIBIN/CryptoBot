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

// ------------------ FETCHERS ------------------

// Binance
func (rf *RealFlow) fetchBinance(ctx context.Context, base string, depth int) (book, fetchDiag) {
	symbol := strings.ToUpper(base) + "USDT"
	url := fmt.Sprintf("https://api.binance.com/api/v3/depth?limit=%d&symbol=%s", depth, symbol)
	var raw struct {
		Asks [][]string `json:"asks"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "binance"}, fetchDiag{"binance", err.Error()}
	}
	asks := make([]level, 0, len(raw.Asks))
	for _, a := range raw.Asks {
		if len(a) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(a[0], 64)
		q, _ := strconv.ParseFloat(a[1], 64)
		if p > 0 && q > 0 {
			asks = append(asks, level{Price: p, Qty: q})
		}
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	if len(asks) == 0 {
		return book{Exchange: "binance"}, fetchDiag{"binance", "200 but 0 asks"}
	}
	return book{Exchange: "binance", Asks: asks}, fetchDiag{"binance", "ok"}
}

// OKX
func (rf *RealFlow) fetchOKX(ctx context.Context, base string, depth int) (book, fetchDiag) {
	inst := strings.ToUpper(base) + "-USDT"
	url := fmt.Sprintf("https://www.okx.com/api/v5/market/books?instId=%s&sz=%d", inst, depth)
	var raw struct {
		Data []struct {
			Asks [][]string `json:"asks"`
		} `json:"data"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "okx"}, fetchDiag{"okx", err.Error()}
	}
	if len(raw.Data) == 0 {
		return book{Exchange: "okx"}, fetchDiag{"okx", "200 but empty"}
	}
	asks := make([]level, 0, len(raw.Data[0].Asks))
	for _, a := range raw.Data[0].Asks {
		if len(a) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(a[0], 64)
		q, _ := strconv.ParseFloat(a[1], 64)
		if p > 0 && q > 0 {
			asks = append(asks, level{Price: p, Qty: q})
		}
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	if len(asks) == 0 {
		return book{Exchange: "okx"}, fetchDiag{"okx", "200 but 0 asks"}
	}
	return book{Exchange: "okx", Asks: asks}, fetchDiag{"okx", "ok"}
}

// Bybit
func (rf *RealFlow) fetchBybit(ctx context.Context, base string, depth int) (book, fetchDiag) {
	symbol := strings.ToUpper(base) + "USDT"
	url := fmt.Sprintf("https://api.bybit.com/v5/market/orderbook?category=spot&symbol=%s&limit=%d", symbol, depth)
	var raw struct {
		Result struct {
			A [][]string `json:"a"`
		} `json:"result"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "bybit"}, fetchDiag{"bybit", err.Error()}
	}
	asks := make([]level, 0, len(raw.Result.A))
	for _, a := range raw.Result.A {
		if len(a) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(a[0], 64)
		q, _ := strconv.ParseFloat(a[1], 64)
		if p > 0 && q > 0 {
			asks = append(asks, level{Price: p, Qty: q})
		}
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	if len(asks) == 0 {
		return book{Exchange: "bybit"}, fetchDiag{"bybit", "200 but 0 asks"}
	}
	return book{Exchange: "bybit", Asks: asks}, fetchDiag{"bybit", "ok"}
}

// KuCoin
func (rf *RealFlow) fetchKucoin(ctx context.Context, base string, depth int) (book, fetchDiag) {
	symbol := strings.ToUpper(base) + "-USDT"
	if depth > 100 {
		depth = 100
	}
	url := fmt.Sprintf("https://api.kucoin.com/api/v1/market/orderbook/level2_%d?symbol=%s", depth, symbol)
	var raw struct {
		Code string `json:"code"`
		Data struct {
			Asks [][]string `json:"asks"`
		} `json:"data"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "kucoin"}, fetchDiag{"kucoin", err.Error()}
	}
	if raw.Code != "200000" {
		return book{Exchange: "kucoin"}, fetchDiag{"kucoin", "non-200000"}
	}
	asks := make([]level, 0, len(raw.Data.Asks))
	for _, a := range raw.Data.Asks {
		if len(a) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(a[0], 64)
		q, _ := strconv.ParseFloat(a[1], 64)
		if p > 0 && q > 0 {
			asks = append(asks, level{Price: p, Qty: q})
		}
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	if len(asks) == 0 {
		return book{Exchange: "kucoin"}, fetchDiag{"kucoin", "200 but 0 asks"}
	}
	return book{Exchange: "kucoin", Asks: asks}, fetchDiag{"kucoin", "ok"}
}

// Gate
func (rf *RealFlow) fetchGate(ctx context.Context, base string, depth int) (book, fetchDiag) {
	pair := strings.ToUpper(base) + "_USDT"
	url := fmt.Sprintf("https://api.gateio.ws/api/v4/spot/order_book?currency_pair=%s&limit=%d", pair, depth)
	var raw struct {
		Asks [][]string `json:"asks"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "gate"}, fetchDiag{"gate", err.Error()}
	}
	asks := make([]level, 0, len(raw.Asks))
	for _, a := range raw.Asks {
		if len(a) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(a[0], 64)
		q, _ := strconv.ParseFloat(a[1], 64)
		if p > 0 && q > 0 {
			asks = append(asks, level{Price: p, Qty: q})
		}
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	if len(asks) == 0 {
		return book{Exchange: "gate"}, fetchDiag{"gate", "200 but 0 asks"}
	}
	return book{Exchange: "gate", Asks: asks}, fetchDiag{"gate", "ok"}
}

// HTX (Huobi)
func (rf *RealFlow) fetchHTX(ctx context.Context, base string, depth int) (book, fetchDiag) {
	symbol := strings.ToLower(base) + "usdt"
	if depth > 200 {
		depth = 200
	}
	url := fmt.Sprintf("https://api.huobi.pro/market/depth?symbol=%s&type=step0", symbol)
	var raw struct {
		Status string `json:"status"`
		Tick   struct {
			Asks [][]float64 `json:"asks"`
		} `json:"tick"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "htx"}, fetchDiag{"htx", err.Error()}
	}
	if raw.Status != "ok" {
		return book{Exchange: "htx"}, fetchDiag{"htx", "status != ok"}
	}
	asks := make([]level, 0, len(raw.Tick.Asks))
	for _, a := range raw.Tick.Asks {
		if len(a) < 2 {
			continue
		}
		p, q := a[0], a[1]
		if p > 0 && q > 0 {
			asks = append(asks, level{Price: p, Qty: q})
		}
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	if len(asks) == 0 {
		return book{Exchange: "htx"}, fetchDiag{"htx", "200 but 0 asks"}
	}
	return book{Exchange: "htx", Asks: asks}, fetchDiag{"htx", "ok"}
}

// Bitget
func (rf *RealFlow) fetchBitget(ctx context.Context, base string, depth int) (book, fetchDiag) {
	symbol := strings.ToUpper(base) + "USDT"
	if depth > 100 {
		depth = 100
	}
	url := fmt.Sprintf("https://api.bitget.com/api/spot/v1/market/depth?symbol=%s&limit=%d", symbol, depth)
	var raw struct {
		Code string `json:"code"`
		Data struct {
			Asks [][]string `json:"asks"`
		} `json:"data"`
	}
	if err := httpGetJSON(ctx, rf.http, url, &raw); err != nil {
		return book{Exchange: "bitget"}, fetchDiag{"bitget", err.Error()}
	}
	if raw.Code != "00000" {
		return book{Exchange: "bitget"}, fetchDiag{"bitget", "non-00000"}
	}
	asks := make([]level, 0, len(raw.Data.Asks))
	for _, a := range raw.Data.Asks {
		if len(a) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(a[0], 64)
		q, _ := strconv.ParseFloat(a[1], 64)
		if p > 0 && q > 0 {
			asks = append(asks, level{Price: p, Qty: q})
		}
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	if len(asks) == 0 {
		return book{Exchange: "bitget"}, fetchDiag{"bitget", "200 but 0 asks"}
	}
	return book{Exchange: "bitget", Asks: asks}, fetchDiag{"bitget", "ok"}
}

// --- greedy: returns perEx, vwap, costUSDT ---
func greedyFillUSD(books []book, amountUSDT float64) (perEx map[string]float64, vwap float64, cost float64) {
	type cur struct {
		ex  string
		i   int
		lvl level
	}

	// min-heap by price
	h := make([]cur, 0, len(books))
	bookByEx := make(map[string]book, len(books))
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
			if !less(j, i) {
				break
			}
			swap(i, j)
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
	var got, spend float64
	usdCap := amountUSDT

	for spend < usdCap {
		x, ok := pop()
		if !ok {
			break
		}
		maxUSD := x.lvl.Price * x.lvl.Qty
		if spend+maxUSD <= usdCap {
			// берем весь уровень
			got += x.lvl.Qty
			spend += maxUSD
			cost += maxUSD
			perEx[x.ex] += x.lvl.Qty

			// подгружаем следующий уровень этой биржи
			if b, ok := bookByEx[x.ex]; ok {
				if x.i+1 < len(b.Asks) {
					push(cur{ex: x.ex, i: x.i + 1, lvl: b.Asks[x.i+1]})
				}
			}
		} else {
			// частичное исполнение на текущем уровне
			leftUSD := usdCap - spend
			qty := leftUSD / x.lvl.Price
			got += qty
			spend = usdCap
			cost += leftUSD
			perEx[x.ex] += qty
			break
		}
	}
	if got > 0 {
		vwap = cost / got
	}
	return perEx, vwap, cost
}

// --------------- main Plan ---------------
func (rf *RealFlow) Plan(ctx context.Context, req httpapi.PlanRequest) (httpapi.PlanResponse, error) {
	// Spend: integer USD
	amountUSDT := math.Floor(req.Amount + 1e-9)
	if amountUSDT < 1 {
		amountUSDT = 1
	}

	depth := req.Depth
	if depth <= 0 || depth > 500 {
		depth = 100
	}

	type res struct {
		b book
		d fetchDiag
	}
	ch := make(chan res, 7)

	go func() { b, d := rf.fetchBinance(ctx, req.Base, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchOKX(ctx, req.Base, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchBybit(ctx, req.Base, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchKucoin(ctx, req.Base, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchGate(ctx, req.Base, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchHTX(ctx, req.Base, depth); ch <- res{b, d} }()
	go func() { b, d := rf.fetchBitget(ctx, req.Base, depth); ch <- res{b, d} }()

	var books []book
	diags := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		r := <-ch
		diags = append(diags, fmt.Sprintf("%s: %s", r.d.Exchange, r.d.Status))
		if len(r.b.Asks) > 0 {
			books = append(books, r.b)
		}
	}
	if len(books) == 0 {
		return httpapi.PlanResponse{}, fmt.Errorf("no order books (%s)", strings.Join(diags, "; "))
	}

	const feeRate = 0.001 // 0.1% mock fees

	var legs []httpapi.PlanLeg
	var costUSDT, feesUSDT, vwap float64

	switch req.Scenario {
	case "best_single":
		var bestVWAP float64
		var bestCost, bestFee float64
		var bestLeg httpapi.PlanLeg
		for _, b := range books {
			pe, v, c := greedyFillUSD([]book{b}, amountUSDT)
			qty := pe[b.Exchange]
			if qty <= 0 {
				continue
			}
			fee := roundCents(c * feeRate)
			if bestVWAP == 0 || v < bestVWAP {
				bestVWAP = v
				bestCost = c
				bestFee = fee
				bestLeg = httpapi.PlanLeg{
					Exchange: b.Exchange,
					Amount:   qty,
					Price:    v, // средняя цена исполнения на этой бирже
					Fee:      fee,
				}
			}
		}
		if bestLeg.Exchange != "" {
			legs = append(legs, bestLeg)
		}
		costUSDT = roundCents(bestCost)
		feesUSDT = roundCents(bestFee)
		vwap = roundCents(bestVWAP)

	case "equal_split":
		part := amountUSDT / float64(len(books))
		var totalQty, totalCost float64
		for _, b := range books {
			pe, v, c := greedyFillUSD([]book{b}, part)
			qty := pe[b.Exchange]
			if qty <= 0 {
				continue
			}
			fee := roundCents(c * feeRate)
			legs = append(legs, httpapi.PlanLeg{
				Exchange: b.Exchange,
				Amount:   qty,
				Price:    v, // средняя цена исполнения доли
				Fee:      fee,
			})
			totalQty += qty
			totalCost += c
			feesUSDT += fee
		}
		costUSDT = roundCents(totalCost)
		if totalQty > 0 {
			vwap = roundCents(costUSDT / totalQty)
		}

	default: // optimal — общий greedy по всем книгам
		perEx, v, _ := greedyFillUSD(books, amountUSDT)
		vwap = roundCents(v)

		// Цена в legs — лучший ask для справки; комиссии считаем как раньше
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
			price := priceByEx[b.Exchange]
			usd := roundCents(qty * price)
			fee := roundCents(usd * feeRate)
			costUSDT += usd
			feesUSDT += fee
			legs = append(legs, httpapi.PlanLeg{
				Exchange: b.Exchange,
				Amount:   qty,
				Price:    price,
				Fee:      fee,
			})
		}
		costUSDT = roundCents(costUSDT)
		feesUSDT = roundCents(feesUSDT)
	}

	unspent := roundCents(amountUSDT - costUSDT)
	if unspent < 0 {
		unspent = 0
	}

	return httpapi.PlanResponse{
		Base:        req.Base,
		Quote:       req.Quote,
		Amount:      amountUSDT,
		Scenario:    req.Scenario,
		VWAP:        vwap,
		TotalCost:   costUSDT,
		TotalFees:   feesUSDT,
		Unspent:     unspent,
		Legs:        legs,
		GeneratedAt: time.Now().Format("15:04 02.01.2006"),
	}, nil
}
