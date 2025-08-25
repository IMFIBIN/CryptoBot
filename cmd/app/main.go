package main

import (
	"log"

	binanceadapter "cryptobot/internal/adapters/exchange/binance"
	bybitadapter "cryptobot/internal/adapters/exchange/bybit"
	"cryptobot/internal/domain"
	"cryptobot/internal/usecase"
)

func main() {
	cfg := domain.Config{DelayMS: 100, Limit: 100}

	exchanges := []domain.Exchange{
		binanceadapter.New(cfg),
		bybitadapter.New(cfg),
	}

	if err := usecase.Run(cfg, exchanges); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
