package exchangebooks

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

	"cryptobot/internal/usecase/planner"
)

// HTTPRepo реализует planner.Repo: тянет стаканы <COIN>/USDT с крупных бирж по HTTP.
type HTTPRepo struct {
	http *http.Client
}

func NewHTTPRepo() *HTTPRepo {
	return &HTTPRepo{http: &http.Client{Timeout: 8 * time.Second}}
}

// ====== Вспомогалки ======

func (r *HTTPRepo) doGET(ctx context.Context, url string, target any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "otccalc/httprepo")
	res, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("http %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func sortAsks(xs []planner.Level) {
	sort.Slice(xs, func(i, j int) bool { return xs[i].Price < xs[j].Price })
}
func sortBids(xs []planner.Level) {
	sort.Slice(xs, func(i, j int) bool { return xs[i].Price > xs[j].Price })
}

func clampPositive(xs []planner.Level) []planner.Level {
	out := xs[:0]
	for _, l := range xs {
		if l.Price > 0 && l.Qty > 0 && isFinite(l.Price) && isFinite(l.Qty) {
			out = append(out, l)
		}
	}
	return out
}

func isFinite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func lastOrNil[T any](xs []T) *T {
	if len(xs) == 0 {
		return nil
	}
	last := &xs[len(xs)-1]
	if last == nil {
		return nil
	}
	return last
}

// ====== Реализация Repo ======

func (r *HTTPRepo) FetchAllBooks(ctx context.Context, coin string, depth int) ([]planner.Book, []string, error) {
	type res struct {
		b planner.Book
		d string
	}
	ch := make(chan res, 7)

	go func() { b, d := r.fetchBinance(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := r.fetchOKX(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := r.fetchBybit(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := r.fetchKucoin(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := r.fetchGate(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := r.fetchHTX(ctx, coin, depth); ch <- res{b, d} }()
	go func() { b, d := r.fetchBitget(ctx, coin, depth); ch <- res{b, d} }()

	var books []planner.Book
	var diags []string
	for i := 0; i < 7; i++ {
		r := <-ch
		if (len(r.b.Asks) + len(r.b.Bids)) > 0 {
			books = append(books, r.b)
		}
		if r.d != "" {
			diags = append(diags, r.d)
		}
	}
	return books, diags, nil
}

// ====== Фетчеры бирж (<COIN>/USDT) ======

// BINANCE
func (r *HTTPRepo) fetchBinance(ctx context.Context, coin string, depth int) (planner.Book, string) {
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
	if err := r.doGET(ctx, url, &raw); err != nil {
		return planner.Book{Exchange: "binance"}, "binance:err:" + err.Error()
	}
	var asks, bids []planner.Level
	for _, it := range raw.Asks {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		asks = append(asks, planner.Level{Price: p, Qty: q})
	}
	for _, it := range raw.Bids {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		bids = append(bids, planner.Level{Price: p, Qty: q})
	}
	asks = clampPositive(asks)
	bids = clampPositive(bids)
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return planner.Book{Exchange: "binance"}, "binance:empty"
	}
	return planner.Book{Exchange: "binance", Asks: asks, Bids: bids}, "binance:ok"
}

// OKX
func (r *HTTPRepo) fetchOKX(ctx context.Context, coin string, depth int) (planner.Book, string) {
	d := depth
	if d <= 0 || d > 400 {
		d = 400
	}
	inst := strings.ToUpper(coin) + "-USDT"
	url := fmt.Sprintf("https://www.okx.com/api/v5/market/books?instId=%s&sz=%d", inst, d)
	var raw struct {
		Code string `json:"code"`
		Data []struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
		} `json:"data"`
	}
	if err := r.doGET(ctx, url, &raw); err != nil || raw.Code != "0" {
		return planner.Book{Exchange: "okx"}, "okx:err"
	}
	data := lastOrNil(raw.Data)
	if data == nil {
		return planner.Book{Exchange: "okx"}, "okx:empty"
	}
	var asks, bids []planner.Level
	for _, it := range data.Asks {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		asks = append(asks, planner.Level{Price: p, Qty: q})
	}
	for _, it := range data.Bids {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		bids = append(bids, planner.Level{Price: p, Qty: q})
	}
	asks = clampPositive(asks)
	bids = clampPositive(bids)
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return planner.Book{Exchange: "okx"}, "okx:empty"
	}
	return planner.Book{Exchange: "okx", Asks: asks, Bids: bids}, "okx:ok"
}

// BYBIT
func (r *HTTPRepo) fetchBybit(ctx context.Context, coin string, depth int) (planner.Book, string) {
	d := depth
	if d <= 0 || d > 200 {
		d = 200
	}
	symbol := strings.ToUpper(coin) + "USDT"
	url := fmt.Sprintf("https://api.bybit.com/v5/market/orderbook?category=spot&symbol=%s&limit=%d", symbol, d)
	var raw struct {
		Result struct {
			Asks [][]string `json:"a"`
			Bids [][]string `json:"b"`
		} `json:"result"`
	}
	if err := r.doGET(ctx, url, &raw); err != nil {
		return planner.Book{Exchange: "bybit"}, "bybit:err"
	}
	var asks, bids []planner.Level
	for _, it := range raw.Result.Asks {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		asks = append(asks, planner.Level{Price: p, Qty: q})
	}
	for _, it := range raw.Result.Bids {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		bids = append(bids, planner.Level{Price: p, Qty: q})
	}
	asks = clampPositive(asks)
	bids = clampPositive(bids)
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return planner.Book{Exchange: "bybit"}, "bybit:empty"
	}
	return planner.Book{Exchange: "bybit", Asks: asks, Bids: bids}, "bybit:ok"
}

// KUCOIN
func (r *HTTPRepo) fetchKucoin(ctx context.Context, coin string, depth int) (planner.Book, string) {
	d := depth
	if d <= 0 || d > 200 {
		d = 200
	}
	symbol := strings.ToUpper(coin) + "-USDT"
	url := fmt.Sprintf("https://api.kucoin.com/api/v1/market/orderbook/level2_100?symbol=%s", symbol)
	var raw struct {
		Code string `json:"code"`
		Data struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
		} `json:"data"`
	}
	if err := r.doGET(ctx, url, &raw); err != nil || raw.Code != "200000" {
		return planner.Book{Exchange: "kucoin"}, "kucoin:err"
	}
	var asks, bids []planner.Level
	for _, it := range raw.Data.Asks {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		asks = append(asks, planner.Level{Price: p, Qty: q})
	}
	for _, it := range raw.Data.Bids {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		bids = append(bids, planner.Level{Price: p, Qty: q})
	}
	asks = clampPositive(asks)
	bids = clampPositive(bids)
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return planner.Book{Exchange: "kucoin"}, "kucoin:empty"
	}
	return planner.Book{Exchange: "kucoin", Asks: asks, Bids: bids}, "kucoin:ok"
}

