package planner

import (
	"context"
	"strings"
	"time"
)

// ====== Чистые типы use-case (не зависят от HTTP и конкретных бирж) ======

type Level struct {
	Price float64 // цена
	Qty   float64 // количество
}

type Book struct {
	Exchange string
	Asks     []Level
	Bids     []Level
}

// Leg — одна "ножка" плана на конкретной бирже.
type Leg struct {
	Exchange string
	Amount   float64 // количество монеты (BASE при покупке, QUOTE при продаже)
	Price    float64 // цена (USDT за 1 BASE или USDT за 1 QUOTE)
}

// Request — вход для расчёта плана.
type Request struct {
	Base     string  // что покупаем (или что получаем в итоге для sideRoute)
	Quote    string  // чем платим (или что тратим для sideRoute)
	Amount   float64 // сколько платим (в USDT для sideBuy; в монете для sideSell/sideRoute)
	Scenario string  // пока игнорируется (будем внедрять позже)
}

// Result — результат расчёта.
type Result struct {
	Scenario    string
	Base        string
	Quote       string
	VWAP        float64 // см. ниже: единицы зависят от направления
	TotalCost   float64 // сколько реально потратили (в USDT для sideBuy, в QUOTE для sideRoute)
	Unspent     float64 // остаток неиспользованных средств (в тех же единицах, что и TotalCost)
	Generated   float64 // сколько реально получили целевой монеты (BASE для sideBuy/sideRoute; USDT для sideSell)
	Legs        []Leg
	Diagnostics []string
	GeneratedAt string // "15:04 02.01.2006"
}

// Repo — интерфейс доступа к стаканам (реализация будет в инфраструктуре).
type Repo interface {
	// FetchAllBooks должен вернуть стаканы <coin>/USDT по всем биржам.
	FetchAllBooks(ctx context.Context, coin string, depth int) ([]Book, []string, error)
}

func nowString() string { return time.Now().Format("15:04 02.01.2006") }

func isUSDT(s string) bool { return strings.EqualFold(strings.TrimSpace(s), "USDT") }
