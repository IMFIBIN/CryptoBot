package binanceadapter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/shared/retry"

	gbinance "github.com/adshao/go-binance/v2"
)

type BinanceExchange struct {
	client *gbinance.Client
	config domain.Config
}

func New(config domain.Config) *BinanceExchange {
	client := gbinance.NewClient("", "")
	// Чуть мягче таймаут: не висим долго, но и не рвём слишком быстро
	client.HTTPClient = &http.Client{Timeout: 7 * time.Second}
	return &BinanceExchange{client: client, config: config}
}

func (b *BinanceExchange) Name() string { return "Binance" }

func (b *BinanceExchange) GetSymbols() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exInfo, err := b.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance: ошибка получения информации: %w", err)
	}
	var symbols []string
	for _, s := range exInfo.Symbols {
		if s.Status == "TRADING" {
			symbols = append(symbols, s.Symbol)
		}
	}
	return symbols, nil
}

func (b *BinanceExchange) GetOrderBook(symbol string, limit int) (*domain.OrderBook, error) {
	// Поддерживаемые лимиты Binance
	allowed := []int{5, 10, 20, 50, 100}
	chosen := allowed[len(allowed)-1]
	for _, v := range allowed {
		if limit <= v {
			chosen = v
			break
		}
	}

	var depth *gbinance.DepthResponse
	// 2 попытки по 5s — компромисс между скоростью и стабильностью
	err := retry.WithRetry(2, 500*time.Millisecond, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		depth, err = b.client.NewDepthService().Symbol(symbol).Limit(chosen).Do(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("binance: стакан %s (limit=%d): %w", symbol, chosen, err)
	}

	ob := &domain.OrderBook{
		Symbol:    symbol,
		Exchange:  b.Name(),
		Timestamp: time.Now().UnixMilli(),
	}
	for _, a := range depth.Asks {
		ob.Asks = append(ob.Asks, domain.Order{Price: a.Price, Quantity: a.Quantity})
	}
	for _, d := range depth.Bids {
		ob.Bids = append(ob.Bids, domain.Order{Price: d.Price, Quantity: d.Quantity})
	}
	return ob, nil
}

func (b *BinanceExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*domain.OrderBook, error) {
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
