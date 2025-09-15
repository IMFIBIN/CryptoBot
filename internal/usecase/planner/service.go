package planner

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/usecase/scenario"
)

// Service — чистый планировщик.
type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

// Plan — рассчитывает план исполнения:
// 1) Quote=USDT,  Base!=USDT → покупка BASE за USDT (Buy)
// 2) Base=USDT,   Quote!=USDT → продажа QUOTE за USDT (Sell)
// 3) Base!=USDT,  Quote!=USDT → QUOTE->USDT и покупка BASE (через мост USDT)
func (s *Service) Plan(ctx context.Context, in Request) (Result, error) {
	base := strings.ToUpper(strings.TrimSpace(in.Base))
	quote := strings.ToUpper(strings.TrimSpace(in.Quote))
	if base == "" || quote == "" {
		return Result{}, fmt.Errorf("unsupported pair selection")
	}
	if base == quote {
		return Result{}, fmt.Errorf("нельзя выбирать одинаковые монеты: %s/%s", base, quote)
	}
	if in.Amount <= 0 {
		return Result{}, fmt.Errorf("amount must be > 0")
	}

	// нормализуем название сценария
	sc := strings.ToLower(strings.TrimSpace(in.Scenario))
	if sc == "" {
		sc = "optimal"
	}
	runScenario := func() interface {
		Run(scenario.Inputs) scenario.Result
		Name() string
	} {
		switch sc {
		case "best_single":
			return scenario.BestSingle{}
		case "equal_split":
			return scenario.EqualSplit{}
		default:
			return scenario.Optimal{}
		}
	}()

	now := time.Now()
	depth := 0 // «максимальная» глубина оставлена на реализацию Repo

	var res Result
	res.Scenario = sc
	res.Base = base
	res.Quote = quote
	res.GeneratedAt = now.Format("15:04 02.01.2006")

	switch {
	// === Покупка BASE за USDT ===
	case !isUSDT(base) && isUSDT(quote):
		// тянем стаканы <BASE>/USDT со всех бирж
		books, diags, err := s.repo.FetchAllBooks(ctx, base, depth)
		if err != nil {
			return Result{}, err
		}
		res.Diagnostics = append(res.Diagnostics, diags...)

		// готовим вход для сценария
		inp := scenario.Inputs{
			Direction:  scenario.Buy,
			Symbol:     base + "USDT",
			Right:      base,      // для BUY это «получаемая» монета
			Amount:     in.Amount, // бюджет в USDT
			OrderBooks: toOrderBooks(books, base+"USDT", now),
			Now:        now,
			MaxStale:   0,
		}
		out := runScenario.Run(inp)

		// маппинг результата
		res.VWAP = round2(out.AveragePrice)   // USDT за 1 BASE
		res.TotalCost = round2(out.TotalUSDT) // потрачено USDT
		res.Unspent = round2(out.Leftover)    // не потратили USDT
		res.Generated = out.TotalQty          // получили BASE
		res.Legs = toPlanLegs(out.Legs)       // Qty — это BASE на ножке

	// === Продажа QUOTE за USDT ===
	case isUSDT(base) && !isUSDT(quote):
		books, diags, err := s.repo.FetchAllBooks(ctx, quote, depth)
		if err != nil {
			return Result{}, err
		}
		res.Diagnostics = append(res.Diagnostics, diags...)

		inp := scenario.Inputs{
			Direction:  scenario.Sell,
			Symbol:     quote + "USDT",
			Right:      "USDT",
			Amount:     in.Amount, // количество монеты QUOTE, которое продаём
			OrderBooks: toOrderBooks(books, quote+"USDT", now),
			Now:        now,
			MaxStale:   0,
		}
		out := runScenario.Run(inp)

		// Для SELL сценарии обычно не выставляют Leftover, поэтому считаем остаток сами
		sold := out.TotalQty                   // реально продали QUOTE
		res.VWAP = round2(out.AveragePrice)    // USDT за 1 QUOTE
		res.TotalCost = round2(out.TotalUSDT)  // получили USDT
		res.Unspent = round2(in.Amount - sold) // не успели продать QUOTE
		if res.Unspent < 0 {
			res.Unspent = 0
		}
		res.Generated = out.TotalUSDT   // получили USDT
		res.Legs = toPlanLegs(out.Legs) // Qty — это QUOTE на ножке

	// === Маршрут через USDT: QUOTE -> USDT -> BASE ===
	case !isUSDT(base) && !isUSDT(quote):
		// 1) продаём QUOTE -> USDT выбранным сценарием
		booksQ, diagsQ, err := s.repo.FetchAllBooks(ctx, quote, depth)
		if err != nil {
			return Result{}, err
		}
		res.Diagnostics = append(res.Diagnostics, diagsQ...)
		inSell := scenario.Inputs{
			Direction:  scenario.Sell,
			Symbol:     quote + "USDT",
			Right:      "USDT",
			Amount:     in.Amount, // QUOTE
			OrderBooks: toOrderBooks(booksQ, quote+"USDT", now),
			Now:        now,
			MaxStale:   0,
		}
		outSell := runScenario.Run(inSell)
		soldQuote := outSell.TotalQty    // сколько QUOTE реально продали
		usdProceeds := outSell.TotalUSDT // сколько USDT получили

		if soldQuote <= 0 || usdProceeds <= 0 {
			return Result{}, fmt.Errorf("insufficient depth on QUOTE->USDT leg")
		}

		// 2) покупаем BASE на полученные USDT тем же сценарием
		booksB, diagsB, err := s.repo.FetchAllBooks(ctx, base, depth)
		if err != nil {
			return Result{}, err
		}
		res.Diagnostics = append(res.Diagnostics, diagsB...)
		inBuy := scenario.Inputs{
			Direction:  scenario.Buy,
			Symbol:     base + "USDT",
			Right:      base,
			Amount:     usdProceeds, // бюджет в USDT
			OrderBooks: toOrderBooks(booksB, base+"USDT", now),
			Now:        now,
			MaxStale:   0,
		}
		outBuy := runScenario.Run(inBuy)
		gotBase := outBuy.TotalQty
		if gotBase <= 0 {
			return Result{}, fmt.Errorf("insufficient depth on USDT->BASE leg")
		}

		// Итоги (важно: цена = QUOTE за 1 BASE)
		res.VWAP = round2(soldQuote / gotBase)      // QUOTE/BASE
		res.TotalCost = round2(soldQuote)           // потратили QUOTE
		res.Unspent = round2(in.Amount - soldQuote) // остаток QUOTE
		if res.Unspent < 0 {
			res.Unspent = 0
		}
		res.Generated = gotBase

		// Склеиваем ножки: сначала продажа (QUOTE), затем покупка (BASE)
		res.Legs = append(toPlanLegs(outSell.Legs), toPlanLegs(outBuy.Legs)...)
	}

	return res, nil
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// ------------------------ ВСПОМОГАТЕЛЬНЫЕ МАППЕРЫ ------------------------

func toOrderBooks(src []Book, symbol string, now time.Time) map[string]*domain.OrderBook {
	out := make(map[string]*domain.OrderBook, len(src))
	for _, b := range src {
		ob := &domain.OrderBook{
			Symbol:    symbol,
			Exchange:  b.Exchange,
			Timestamp: now.UnixMilli(),
		}
		ob.Asks = make([]domain.Order, 0, len(b.Asks))
		for _, a := range b.Asks {
			ob.Asks = append(ob.Asks, domain.Order{
				Price:    strconv.FormatFloat(a.Price, 'f', 8, 64),
				Quantity: strconv.FormatFloat(a.Qty, 'f', 8, 64),
			})
		}
		ob.Bids = make([]domain.Order, 0, len(b.Bids))
		for _, d := range b.Bids {
			ob.Bids = append(ob.Bids, domain.Order{
				Price:    strconv.FormatFloat(d.Price, 'f', 8, 64),
				Quantity: strconv.FormatFloat(d.Qty, 'f', 8, 64),
			})
		}
		out[b.Exchange] = ob
	}
	return out
}

func toPlanLegs(src []scenario.Leg) []Leg {
	legs := make([]Leg, 0, len(src))
	for _, l := range src {
		legs = append(legs, Leg{
			Exchange: l.Exchange,
			Amount:   l.Qty,   // Qty монеты на ножке
			Price:    l.Price, // цена (USDT/монета)
		})
	}
	return legs
}
