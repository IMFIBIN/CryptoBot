package htxadapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/shared/retry"
)

// HTX (Huobi) использует "btcusdt" (lowercase, без разделителей).
// В адаптере переводим из "BTCUSDT" -> "btcusdt".
func toHTXSymbol(unified string) string {
	return strings.ToLower(unified)
}

type httpClient struct {
	baseURL string
	client  *http.Client
}

func newHTTPClient() *httpClient {
	return &httpClient{
		baseURL: "https://api.huobi.pro",
		client:  &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *httpClient) get(url string) ([]byte, error) {
	var body []byte
	err := retry.WithRetry(2, 400*time.Millisecond, func() error {
		resp, err := c.client.Get(url)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %s", resp.Status)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		body = b
		return nil
	})
	return body, err
}

type htxExchange struct {
	http   *httpClient
	config domain.Config
}

func New(cfg domain.Config) domain.Exchange {
	return &htxExchange{
		http:   newHTTPClient(),
		config: cfg,
	}
}

func (h *htxExchange) Name() string { return "HTX" }

// ===== symbols =====
type symbolsResp struct {
	Status string `json:"status"`
	Data   []struct {
		Symbol string `json:"symbol"` // "btcusdt"
		State  string `json:"state"`  // "online"
		Quote  string `json:"quote-currency"`
	} `json:"data"`
}

func (h *htxExchange) GetSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/v1/common/symbols", h.http.baseURL)
	data, err := h.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("htx: symbols: %w", err)
	}
	var resp symbolsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("htx: parse symbols: %w", err)
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("htx: API status=%s", resp.Status)
	}
	var out []string
	for _, it := range resp.Data {
		if it.State == "online" && it.Quote == "usdt" {
			out = append(out, strings.ToUpper(it.Symbol)) // -> "BTCUSDT"
		}
	}
	return out, nil
}

// ===== order book =====
type depthResp struct {
	Status string `json:"status"`
	Ts     int64  `json:"ts"`
	Tick   struct {
		Bids [][]float64 `json:"bids"` // [[price, amount], ...]
		Asks [][]float64 `json:"asks"`
	} `json:"tick"`
}

func (h *htxExchange) GetOrderBook(symbol string, limit int) (*domain.OrderBook, error) {
	s := toHTXSymbol(symbol)
	// type=step0 — наименьшая агрегация. huobi не принимает limit — ограничим вручную.
	url := fmt.Sprintf("%s/market/depth?symbol=%s&type=step0", h.http.baseURL, s)
	data, err := h.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("htx: depth: %w", err)
	}
	var resp depthResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("htx: parse depth: %w", err)
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("htx: API status=%s", resp.Status)
	}
	ob := &domain.OrderBook{
		Symbol:    symbol,
		Exchange:  h.Name(),
		Timestamp: resp.Ts,
	}
	appendLevels := func(dst *[]domain.Order, src [][]float64) {
		n := limit
		if n <= 0 || n > len(src) {
			n = len(src)
		}
		for i := 0; i < n; i++ {
			r := src[i]
			if len(r) >= 2 {
				*dst = append(*dst, domain.Order{
					Price:    fmt.Sprintf("%.8f", r[0]),
					Quantity: fmt.Sprintf("%.8f", r[1]),
				})
			}
		}
	}
	appendLevels(&ob.Asks, resp.Tick.Asks)
	appendLevels(&ob.Bids, resp.Tick.Bids)
	return ob, nil
}

func (h *htxExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*domain.OrderBook, error) {
	res := make(map[string]*domain.OrderBook)
	var lastErr error
	for _, s := range symbols {
		ob, err := h.GetOrderBook(s, limit)
		if err != nil {
			lastErr = err
			if delay > 0 {
				time.Sleep(delay)
			}
			continue
		}
		res[s] = ob
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	if len(res) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return res, nil
}
