package usecase

import (
	"sort"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/usecase/scenario"
)

type Snap struct {
	Name string
	Res  scenario.Result
}

type HeadlessInput struct {
	Direction  scenario.Direction
	Symbol     string
	Right      string
	Amount     float64
	OrderBooks map[string]*domain.OrderBook
	Now        time.Time
	MaxStale   time.Duration
}

func RunHeadless(in HeadlessInput) ([]Snap, error) {
	inputs := scenario.Inputs{
		Direction:  in.Direction,
		Symbol:     in.Symbol,
		Right:      in.Right,
		Amount:     in.Amount,
		OrderBooks: in.OrderBooks,
		Now:        in.Now,
		MaxStale:   in.MaxStale,
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
		res := st.run(inputs)
		snaps = append(snaps, Snap{Name: st.name, Res: res})
	}

	sort.SliceStable(snaps, func(i, j int) bool { return snaps[i].Name < snaps[j].Name })
	return snaps, nil
}
