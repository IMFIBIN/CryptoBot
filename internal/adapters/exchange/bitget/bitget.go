package bitgetadapter

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

// Bitget spot возвращает символы "BTCUSDT" (без суффиксов) в products.
// API: https://api.bitget.com

type httpClient struct {
	baseURL string
	client  *http.Client
}

func newHTTPClient() *httpClient {
	return &httpClient{
		baseURL: "https://api.bitget.com",
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

type bitgetExchange struct {
	http   *httpClient
	config domain.Config
}

func New(cfg domain.Config) domain.Exchange {
	return &bitgetExchange{
		http: newHTTPClient(),
	}
}

func (b *bitgetExchange) Name() string { return "Bitget" }

// ===== symbols =====
type productsResp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Symbol string `json:"symbol"` // "BTCUSDT"
		Status string `json:"status"` // "online"
		Quote  string `json:"quoteCoin"`
	} `json:"data"`
}

func (b *bitgetExchange) GetSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/api/spot/v1/public/products", b.http.baseURL)
	data, err := b.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("bitget: products: %w", err)
	}
	var resp productsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("bitget: parse products: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("bitget: API error: %s", resp.Msg)
	}
	var out []string
	for _, it := range resp.Data {
		if strings.EqualFold(it.Status, "online") && it.Quote == "USDT" && strings.HasSuffix(it.Symbol, "USDT") {
			out = append(out, it.Symbol) // уже "BTCUSDT"
		}
	}
	return out, nil
}

// ===== order book =====
type depthResp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Ts   string     `json:"ts"`
		Asks [][]string `json:"asks"` // [[price, size]]
		Bids [][]string `json:"bids"`
	} `json:"data"`
}

func (b *bitgetExchange) GetOrderBook(symbol string, limit int) (*domain.OrderBook, error) {
	if limit <= 0 || limit > 100 { // поддерживаемые: 5,10,15,20,50,100
		limit = 100
	}
	url := fmt.Sprintf("%s/api/spot/v1/market/depth?symbol=%s&limit=%d", b.http.baseURL, symbol, limit)
	data, err := b.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("bitget: depth: %w", err)
	}
	var resp depthResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("bitget: parse depth: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("bitget: API error: %s", resp.Msg)
	}
	ob := &domain.OrderBook{
		Symbol:    symbol,
		Exchange:  b.Name(),
		Timestamp: time.Now().UnixMilli(),
	}
	for _, a := range resp.Data.Asks {
		if len(a) >= 2 {
			ob.Asks = append(ob.Asks, domain.Order{Price: a[0], Quantity: a[1]})
		}
	}
	for _, d := range resp.Data.Bids {
		if len(d) >= 2 {
			ob.Bids = append(ob.Bids, domain.Order{Price: d[0], Quantity: d[1]})
		}
	}
	return ob, nil
}

func (b *bitgetExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*domain.OrderBook, error) {
	res := make(map[string]*domain.OrderBook)
	var lastErr error
	for _, s := range symbols {
		ob, err := b.GetOrderBook(s, limit)
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
