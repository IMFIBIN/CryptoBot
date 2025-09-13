package usecase

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/transport/cli"
	"cryptobot/internal/usecase/scenario"
)

// --- локальные интерфейсы ---

type strategy interface {
	Name() string
	Run(in scenario.Inputs) scenario.Result
}

type presenterLite interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	ShowOrderBookSummary(ob *domain.OrderBook)
	RenderScenario(title string, r scenario.Result)
	RenderComparisons(results map[string]scenario.Result)
}

type snap struct {
	name string
	res  scenario.Result
}

func Run(cfg domain.Config, exchanges []domain.Exchange) error {
	pr := cli.NewCLIPresenter()
	strategies := []strategy{
		scenario.BestSingle{},
		scenario.EqualSplit{},
		scenario.Optimal{},
	}
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
	pr presenterLite,
	strategies []strategy,
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
			// limit всегда = 0 → максимальная глубина
			obs, err := ex.GetMultipleOrderBooks(symbols, 0, time.Duration(cfg.DelayMS)*time.Millisecond)
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
			pr.Warnf("Предупреждение: от %s не получено ни одного стакана\n", name)
			continue
		}
		pr.Infof("Успешно получено стаканов: %d\n", len(res.obs))
		if ob, ok := res.obs[symbol]; ok && ob != nil {
			// проверка на устаревание
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

	// формируем Inputs без комиссий
	in := scenario.Inputs{
		Direction:  dir,
		Symbol:     symbol,
		Right:      right,
		Amount:     params.LeftCoinVolume,
		OrderBooks: allByEx,
		Now:        now,
		MaxStale:   maxStale,
	}

	// Запуск стратегий
	resultsMap := make(map[string]scenario.Result, len(strategies))
	var snaps []snap
	for _, st := range strategies {
		res := st.Run(in)
		snaps = append(snaps, snap{name: st.Name(), res: res})
		resultsMap[st.Name()] = res
	}

	// Печать каждого сценария
	for _, sn := range snaps {
		title := fmt.Sprintf("%s", sn.name)
		pr.RenderScenario(title, sn.res)
	}

	// Сравнение сценариев одной таблицей
	// Для наглядности отсортируем: BUY — по возрастанию VWAP; SELL — по убыванию VWAP.
	if dir == scenario.Buy {
		sort.Slice(snaps, func(i, j int) bool { return snaps[i].res.AveragePrice < snaps[j].res.AveragePrice })
	} else {
		sort.Slice(snaps, func(i, j int) bool { return snaps[i].res.AveragePrice > snaps[j].res.AveragePrice })
	}
	pr.RenderComparisons(resultsMap)

	return nil
}
