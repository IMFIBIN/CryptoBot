package scenario

import (
	"time"

	"cryptobot/internal/domain"
)

type ExchangeEval struct {
	Exchange   string
	AvgPrice   float64
	Qty        float64
	AmountUSDT float64
	Coverage   float64
}

type Leg struct {
	Exchange   string
	Price      float64
	Qty        float64
	AmountUSDT float64
}

type Result struct {
	Legs         []Leg
	TotalQty     float64
	TotalUSDT    float64
	AveragePrice float64
	Leftover     float64
	Asset        string
}

type Direction int

const (
	Buy Direction = iota + 1
	Sell
)

type Inputs struct {
	Direction  Direction
	Symbol     string
	Right      string
	Amount     float64
	OrderBooks map[string]*domain.OrderBook
	Now        time.Time
	MaxStale   time.Duration
}
