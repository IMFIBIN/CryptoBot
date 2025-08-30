package okxadapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/shared/retry"
)

// ВНИМАНИЕ: во всём проекте мы используем унифицированный тикер без дефиса, например "BTCUSDT".
// У OKX формат другой — "BTC-USDT". В адаптере делаем конверсию туда/обратно.

func toOKXSymbol(unified string) string {
	// "BTCUSDT" -> "BTC-USDT" (и вообще <BASE><QUOTE> -> <BASE>-<QUOTE>)
	if len(unified) > 5 && strings.HasSuffix(unified, "USDT") {
		return unified[:len(unified)-4] + "-USDT"
	}
	// универсальная попытка: разделим по последним 4 символам
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
		baseURL: "https://www.okx.com",
		client:  &http.Client{Timeout: 7 * time.Second},
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

type okxExchange struct {
	http   *httpClient
	config domain.Config
}

func New(config domain.Config) domain.Exchange {
	return &okxExchange{
		http:   newHTTPClient(),
		config: config,
	}
}

func (o *okxExchange) Name() string { return "OKX" }

// ===== /public/instruments (необязательно, но оставим для симметрии) =====

type instrumentsResp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		InstID string `json:"instId"` // "BTC-USDT"
		State  string `json:"state"`  // "live"
	} `json:"data"`
}

func (o *okxExchange) GetSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/api/v5/public/instruments?instType=SPOT", o.http.baseURL)
	data, err := o.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("okx: ошибка запроса: %w", err)
	}
	var resp instrumentsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("okx: ошибка парсинга JSON: %w", err)
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx: API error: %s", resp.Msg)
	}
	var out []string
	for _, it := range resp.Data {
		if it.State == "live" && strings.HasSuffix(it.InstID, "-USDT") {
			// вернём в унифицированном виде без дефиса
			out = append(out, strings.ReplaceAll(it.InstID, "-", ""))
		}
	}
	return out, nil
}

// ===== /market/books =====

type orderbookResp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Asks [][]string `json:"asks"` // [[price, size, ...], ...]
		Bids [][]string `json:"bids"`
		Ts   string     `json:"ts"` // millis in string
	} `json:"data"`
}

func (o *okxExchange) GetOrderBook(symbol string, limit int) (*domain.OrderBook, error) {
	instID := toOKXSymbol(symbol)
	url := fmt.Sprintf("%s/api/v5/market/books?instId=%s&sz=%d", o.http.baseURL, instID, limit)
	data, err := o.http.get(url)
	if err != nil {
		return nil, fmt.Errorf("okx: ошибка запроса стакана: %w", err)
	}
	var resp orderbookResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("okx: ошибка парсинга стакана: %w", err)
	}
	if resp.Code != "0" || len(resp.Data) == 0 {
		return nil, fmt.Errorf("okx: API error: %s", resp.Msg)
	}

	// timestamp
	ts := time.Now().UnixMilli()
	if ms, err := strconv.ParseInt(resp.Data[0].Ts, 10, 64); err == nil {
		ts = ms
	}

	ob := &domain.OrderBook{
		// ВАЖНО: возвращаем символ в унифицированном виде, чтобы остальной код сопоставлял биржи корректно.
		Symbol:    symbol, // "BTCUSDT" и т.п.
		Exchange:  o.Name(),
		Timestamp: ts,
	}
	for _, a := range resp.Data[0].Asks {
		if len(a) >= 2 {
			ob.Asks = append(ob.Asks, domain.Order{Price: a[0], Quantity: a[1]})
		}
	}
	for _, b := range resp.Data[0].Bids {
		if len(b) >= 2 {
			ob.Bids = append(ob.Bids, domain.Order{Price: b[0], Quantity: b[1]})
		}
	}
	return ob, nil
}

func (o *okxExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*domain.OrderBook, error) {
	res := make(map[string]*domain.OrderBook)
	var lastErr error
	for _, s := range symbols {
		ob, err := o.GetOrderBook(s, limit)
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
