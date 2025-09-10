package cli

import (
	"fmt"
	"sort"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/usecase/scenario"
)

type CLIPresenter struct{}

func NewCLIPresenter() *CLIPresenter { return &CLIPresenter{} }

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
	byEx := c.aggregate(res.Legs)
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
	byEx := c.aggregate(res.Legs)
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

// ====== агрегация по биржам (используем FeeUSDT из Legs) ======

type aggRow struct {
	ex       string
	qty      float64
	usdt     float64
	avgPrice float64
	feeAmt   float64
}

func (c *CLIPresenter) aggregate(legs []scenario.Leg) map[string]*aggRow {
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
		row.feeAmt += l.FeeUSDT
	}
	for _, r := range m {
		if r.qty > 0 {
			r.avgPrice = r.usdt / r.qty
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
