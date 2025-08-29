package cli

import (
	"fmt"
	"sort"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/usecase/scenario"
)

type CLIPresenter struct {
	fees map[string]scenario.FeeConfig
}

func NewCLIPresenterWithFees(fees map[string]scenario.FeeConfig) *CLIPresenter {
	cp := make(map[string]scenario.FeeConfig, len(fees))
	for k, v := range fees {
		cp[k] = v
	}
	return &CLIPresenter{fees: cp}
}

func (c *CLIPresenter) Infof(format string, args ...any) { fmt.Printf(format, args...) }
func (c *CLIPresenter) Warnf(format string, args ...any) { fmt.Printf(format, args...) }

func (c *CLIPresenter) ShowOrderBookSummary(ob *domain.OrderBook) {
	fmt.Printf("\n=== %s - %s ===\n", ob.Exchange, ob.Symbol)
	var t time.Time
	if ob.Timestamp > 1e12 {
		t = time.UnixMilli(ob.Timestamp)
	} else {
		t = time.Unix(ob.Timestamp, 0)
	}
	fmt.Printf("Время: %s\n", t.Format("15:04 02.01.2006"))
	if len(ob.Asks) > 0 && len(ob.Bids) > 0 {
		fmt.Printf("Ask=%s, Bid=%s\n", ob.Asks[0].Price, ob.Bids[0].Price)
	}
}

func (c *CLIPresenter) ShowCrossExchangeLine(_ string, exchange, ask, bid string) {
	fmt.Printf("  %s: Ask=%s, Bid=%s\n", exchange, ask, bid)
}

func (c *CLIPresenter) ShowScenarioHeader(name string, dir scenario.Direction) {
	if dir == scenario.Buy {
		fmt.Printf("\n=== %s (BUY) ===\n", name)
	} else {
		fmt.Printf("\n=== %s (SELL) ===\n", name)
	}
}

func (c *CLIPresenter) ShowBuyTotals(_, _ string, res scenario.Result, amount float64) {
	fmt.Printf("Итого: %.8f %s, средняя цена за 1 %s = %.8f USDT\n",
		res.TotalQty, res.Asset, res.Asset, res.AveragePrice)
	if res.Leftover > 0 {
		fmt.Printf("Лимит стакана исчерпан: потрачено %.2f USDT, остаток %.2f USDT не использован\n",
			res.TotalUSDT, amount-res.TotalUSDT)
	}
	byEx := c.aggregateBuy(res.Legs)
	if len(byEx) > 0 {
		fmt.Println("План исполнения по биржам:")
		rows := c.sorted(byEx)
		for _, r := range rows {
			fmt.Printf("  %s: %.8f %s, средняя цена %.8f USDT, комиссия %.2f USDT\n",
				r.ex, r.qty, res.Asset, r.avgPrice, r.feeAmt)
		}
	}
}

func (c *CLIPresenter) ShowSellTotals(_, _ string, res scenario.Result) {
	fmt.Printf("Итого: %.2f USDT, средняя цена продажи 1 %s = %.8f USDT\n",
		res.TotalUSDT, res.Asset, res.AveragePrice)
	byEx := c.aggregateSell(res.Legs)
	if len(byEx) > 0 {
		fmt.Println("План исполнения по биржам:")
		rows := c.sorted(byEx)
		for _, r := range rows {
			fmt.Printf("  %s: продано %.8f %s, средняя цена %.8f USDT, комиссия %.2f USDT\n",
				r.ex, r.qty, res.Asset, r.avgPrice, r.feeAmt)
		}
	}
}

func (c *CLIPresenter) ShowBuyComparison(_, _ string, rows []string) {
	fmt.Println("\n=== Итоговое сравнение сценариев (BUY) ===")
	for _, r := range rows {
		fmt.Println(r)
	}
}

func (c *CLIPresenter) ShowSellComparison(_, _ string, rows []string) {
	fmt.Println("\n=== Итоговое сравнение сценариев (SELL) ===")
	for _, r := range rows {
		fmt.Println(r)
	}
}

// === Новое: печать обоснования сценария #1 (“вложить всё на одну биржу”) ===

