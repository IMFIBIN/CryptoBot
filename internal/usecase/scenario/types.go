package scenario

import (
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/usecase/fees"
)

type ExchangeEval struct {
	Exchange   string
	AvgPrice   float64
	Qty        float64
	AmountUSDT float64
	Commission float64
	Coverage   float64 // %
}

type Leg struct {
	Exchange   string
	Price      float64
	Qty        float64
	AmountUSDT float64 // BUY: потрачено; SELL: получено (после комиссии)
	FeeUSDT    float64 // комиссия по этой "ножке"
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
	Fees       map[string]fees.Fee // <<<<< ИНТЕРФЕЙС КОМИССИЙ
	Now        time.Time
	MaxStale   time.Duration
}
