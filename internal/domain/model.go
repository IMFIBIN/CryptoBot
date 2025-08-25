package domain

import "time"

// Базовые доменные сущности

type Order struct {
	Price    string
	Quantity string
}

type OrderBook struct {
	Symbol    string
	Exchange  string
	Timestamp int64
	Asks      []Order
	Bids      []Order
}

// Параметры запроса стаканов и задержек
type Config struct {
	DelayMS int `json:"delay_ms"`
	Limit   int `json:"limit"`
}

// Контракт адаптера биржи
type Exchange interface {
	Name() string
	GetSymbols() ([]string, error)
	GetOrderBook(symbol string, limit int) (*OrderBook, error)
	GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*OrderBook, error)
}
