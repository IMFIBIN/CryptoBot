package main

import (
	"fmt"
	"os"

	binanceadapter "cryptobot/internal/adapters/exchange/binance"
	bitgetadapter "cryptobot/internal/adapters/exchange/bitget"
	bybitadapter "cryptobot/internal/adapters/exchange/bybit"
	gateadapter "cryptobot/internal/adapters/exchange/gate"
	htxadapter "cryptobot/internal/adapters/exchange/htx"
	kucoinadapter "cryptobot/internal/adapters/exchange/kucoin"
	okxadapter "cryptobot/internal/adapters/exchange/okx"

	"cryptobot/internal/domain"
	"cryptobot/internal/usecase"
)

func main() {
	cfg := domain.Config{
		Limit:   100,
		DelayMS: 100,
	}

	exchanges := []domain.Exchange{
		binanceadapter.New(cfg),
		bybitadapter.New(cfg),
		okxadapter.New(cfg),
		kucoinadapter.New(cfg),
		bitgetadapter.New(cfg),
		htxadapter.New(cfg),
		gateadapter.New(cfg),
	}

	if err := usecase.Run(cfg, exchanges); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Ошибка выполнения: %v\n", err)
		os.Exit(1)
	}
}
