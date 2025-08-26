package usecase

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"cryptobot/internal/domain"
	"cryptobot/internal/transport/cli"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Run — единая точка сценария:
// 1) интерактивный ввод;
// 2) сбор стаканов с бирж;
// 3) печать сводок и сравнение;
// 4) расчёт, сколько монеты купим на введённый объём USDT.
func Run(cfg domain.Config, exchanges []domain.Exchange) error {
	// 1) Ввод
	params := cli.GetInteractiveParams()
	fmt.Printf("DEBUG: base=%s, target=%s, amount=%.2f\n",
		params.LeftCoinName, params.RightCoinName, params.LeftCoinVolume)

	// 2) Формируем символ для стаканов из выбора пользователя: <RIGHT>USDT
	right := strings.ToUpper(params.RightCoinName)
	symbol := right + "USDT"
	symbols := []string{symbol}

	allResults := make(map[string]map[string]*domain.OrderBook)

	fmt.Println("=== Крипто-биржи Монитор ===")
	fmt.Print("Доступные биржи: [")
	for i, ex := range exchanges {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(ex.Name())
	}
	fmt.Println("]")

	for _, ex := range exchanges {
		fmt.Printf("\n=== Работа с %s ===\n", ex.Name())
		obs, err := ex.GetMultipleOrderBooks(
			symbols,
			cfg.Limit,
			time.Duration(cfg.DelayMS)*time.Millisecond,
		)
		if err != nil {
			fmt.Printf("Ошибка получения данных с %s: %v\n", ex.Name(), err)
			continue
		}
		allResults[ex.Name()] = obs
		fmt.Printf("Успешно получено стаканов: %d\n", len(obs))
		for _, ob := range obs {
			printOrderBookSummary(ob, false)
		}
	}

	// 3) Сравнение по выбранному символу
	fmt.Println("\n=== Сравнение цен между биржами ===")
	fmt.Printf("\n%s:\n", symbol)
	for exName, obs := range allResults {
		if ob, ok := obs[symbol]; ok && len(ob.Asks) > 0 && len(ob.Bids) > 0 {
			fmt.Printf("  %s: Ask=%s, Bid=%s\n", exName, ob.Asks[0].Price, ob.Bids[0].Price)
		}
	}

	// 4) Детально по выбранному символу
	fmt.Printf("\n=== Детальная информация по %s ===\n", symbol)
	for _, obs := range allResults {
		if ob, ok := obs[symbol]; ok {
			printOrderBookSummary(ob, true)
		}
	}

	// 5) Расчёт покупки target-коина на сумму USDT
	fmt.Printf("\n=== Расчёт покупки %s на сумму %.2f USDT ===\n", right, params.LeftCoinVolume)
	type quote struct {
		exName   string
		qty      float64
		avgPrice float64
	}
	var best *quote

	// форматер для вывода чисел
	pr := message.NewPrinter(language.Russian)

	for exName, books := range allResults {
		ob, ok := books[symbol]
		if !ok || ob == nil || len(ob.Asks) == 0 {
			fmt.Printf("%s: нет данных по %s\n", exName, symbol)
			continue
		}
		qty, avgPrice := buyQtyFromAsks(ob.Asks, params.LeftCoinVolume)
		if qty <= 0 {
			fmt.Printf("%s: недостаточная ликвидность по %s\n", exName, symbol)
			continue
		}
		fmt.Printf("%s → получим ~ %.8f %s, средняя цена ~ %.8f USDT\n", exName, qty, right, avgPrice)

		// Остаток USDT после покупки на этой бирже
		spent := avgPrice * qty
		leftover := params.LeftCoinVolume - spent
		if leftover > 0 {
			pr.Printf("  Остаток USDT: %0.2f\n", leftover)
		}

		if best == nil || avgPrice < best.avgPrice {
			best = &quote{exName: exName, qty: qty, avgPrice: avgPrice}
		}
	}

	if best != nil {
		fmt.Printf("\nЛучший обмен: %s → %.8f %s по средней цене ~ %.8f USDT\n",
			best.exName, best.qty, right, best.avgPrice)

		// Остаток USDT для лучшего обмена
		bestSpent := best.avgPrice * best.qty
		bestLeftover := params.LeftCoinVolume - bestSpent
		if bestLeftover > 0 {
			pr.Printf("Остаток USDT: %0.2f\n", bestLeftover)
		}
	} else {
		fmt.Println("\nНе удалось рассчитать покупку: нет подходящих котировок.")
	}

	// 6) (опционально) сохранить результат на диск
	// _ = saveResultsToFile(allResults, "results.json")

	return nil
}

func printOrderBookSummary(ob *domain.OrderBook, showDetails bool) {
	fmt.Printf("\n=== %s - %s ===\n", ob.Exchange, ob.Symbol)

	ts := ob.Timestamp
	var t time.Time
	switch {
	case ts > 1e12: // миллисекунды
		t = time.UnixMilli(ts)
	default: // секунды
		t = time.Unix(ts, 0)
	}
	fmt.Printf("Время: %s\n", t.Format("15:04 02.01.2006"))

	if showDetails {
		fmt.Println("Аски (TOP 3):")
		for i := 0; i < 3 && i < len(ob.Asks); i++ {
			fmt.Printf("  %s - %s\n", ob.Asks[i].Price, ob.Asks[i].Quantity)
		}
		fmt.Println("Биды (TOP 3):")
		for i := 0; i < 3 && i < len(ob.Bids); i++ {
			fmt.Printf("  %s - %s\n", ob.Bids[i].Price, ob.Bids[i].Quantity)
		}
	} else {
		fmt.Printf("Аски: %d, Биды: %d\n", len(ob.Asks), len(ob.Bids))
		if len(ob.Asks) > 0 && len(ob.Bids) > 0 {
			fmt.Printf("Ask=%s, Bid=%s\n", ob.Asks[0].Price, ob.Bids[0].Price)
		}
	}
}

func saveResultsToFile(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, jsonData, 0644)
}

// buyQtyFromAsks рассчитывает, сколько целевой монеты можно купить
// на сумму usdt, проходясь по аскам сверху вниз (минимальная цена → выше).
// Возвращает количество и среднюю цену покупки (USDT за 1 монету).
func buyQtyFromAsks(asks []domain.Order, usdt float64) (qty float64, avgPrice float64) {
	if usdt <= 0 || len(asks) == 0 {
		return 0, 0
	}

	var spent float64
	for _, a := range asks {
		p, err1 := strconv.ParseFloat(a.Price, 64)
		q, err2 := strconv.ParseFloat(a.Quantity, 64)
		if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
			continue
		}
		maxSpendHere := p * q // сколько USDT нужно, чтобы взять весь уровень
		remain := usdt - spent
		if remain <= 0 {
			break
		}
		if remain >= maxSpendHere {
			// забираем весь уровень
			qty += q
			spent += maxSpendHere
		} else {
			// берем частично
			partialQty := remain / p
			qty += partialQty
			spent += remain
			break
		}
	}
	if qty <= 0 {
		return 0, 0
	}
	avgPrice = spent / qty
	return qty, avgPrice
}
