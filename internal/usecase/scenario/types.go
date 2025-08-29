package scenario

import (
	"time"

	"cryptobot/internal/domain"
)

type Direction string

const (
	Buy  Direction = "buy"
	Sell Direction = "sell"
)

type FeeConfig struct {
	FeePct      float64
	MinQty      float64
	MinNotional float64
}

type ExchangeEval struct {
	Exchange   string
	AvgPrice   float64
	Qty        float64 // монет куплено/продано
	AmountUSDT float64 // сумма в USDT (BUY: потрачено, SELL: получено)
	Commission float64 // комиссия в USDT
	Coverage   float64 // %
}
type Inputs struct {
	Direction  Direction
	Symbol     string
	Right      string
	Amount     float64
	OrderBooks map[string]*domain.OrderBook
	Fees       map[string]FeeConfig
	Now        time.Time
	MaxStale   time.Duration
}
type Leg struct {
	Exchange   string
	Price      float64 // средняя цена по ножке
	Qty        float64 // монет в этой ножке
	AmountUSDT float64 // сумма в USDT по этой ножке (BUY: потрачено, SELL: получено)
}

type Result struct {
	Legs         []Leg
	TotalQty     float64 // суммарное кол-во монет (BUY/SELL: исходный объём)
	TotalUSDT    float64 // суммарный USDT (BUY: потрачено, SELL: получено)
	AveragePrice float64 // усреднённая цена за 1 монету
	Leftover     float64 // неиспользованный USDT (BUY) или неисполненный объём монеты (SELL)
	Asset        string  // тикер монеты (SOL/ETH/…)
}

type Strategy interface {
	Name() string
	Run(in Inputs) Result
}
