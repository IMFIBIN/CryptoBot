package usecase

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/transport/cli"
	"cryptobot/internal/usecase/orderbook"
	"cryptobot/internal/usecase/presenter"
	"cryptobot/internal/usecase/scenario"
)

type snap struct {
	name string
	res  scenario.Result
}

func RunWithStrategies(
	cfg domain.Config,
	exchanges []domain.Exchange,
	pr presenter.Presenter,
	strategies []scenario.Strategy,
) error {
	return runCore(cfg, exchanges, pr, strategies)
}

type fetchRes struct {
	name string
	obs  map[string]*domain.OrderBook
	err  error
	dur  time.Duration
}

func runCore(
	cfg domain.Config,
	exchanges []domain.Exchange,
	pr presenter.Presenter,
	strategies []scenario.Strategy,
) error {
	params := cli.GetInteractiveParams()

	left := strings.ToUpper(params.LeftCoinName)
	right := strings.ToUpper(params.RightCoinName)

	dir := scenario.Buy
	if params.Action == "sell" {
		dir = scenario.Sell
	}

	var symbol string
	if dir == scenario.Buy {
		symbol = right + "USDT"
	} else {
		symbol = left + "USDT"
	}
	symbols := []string{symbol}

	exNames := make([]string, 0, len(exchanges))
	for _, ex := range exchanges {
		exNames = append(exNames, ex.Name())
	}
	pr.Infof("=== Крипто-биржи Монитор ===\n")
	pr.Infof("Доступные биржи: %v\n", exNames)

	now := time.Now()
	const maxStale = 10 * time.Second

	// Параллельный сбор стаканов
	results := make(map[string]fetchRes, len(exchanges))
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(exchanges))
	for _, ex := range exchanges {
		ex := ex
		go func() {
			defer wg.Done()
			start := time.Now()
			obs, err := ex.GetMultipleOrderBooks(symbols, cfg.Limit, time.Duration(cfg.DelayMS)*time.Millisecond)
			mu.Lock()
			results[ex.Name()] = fetchRes{name: ex.Name(), obs: obs, err: err, dur: time.Since(start)}
			mu.Unlock()
		}()
	}
	wg.Wait()

	allByEx := map[string]*domain.OrderBook{}
	for _, ex := range exchanges {
		name := ex.Name()
		res := results[name]
		pr.Infof("\n=== Работа с %s ===\n", name)

		if res.err != nil {
			pr.Warnf("Ошибка получения стакана с %s: %v\n", name, res.err)
			continue
		}
		if res.obs == nil || len(res.obs) == 0 {
			pr.Warnf("Предупреждение: от %s не получено ни одного стакана (возможен таймаут/сеть/API)\n", name)
			continue
		}
		pr.Infof("Успешно получено стаканов: %d\n", len(res.obs))
		if ob, ok := res.obs[symbol]; ok && ob != nil {
			// устаревание
			var t time.Time
			if ob.Timestamp > 1e12 {
				t = time.UnixMilli(ob.Timestamp)
			} else {
				t = time.Unix(ob.Timestamp, 0)
			}
			if time.Since(t) > maxStale {
				pr.Warnf("Данные %s:%s устарели на ~%ds\n", name, symbol, int(time.Since(t).Seconds()))
			}
			allByEx[name] = ob
			pr.ShowOrderBookSummary(ob)
		}
	}

	// Сравнение best bid/ask
	pr.Infof("\n=== Сравнение цен между биржами ===\n\n%s:\n", symbol)
	exKeys := make([]string, 0, len(allByEx))
	for k := range allByEx {
		exKeys = append(exKeys, k)
	}
	sort.Strings(exKeys)
	for _, exName := range exKeys {
		ob := allByEx[exName]
		if ob != nil && len(ob.Asks) > 0 && len(ob.Bids) > 0 {
			pr.ShowCrossExchangeLine(symbol, exName, ob.Asks[0].Price, ob.Bids[0].Price)
		}
	}

	// Контекст сделки
	fmt.Println("\n=== Контекст сделки ===")
	if dir == scenario.Sell {
		fmt.Printf("Инструмент: %s  | Объём: %.8f %s  | Глубина: %d\n", symbol, params.LeftCoinVolume, left, cfg.Limit)
	} else {
		fmt.Printf("Инструмент: %s  | Сумма: %.2f USDT  | Глубина: %d\n", symbol, params.LeftCoinVolume, cfg.Limit)
	}

	fees := map[string]scenario.FeeConfig{
		"Binance": {FeePct: 0.001, MinQty: 0, MinNotional: 10},
		"Bybit":   {FeePct: 0.001, MinQty: 0, MinNotional: 10},
	}

	in := scenario.Inputs{
		Direction:  dir,
		Symbol:     symbol,
		Right:      right,
		Amount:     params.LeftCoinVolume,
		OrderBooks: allByEx,
		Fees:       fees,
		Now:        now,
		MaxStale:   maxStale,
	}

	// Запуск стратегий
	resultsSnaps := make([]snap, 0, len(strategies))
	for idx, st := range strategies {
		res := st.Run(in)
		resultsSnaps = append(resultsSnaps, snap{name: st.Name(), res: res})

		pr.ShowScenarioHeader(st.Name(), dir)
		if dir == scenario.Buy {
			if res.TotalQty <= 0 {
				pr.Infof("Не удалось купить — нет ликвидности.\n")
				continue
			}
			pr.ShowBuyTotals(st.Name(), right, res, in.Amount)
		} else {
			if res.TotalUSDT <= 0 {
				pr.Infof("Не удалось продать — нет ликвидности.\n")
				continue
			}
			pr.ShowSellTotals(st.Name(), right, res)
		}

		// После сценария #1 показываем "что будет, если вложить всё на биржу X/Y"
		if idx == 0 { // сценарий #1 в списке
			evals := buildScenario1Evals(in, fees, allByEx)
			pr.ShowScenario1Rationale(res.Asset, dir, evals)
		}
	}

	// Финальное сравнение
	if dir == scenario.Buy {
		rows, best := buildBuyComparison(resultsSnaps, right)
		pr.ShowBuyComparison(best, right, rows)
	} else {
		rows, best := buildSellComparison(resultsSnaps, right)
		pr.ShowSellComparison(best, right, rows)
	}
	return nil
}

