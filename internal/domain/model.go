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

type Config struct {
	DelayMS int `json:"delay_ms"`
	Limit   int `json:"limit"`
}

type Exchange interface {
	Name() string
	GetSymbols() ([]string, error)
	GetOrderBook(symbol string, limit int) (*OrderBook, error)
	GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*OrderBook, error)
}
