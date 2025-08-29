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

	// Комиссии/минимумы — используем в презентере для печати комиссии.
	fees := map[string]scenario.FeeConfig{
		"Binance": {FeePct: 0.001, MinQty: 0, MinNotional: 10},
		"Bybit":   {FeePct: 0.001, MinQty: 0, MinNotional: 10},
	}

	// Если в адаптерах New теперь возвращает domain.Exchange — всё ок.
	exchanges := []domain.Exchange{
		binanceadapter.New(cfg),
		bybitadapter.New(cfg),
	}

	// Презентер с комиссиями.
	pr := cli.NewCLIPresenterWithFees(fees)

	strategies := []scenario.Strategy{
		scenario.BestSingle{},
		scenario.EqualSplit{},
		scenario.Optimal{},
	}

	// Печатаем ошибку и выходим. Явно игнорируем ошибку записи в Stderr.
	if err := usecase.RunWithStrategies(cfg, exchanges, pr, strategies); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Ошибка выполнения: %v\n", err)
		os.Exit(1)
	}
}
