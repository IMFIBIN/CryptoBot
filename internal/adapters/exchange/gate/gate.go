package gateadapter

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

// Gate.io использует "BTC_USDT".
// Конвертируем из/в унифицированный "BTCUSDT".
func toGateSymbol(unified string) string {
	if len(unified) > 5 && strings.HasSuffix(unified, "USDT") {
		return unified[:len(unified)-4] + "_USDT"
	}
	if len(unified) > 4 {
		return unified[:len(unified)-4] + "_" + unified[len(unified)-4:]
	}
	return unified
}

type httpClient struct {
	baseURL string
	client  *http.Client
}

func newHTTPClient() *httpClient {
	return &httpClient{
		baseURL: "https://api.gateio.ws",
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

type gateExchange struct {
	http   *httpClient
	config domain.Config
}

func New(cfg domain.Config) domain.Exchange {
	return &gateExchange{
		http:   newHTTPClient(),
		config: cfg,
	}
}

func (g *gateExchange) Name() string { return "Gate" }

// ===== symbols =====
type pairsResp []struct {
	ID          string `json:"id"`           // "BTC_USDT"
	TradeStatus string `json:"trade_status"` // "tradable"
}

func (g *gateExchange) GetSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/api/v4/spot/currency_pairs", g.http.baseURL)
	data, err := g.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("gate: currency_pairs: %w", err)
	}
	var resp pairsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("gate: parse pairs: %w", err)
	}
	var out []string
	for _, it := range resp {
		if it.TradeStatus == "tradable" && strings.HasSuffix(it.ID, "_USDT") {
			out = append(out, strings.ReplaceAll(it.ID, "_", "")) // -> "BTCUSDT"
		}
	}
	return out, nil
}

// ===== order book =====
type bookResp struct {
	Asks [][]string `json:"asks"` // [[price, amount], ...]
	Bids [][]string `json:"bids"`
}

func (g *gateExchange) GetOrderBook(symbol string, limit int) (*domain.OrderBook, error) {
	cp := toGateSymbol(symbol)
	url := fmt.Sprintf("%s/api/v4/spot/order_book?currency_pair=%s&limit=%d", g.http.baseURL, cp, limit)
	data, err := g.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("gate: order_book: %w", err)
	}
	var resp bookResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("gate: parse order_book: %w", err)
	}
	ob := &domain.OrderBook{
		Symbol:    symbol,
		Exchange:  g.Name(),
		Timestamp: time.Now().UnixMilli(),
	}
	for _, a := range resp.Asks {
		if len(a) >= 2 {
			ob.Asks = append(ob.Asks, domain.Order{Price: a[0], Quantity: a[1]})
		}
	}
	for _, b := range resp.Bids {
		if len(b) >= 2 {
			ob.Bids = append(ob.Bids, domain.Order{Price: b[0], Quantity: b[1]})
		}
	}
	return ob, nil
}

func (g *gateExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*domain.OrderBook, error) {
	res := make(map[string]*domain.OrderBook)
	var lastErr error
	for _, s := range symbols {
		ob, err := g.GetOrderBook(s, limit)
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
