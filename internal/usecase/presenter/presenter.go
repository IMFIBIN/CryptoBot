package presenter

import (
	"cryptobot/internal/domain"
	"cryptobot/internal/usecase/scenario"
)

type Presenter interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)

	ShowOrderBookSummary(ob *domain.OrderBook)
	ShowCrossExchangeLine(symbol, exchange, ask, bid string)

	ShowScenarioHeader(name string, dir scenario.Direction)
	ShowBuyTotals(name, right string, res scenario.Result, amount float64)
	ShowSellTotals(name, right string, res scenario.Result)

	ShowBuyComparison(bestName, right string, rows []string)
	ShowSellComparison(bestName, right string, rows []string)

	ShowScenario1Rationale(asset string, dir scenario.Direction, evals []scenario.ExchangeEval)
}
