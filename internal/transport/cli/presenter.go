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

func (c *CLIPresenter) Infof(format string, args ...any)  { fmt.Printf(format, args...) }
func (c *CLIPresenter) Warnf(format string, args ...any)  { fmt.Printf(format, args...) }
func (c *CLIPresenter) Errorf(format string, args ...any) { fmt.Printf(format, args...) }

// Короткое резюме по стакану конкретной биржи (топ-уровни ask/bid)
func (c *CLIPresenter) ShowOrderBookSummary(ob *domain.OrderBook) {
	if ob == nil {
		return
	}
	var bestAsk, bestBid string
	if len(ob.Asks) > 0 {
		bestAsk = ob.Asks[0].Price
	}
	if len(ob.Bids) > 0 {
		bestBid = ob.Bids[0].Price
	}
	// timestamp может быть в секундах или миллисекундах
	var ts time.Time
	if ob.Timestamp > 1e12 {
		ts = time.UnixMilli(ob.Timestamp)
	} else {
		ts = time.Unix(ob.Timestamp, 0)
	}
	fmt.Printf("  %-7s %-12s  bestBid=%-12s  bestAsk=%-12s  @%s\n",
		ob.Exchange, ob.Symbol, bestBid, bestAsk, ts.Format("15:04:05"))
}

// Печать результатов одного сценария
func (c *CLIPresenter) RenderScenario(title string, r scenario.Result) {
	fmt.Printf("\n== %s ==\n", title)
	if len(r.Legs) == 0 || r.TotalQty == 0 || r.AveragePrice == 0 {
		fmt.Println("Нет данных для отображения.")
		return
	}

	fmt.Printf("Asset: %s\n", r.Asset)
	fmt.Printf("VWAP:  %.8f\n", r.AveragePrice)
	fmt.Printf("Итого монет: %.8f\n", r.TotalQty)
	fmt.Printf("Итого USDT:  %.2f\n", r.TotalUSDT)
	if r.Leftover > 0 {
		fmt.Printf("Не израсходовано (из-за глубины): %.2f USDT\n", r.Leftover)
	}

	// Агрегируем по бирже — чтобы в Legs не было «шума» из множества дробных ног
	type aggRow struct {
		ex   string
		qty  float64
		usdt float64
		avg  float64
	}
	agg := map[string]*aggRow{}
	for _, l := range r.Legs {
		row := agg[l.Exchange]
		if row == nil {
			row = &aggRow{ex: l.Exchange}
			agg[l.Exchange] = row
		}
		row.qty += l.Qty
		row.usdt += l.AmountUSDT
	}

	rows := make([]aggRow, 0, len(agg))
	for _, v := range agg {
		if v.qty > 0 {
			v.avg = v.usdt / v.qty
		}
		rows = append(rows, *v)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].usdt > rows[j].usdt })

	fmt.Println("\nБиржа            Кол-во (qty)        Цена (avg)        Сумма (USDT)")
	fmt.Println("---------------------------------------------------------------------")
	for _, x := range rows {
		fmt.Printf("%-16s %-18.8f %-16.8f %.2f\n", x.ex, x.qty, x.avg, x.usdt)
	}
}

// Сравнительная печать нескольких сценариев (по имени → результат)
func (c *CLIPresenter) RenderComparisons(results map[string]scenario.Result) {
	type row struct {
		name string
		vwap float64
		qty  float64
		usdt float64
	}
	var xs []row
	for name, r := range results {
		xs = append(xs, row{name: name, vwap: r.AveragePrice, qty: r.TotalQty, usdt: r.TotalUSDT})
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].vwap < xs[j].vwap })

	fmt.Println("\n=== Сравнение сценариев ===")
	fmt.Println("Сценарий                         VWAP              Qty              USDT")
	fmt.Println("--------------------------------------------------------------------------")
	for _, x := range xs {
		fmt.Printf("%-30s %-17.8f %-16.8f %.2f\n", x.name, x.vwap, x.qty, x.usdt)
	}
}