// Собираем “оценку одной биржи на весь объём” для сценария #1
func buildScenario1Evals(in scenario.Inputs, fees map[string]scenario.FeeConfig, books map[string]*domain.OrderBook) []scenario.ExchangeEval {
	out := make([]scenario.ExchangeEval, 0, len(books))
	switch in.Direction {
	case scenario.Buy:
		for ex, ob := range books {
			if ob == nil || len(ob.Asks) == 0 {
				continue
			}
			fee := fees[ex].FeePct
			qty, avg, spent := orderbook.BuyQtyFromAsksWithFee(ob.Asks, in.Amount, fee)
			if qty <= 0 {
				out = append(out, scenario.ExchangeEval{Exchange: ex})
				continue
			}
			comm := spent * fee / (1 + fee)
			cov := 0.0
			if in.Amount > 0 {
				cov = (spent / in.Amount) * 100.0
			}
			out = append(out, scenario.ExchangeEval{
				Exchange:   ex,
				AvgPrice:   avg,
				Qty:        qty,
				AmountUSDT: spent,
				Commission: comm,
				Coverage:   cov,
			})
		}
	case scenario.Sell:
		for ex, ob := range books {
			if ob == nil || len(ob.Bids) == 0 {
				continue
			}
			fee := fees[ex].FeePct
			usdt, avg := orderbook.SellFromBidsWithFee(ob.Bids, in.Amount, fee)
			if usdt <= 0 || avg <= 0 {
				out = append(out, scenario.ExchangeEval{Exchange: ex})
				continue
			}
			soldQty := usdt / avg
			comm := usdt * fee / (1 - fee)
			cov := 0.0
			if in.Amount > 0 {
				cov = (soldQty / in.Amount) * 100.0
			}
			out = append(out, scenario.ExchangeEval{
				Exchange:   ex,
				AvgPrice:   avg,
				Qty:        soldQty,
				AmountUSDT: usdt,
				Commission: comm,
				Coverage:   cov,
			})
		}
	}
	return out
}