func (c *CLIPresenter) ShowScenario1Rationale(asset string, dir scenario.Direction, evals []scenario.ExchangeEval) {
	if len(evals) == 0 {
		return
	}
	fmt.Println("\nОснование выбора (если вложить всю сумму на одну биржу):")

	// Сортировка по выгодности для наглядности
	if dir == scenario.Buy {
		sort.Slice(evals, func(i, j int) bool {
			if evals[i].AvgPrice == evals[j].AvgPrice {
				return evals[i].Coverage > evals[j].Coverage
			}
			return evals[i].AvgPrice < evals[j].AvgPrice // ниже — лучше
		})
	} else {
		sort.Slice(evals, func(i, j int) bool {
			if evals[i].AvgPrice == evals[j].AvgPrice {
				return evals[i].Coverage > evals[j].Coverage
			}
			return evals[i].AvgPrice > evals[j].AvgPrice // выше — лучше
		})
	}

	for idx, e := range evals {
		tag := ""
		if idx == 0 {
			tag = "  ← лучшее"
		}
		if dir == scenario.Buy {
			fmt.Printf("  %s: средняя цена за 1 %s = %.8f USDT, комиссия %.2f USDT, покрытие %.2f%%%s\n",
				e.Exchange, asset, e.AvgPrice, e.Commission, e.Coverage, tag)
		} else {
			fmt.Printf("  %s: средняя цена продажи 1 %s = %.8f USDT, комиссия %.2f USDT, покрытие %.2f%%%s\n",
				e.Exchange, asset, e.AvgPrice, e.Commission, e.Coverage, tag)
		}
	}

	// Итог: на сколько лучшая биржа лучше ближайшего конкурента
	if len(evals) >= 2 {
		best, next := evals[0], evals[1]
		if dir == scenario.Buy {
			delta := next.AvgPrice - best.AvgPrice // экономия на 1 монете
			pct := delta / next.AvgPrice * 100
			fmt.Printf("Итог: наиболее выгодная биржа — %s, цена ниже на %.6f USDT (≈ %.4f%%) относительно %s\n",
				best.Exchange, delta, pct, next.Exchange)
		} else {
			delta := best.AvgPrice - next.AvgPrice // выигрыш на 1 монете
			pct := delta / next.AvgPrice * 100
			fmt.Printf("Итог: наиболее выгодная биржа — %s, цена продажи за 1 %s выше на %.6f USDT (≈ %.4f%%) относительно %s\n",
				best.Exchange, asset, delta, pct, next.Exchange)
		}
	}

	// Критерий
	if dir == scenario.Buy {
		fmt.Printf("Критерий выбора: минимальная средняя цена за 1 %s (учтены комиссия и глубина стакана).\n", asset)
	} else {
		fmt.Printf("Критерий выбора: максимальная средняя цена продажи 1 %s (учтены комиссия и глубина стакана).\n", asset)
	}
}

// ===== внутренние агрегации для печати (как было) =====

type aggRow struct {
	ex       string
	qty      float64
	usdt     float64
	avgPrice float64
	feeAmt   float64
}

func (c *CLIPresenter) aggregateBuy(legs []scenario.Leg) map[string]*aggRow {
	if len(legs) == 0 {
		return nil
	}
	m := map[string]*aggRow{}
	for _, l := range legs {
		row := m[l.Exchange]
		if row == nil {
			row = &aggRow{ex: l.Exchange}
			m[l.Exchange] = row
		}
		row.qty += l.Qty
		row.usdt += l.AmountUSDT
	}
	for ex, r := range m {
		if r.qty > 0 {
			r.avgPrice = r.usdt / r.qty
		}
		fee := c.fees[ex].FeePct
		if fee > 0 {
			r.feeAmt = r.usdt * fee / (1 + fee)
		}
	}
	return m
}

func (c *CLIPresenter) aggregateSell(legs []scenario.Leg) map[string]*aggRow {
	if len(legs) == 0 {
		return nil
	}
	m := map[string]*aggRow{}
	for _, l := range legs {
		row := m[l.Exchange]
		if row == nil {
			row = &aggRow{ex: l.Exchange}
			m[l.Exchange] = row
		}
		row.qty += l.Qty
		row.usdt += l.AmountUSDT
	}
	for ex, r := range m {
		if r.qty > 0 {
			r.avgPrice = r.usdt / r.qty
		}
		fee := c.fees[ex].FeePct
		if fee > 0 {
			r.feeAmt = r.usdt * fee / (1 - fee)
		}
	}
	return m
}

func (c *CLIPresenter) sorted(m map[string]*aggRow) []aggRow {
	rows := make([]aggRow, 0, len(m))
	for _, v := range m {
		rows = append(rows, *v)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].usdt > rows[j].usdt })
	return rows
}