// GATE
func (r *HTTPRepo) fetchGate(ctx context.Context, coin string, depth int) (planner.Book, string) {
	d := depth
	if d <= 0 || d > 200 {
		d = 200
	}
	symbol := strings.ToUpper(coin) + "_USDT"
	url := fmt.Sprintf("https://api.gateio.ws/api/v4/spot/order_book?currency_pair=%s&limit=%d", symbol, d)
	var raw struct {
		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
	}
	if err := r.doGET(ctx, url, &raw); err != nil {
		return planner.Book{Exchange: "gate"}, "gate:err"
	}
	var asks, bids []planner.Level
	for _, it := range raw.Asks {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		asks = append(asks, planner.Level{Price: p, Qty: q})
	}
	for _, it := range raw.Bids {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		bids = append(bids, planner.Level{Price: p, Qty: q})
	}
	asks = clampPositive(asks)
	bids = clampPositive(bids)
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return planner.Book{Exchange: "gate"}, "gate:empty"
	}
	return planner.Book{Exchange: "gate", Asks: asks, Bids: bids}, "gate:ok"
}

// HTX (HUOBI)
func (r *HTTPRepo) fetchHTX(ctx context.Context, coin string, depth int) (planner.Book, string) {
	d := depth
	if d <= 0 || d > 200 {
		d = 200
	}
	symbol := strings.ToLower(coin) + "usdt"
	url := fmt.Sprintf("https://api.huobi.pro/market/depth?symbol=%s&type=step0", symbol)
	var raw struct {
		Tick struct {
			Asks [][]float64 `json:"asks"`
			Bids [][]float64 `json:"bids"`
		} `json:"tick"`
	}
	if err := r.doGET(ctx, url, &raw); err != nil {
		return planner.Book{Exchange: "htx"}, "htx:err"
	}
	var asks, bids []planner.Level
	for _, it := range raw.Tick.Asks {
		if len(it) < 2 {
			continue
		}
		asks = append(asks, planner.Level{Price: it[0], Qty: it[1]})
	}
	for _, it := range raw.Tick.Bids {
		if len(it) < 2 {
			continue
		}
		bids = append(bids, planner.Level{Price: it[0], Qty: it[1]})
	}
	asks = clampPositive(asks)
	bids = clampPositive(bids)
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return planner.Book{Exchange: "htx"}, "htx:empty"
	}
	return planner.Book{Exchange: "htx", Asks: asks, Bids: bids}, "htx:ok"
}

// BITGET
func (r *HTTPRepo) fetchBitget(ctx context.Context, coin string, depth int) (planner.Book, string) {
	d := depth
	if d <= 0 || d > 200 {
		d = 200
	}
	symbol := strings.ToUpper(coin) + "USDT"
	url := fmt.Sprintf("https://api.bitget.com/api/spot/v1/market/depth?symbol=%s&type=step0&limit=%d", symbol, d)
	var raw struct {
		Data struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
		} `json:"data"`
	}
	if err := r.doGET(ctx, url, &raw); err != nil {
		return planner.Book{Exchange: "bitget"}, "bitget:err"
	}
	var asks, bids []planner.Level
	for _, it := range raw.Data.Asks {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		asks = append(asks, planner.Level{Price: p, Qty: q})
	}
	for _, it := range raw.Data.Bids {
		if len(it) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(it[0], 64)
		q, _ := strconv.ParseFloat(it[1], 64)
		bids = append(bids, planner.Level{Price: p, Qty: q})
	}
	asks = clampPositive(asks)
	bids = clampPositive(bids)
	sortAsks(asks)
	sortBids(bids)
	if len(asks) == 0 && len(bids) == 0 {
		return planner.Book{Exchange: "bitget"}, "bitget:empty"
	}
	return planner.Book{Exchange: "bitget", Asks: asks, Bids: bids}, "bitget:ok"
}
