package bybitadapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/shared/retry"
)

type httpClient struct {
	baseURL string
	client  *http.Client
}

func newHTTPClient() *httpClient {
	return &httpClient{
		baseURL: "https://api.bybit.com",
		client:  &http.Client{Timeout: 7 * time.Second}, // мягкий таймаут
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

type bybitExchange struct {
	http   *httpClient
	config domain.Config
}

func New(config domain.Config) domain.Exchange {
	return &bybitExchange{
		http:   newHTTPClient(),
		config: config,
	}
}

func (b *bybitExchange) Name() string { return "Bybit" }

type instrumentsResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []struct {
			Symbol string `json:"symbol"`
			Status string `json:"status"`
		} `json:"list"`
	} `json:"result"`
}

func (b *bybitExchange) GetSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/v5/market/instruments-info?category=spot", b.http.baseURL)
	data, err := b.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("bybit: ошибка запроса: %w", err)
	}
	var resp instrumentsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("bybit: ошибка парсинга JSON: %w", err)
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit: API error: %s", resp.RetMsg)
	}
	var out []string
	for _, it := range resp.Result.List {
		if it.Status == "Trading" {
			out = append(out, it.Symbol)
		}
	}
	return out, nil
}

type orderbookResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Symbol string     `json:"s"`
		Bids   [][]string `json:"b"`
		Asks   [][]string `json:"a"`
		Ts     int64      `json:"ts"`
	} `json:"result"`
}

func clampBybitLimit(limit int) int {
	allowed := []int{1, 3, 5, 10, 20, 50, 100}
	chosen := allowed[len(allowed)-1]
	for _, v := range allowed {
		if limit <= v {
			chosen = v
			break
		}
	}
	return chosen
}

func (b *bybitExchange) GetOrderBook(symbol string, limit int) (*domain.OrderBook, error) {
	chosen := clampBybitLimit(limit)
	url := fmt.Sprintf("%s/v5/market/orderbook?category=spot&symbol=%s&limit=%d",
		b.http.baseURL, symbol, chosen)
	data, err := b.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("bybit: ошибка запроса стакана: %w", err)
	}
	var resp orderbookResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("bybit: ошибка парсинга стакана: %w", err)
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit: API error: %s", resp.RetMsg)
	}
	ob := &domain.OrderBook{
		Symbol:    symbol,
		Exchange:  b.Name(),
		Timestamp: resp.Result.Ts,
	}
	for _, a := range resp.Result.Asks {
		if len(a) >= 2 {
			ob.Asks = append(ob.Asks, domain.Order{Price: a[0], Quantity: a[1]})
		}
	}
	for _, d := range resp.Result.Bids {
		if len(d) >= 2 {
			ob.Bids = append(ob.Bids, domain.Order{Price: d[0], Quantity: d[1]})
		}
	}
	return ob, nil
}

func (b *bybitExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*domain.OrderBook, error) {
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
