package main

import (
	"fmt"
	"os"

	binanceadapter "cryptobot/internal/adapters/exchange/binance"
	bybitadapter "cryptobot/internal/adapters/exchange/bybit"
	"cryptobot/internal/domain"
	"cryptobot/internal/transport/cli"
	"cryptobot/internal/usecase"
	"cryptobot/internal/usecase/scenario"
)

func main() {
	cfg := domain.Config{
		Limit:   100,
		DelayMS: 100,
	}

	// 1) Комиссии/минимумы – передадим их в презентер,
	//    чтобы он корректно показывал "комиссия: ... USDT".
	fees := map[string]scenario.FeeConfig{
		"Binance": {FeePct: 0.001, MinQty: 0, MinNotional: 10}, // 0.10%
		"Bybit":   {FeePct: 0.001, MinQty: 0, MinNotional: 10}, // 0.10%
	}

	exchanges := []domain.Exchange{
		binanceadapter.New(cfg),
		bybitadapter.New(cfg),
	}

	// 2) Презентер с комиссиями (нужен для вывода "комиссия: ... USDT")
	pr := cli.NewCLIPresenterWithFees(fees)

	strategies := []scenario.Strategy{
		scenario.BestSingle{},
		scenario.EqualSplit{},
		scenario.Optimal{},
	}

	// 3) Не глотаем ошибку — напечатаем и выйдем с кодом 1.
	if err := usecase.RunWithStrategies(cfg, exchanges, pr, strategies); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка выполнения: %v\n", err)
		os.Exit(1)
	}
}
