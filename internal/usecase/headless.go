package usecase

import (
	"sort"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/usecase/fees"
	"cryptobot/internal/usecase/scenario"
)

type Snap struct {
	Name string
	Res  scenario.Result
}

func RunHeadless(cfg domain.Config, exchanges []domain.Exchange,
	dir scenario.Direction, symbol, left, right string, amount float64, depth int,
) ([]Snap, error) {

	if depth <= 0 {
		depth = cfg.Limit
	}

	// собираем стаканы параллельно (короткая версия)
	allByEx := map[string]*domain.OrderBook{}
	type item struct {
		name string
		ob   map[string]*domain.OrderBook
	}
	ch := make(chan item, len(exchanges))
	for _, ex := range exchanges {
		go func(ex domain.Exchange) {
			obs, _ := ex.GetMultipleOrderBooks([]string{symbol}, depth, time.Duration(cfg.DelayMS)*time.Millisecond)
			ch <- item{name: ex.Name(), ob: obs}
		}(ex)
	}
	for i := 0; i < len(exchanges); i++ {
		it := <-ch
		if it.ob != nil && it.ob[symbol] != nil {
			allByEx[it.name] = it.ob[symbol]
		}
	}

	// комиссии
	feeModels := map[string]fees.Fee{
		"Binance": fees.NewRelative(0.001),
		"Bybit":   fees.NewRelative(0.001),
		"OKX":     fees.NewAbsolute(1.0),
		"KuCoin":  fees.NewRelative(0.001),
		"Bitget":  fees.NewRelative(0.001),
		"HTX":     fees.NewRelative(0.001),
		"Gate":    fees.NewRelative(0.001),
	}

	in := scenario.Inputs{
		Direction:  dir,
		Symbol:     symbol,
		Right:      right,
		Amount:     amount,
		OrderBooks: allByEx,
		Fees:       feeModels,
		Now:        time.Now(),
		MaxStale:   10 * time.Second,
	}

	strategies := []struct {
		name string
		run  func(scenario.Inputs) scenario.Result
	}{
		{"Сценарий #1 (лучшая биржа)", scenario.BestSingle{}.Run},
		{"Сценарий #2 (равное распределение)", scenario.EqualSplit{}.Run},
		{"Сценарий #3 (оптимальное распределение)", scenario.Optimal{}.Run},
	}

	var snaps []Snap
	for _, st := range strategies {
		res := st.run(in)
		snaps = append(snaps, Snap{Name: st.name, Res: res})
	}

	// отсортируем по среднему сценарию (не обязательно)
	sort.SliceStable(snaps, func(i, j int) bool { return snaps[i].Name < snaps[j].Name })
	return snaps, nil
}