// ===== сравнения (как у тебя, но с человекочитаемыми формулировками) =====

func buildBuyComparison(results []snap, _ string) (rows []string, bestName string) {
	valid := make([]snap, 0, len(results))
	for _, s := range results {
		if s.res.TotalQty > 0 {
			valid = append(valid, s)
		}
	}
	if len(valid) < 2 {
		return []string{"Недостаточно данных для сравнения."}, ""
	}

	best := valid[0]
	for _, s := range valid[1:] {
		if s.res.TotalQty > best.res.TotalQty {
			best = s
		}
	}
	bestName = best.name
	asset := best.res.Asset
	rows = append(rows, fmt.Sprintf("Лучший по количеству: %s — %.8f %s (средняя цена за 1 %s = %.8f USDT)",
		best.name, best.res.TotalQty, asset, asset, best.res.AveragePrice))
	for _, s := range valid {
		if s.name == best.name {
			continue
		}
		diff := best.res.TotalQty - s.res.TotalQty
		pct := (diff / s.res.TotalQty) * 100
		rows = append(rows, fmt.Sprintf("  Преимущество над %s: +%.8f %s (≈ %.4f%%)", s.name, diff, asset, pct))
	}
	bestPrice := valid[0]
	for _, s := range valid[1:] {
		if s.res.AveragePrice > 0 && s.res.AveragePrice < bestPrice.res.AveragePrice {
			bestPrice = s
		}
	}
	if bestPrice.res.AveragePrice > 0 {
		rows = append(rows, fmt.Sprintf("Лучшая средняя цена: %s — %.8f USDT за 1 %s",
			bestPrice.name, bestPrice.res.AveragePrice, bestPrice.res.Asset))
	}
	return rows, bestName
}

func buildSellComparison(results []snap, _ string) (rows []string, bestName string) {
	valid := make([]snap, 0, len(results))
	for _, s := range results {
		if s.res.TotalUSDT > 0 {
			valid = append(valid, s)
		}
	}
	if len(valid) < 2 {
		return []string{"Недостаточно данных для сравнения."}, ""
	}

	best := valid[0]
	for _, s := range valid[1:] {
		if s.res.TotalUSDT > best.res.TotalUSDT {
			best = s
		}
	}
	bestName = best.name
	asset := best.res.Asset
	rows = append(rows, fmt.Sprintf("Лучшая выручка: %s — %.2f USDT (средняя цена продажи 1 %s = %.8f USDT)",
		best.name, best.res.TotalUSDT, asset, best.res.AveragePrice))
	for _, s := range valid {
		if s.name == best.name {
			continue
		}
		diff := best.res.TotalUSDT - s.res.TotalUSDT
		pct := (diff / s.res.TotalUSDT) * 100.0
		rows = append(rows, fmt.Sprintf("  Преимущество над %s: +%.2f USDT (≈ %.4f%%)", s.name, diff, pct))
	}
	bestPrice := valid[0]
	for _, s := range valid[1:] {
		if s.res.AveragePrice > bestPrice.res.AveragePrice {
			bestPrice = s
		}
	}
	if bestPrice.res.AveragePrice > 0 {
		rows = append(rows, fmt.Sprintf("Лучшая средняя цена продажи: %s — %.8f USDT за 1 %s",
			bestPrice.name, bestPrice.res.AveragePrice, bestPrice.res.Asset))
	}
	return rows, bestName
}
