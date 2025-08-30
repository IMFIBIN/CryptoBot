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
	"cryptobot/internal/transport/cli"
	"cryptobot/internal/usecase"
	"cryptobot/internal/usecase/scenario"
)

func main() {
	cfg := domain.Config{
		Limit:   100,
		DelayMS: 100,
	}

	// Комиссии/минимумы для печати и валидации (при необходимости подстрой под реальные)
	fees := map[string]scenario.FeeConfig{
		"Binance": {FeePct: 0.001, MinQty: 0, MinNotional: 10},
		"Bybit":   {FeePct: 0.001, MinQty: 0, MinNotional: 10},
		"OKX":     {FeePct: 0.001, MinQty: 0, MinNotional: 10},
		"KuCoin":  {FeePct: 0.001, MinQty: 0, MinNotional: 10},
		"Bitget":  {FeePct: 0.001, MinQty: 0, MinNotional: 10},
		"HTX":     {FeePct: 0.001, MinQty: 0, MinNotional: 10},
		"Gate":    {FeePct: 0.001, MinQty: 0, MinNotional: 10},
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

	// CLI-презентер с комиссиями
	pr := cli.NewCLIPresenterWithFees(fees)

	// Стратегии
	strategies := []scenario.Strategy{
		scenario.BestSingle{},
		scenario.EqualSplit{},
		scenario.Optimal{},
	}

	// Запуск
	if err := usecase.RunWithStrategies(cfg, exchanges, pr, strategies); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Ошибка выполнения: %v\n", err)
		os.Exit(1)
	}
}
