package kucoinadapter

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

// KuCoin использует формат "BTC-USDT".
// В проекте — унифицированный "BTCUSDT".
func toKuCoinSymbol(unified string) string {
	if len(unified) > 5 && strings.HasSuffix(unified, "USDT") {
		return unified[:len(unified)-4] + "-USDT"
	}
	if len(unified) > 4 {
		return unified[:len(unified)-4] + "-" + unified[len(unified)-4:]
	}
	return unified
}

type httpClient struct {
	baseURL string
	client  *http.Client
}

func newHTTPClient() *httpClient {
	return &httpClient{
		baseURL: "https://api.kucoin.com",
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

type kucoinExchange struct {
	http   *httpClient
	config domain.Config
}

func New(cfg domain.Config) domain.Exchange {
	return &kucoinExchange{
		http:   newHTTPClient(),
		config: cfg,
	}
}

func (k *kucoinExchange) Name() string { return "KuCoin" }

// ====== symbols ======
type symbolsResp struct {
	Code string `json:"code"`
	Data []struct {
		Symbol        string `json:"symbol"`        // "BTC-USDT"
		EnableTrading bool   `json:"enableTrading"` // true
		QuoteCurrency string `json:"quoteCurrency"` // "USDT"
	} `json:"data"`
}

func (k *kucoinExchange) GetSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/symbols", k.http.baseURL)
	data, err := k.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("kucoin: symbols request: %w", err)
	}
	var resp symbolsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("kucoin: parse symbols: %w", err)
	}
	if resp.Code != "200000" {
		return nil, fmt.Errorf("kucoin: API error code=%s", resp.Code)
	}
	var out []string
	for _, s := range resp.Data {
		if s.EnableTrading && s.QuoteCurrency == "USDT" && strings.HasSuffix(s.Symbol, "-USDT") {
			out = append(out, strings.ReplaceAll(s.Symbol, "-", "")) // -> "BTCUSDT"
		}
	}
	return out, nil
}

// ====== order book ======
type bookResp struct {
	Code string `json:"code"`
	Data struct {
		Time int64      `json:"time"`
		Asks [][]string `json:"asks"` // [price, size, ...]
		Bids [][]string `json:"bids"`
	} `json:"data"`
}

func (k *kucoinExchange) GetOrderBook(symbol string, limit int) (*domain.OrderBook, error) {
	inst := toKuCoinSymbol(symbol)
	// level2_100 — 100 уровней
	url := fmt.Sprintf("%s/api/v1/market/orderbook/level2_%d?symbol=%s", k.http.baseURL, 100, inst)
	data, err := k.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("kucoin: orderbook: %w", err)
	}
	var resp bookResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("kucoin: parse orderbook: %w", err)
	}
	if resp.Code != "200000" {
		return nil, fmt.Errorf("kucoin: API error code=%s", resp.Code)
	}
	ob := &domain.OrderBook{
		Symbol:    symbol, // унифицированный
		Exchange:  k.Name(),
		Timestamp: time.Now().UnixMilli(),
	}
	// Ограничим до limit вручную
	appendLevels := func(dst *[]domain.Order, src [][]string) {
		n := limit
		if n <= 0 || n > len(src) {
			n = len(src)
		}
		for i := 0; i < n; i++ {
			r := src[i]
			if len(r) >= 2 {
				*dst = append(*dst, domain.Order{Price: r[0], Quantity: r[1]})
			}
		}
	}
	appendLevels(&ob.Asks, resp.Data.Asks)
	appendLevels(&ob.Bids, resp.Data.Bids)
	return ob, nil
}

func (k *kucoinExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*domain.OrderBook, error) {
	res := make(map[string]*domain.OrderBook)
	var lastErr error
	for _, s := range symbols {
		ob, err := k.GetOrderBook(s, limit)
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
