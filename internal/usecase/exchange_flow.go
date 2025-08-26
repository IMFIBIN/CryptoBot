package usecase

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
// 4) общий объединённый стакан TOP-10;
// 5) сценарий #1: лучший обмен на всю сумму;
// 6) сценарий #2: равное распределение суммы по биржам;
// 7) итоговое сравнение сценариев.
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

	// 4.1) Общий объединённый стакан TOP-10
	combinedAsks, combinedBids := buildCombinedOrderBook(allResults, symbol)
	printCombinedTop(symbol, combinedAsks, combinedBids, 10)

	// форматтер чисел (разделители разрядов и запятая как десятичный)
	pr := message.NewPrinter(language.Russian)

	// 5) Сценарий #1: лучший обмен на всю сумму
	fmt.Printf("\n=== Расчёт покупки %s на сумму %.2f USDT ===\n", right, params.LeftCoinVolume)
	type quote struct {
		exName   string
		qty      float64
		avgPrice float64
	}
	var best *quote

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

		// Остаток USDT после покупки на этой бирже (для справки)
		spent := avgPrice * qty
		leftover := params.LeftCoinVolume - spent
		if leftover > 0 {
			pr.Printf("  Остаток USDT: %0.2f\n", leftover)
		}

		if best == nil || avgPrice < best.avgPrice {
			best = &quote{exName: exName, qty: qty, avgPrice: avgPrice}
		}
	}

	var scen1Qty, scen1Spent, scen1Avg float64
	var scen1Leftover float64

	if best != nil {
		fmt.Printf("\nЛучший обмен: %s → %.8f %s по средней цене ~ %.8f USDT\n",
			best.exName, best.qty, right, best.avgPrice)

		scen1Qty = best.qty
		scen1Spent = best.avgPrice * best.qty
		scen1Avg = best.avgPrice
		scen1Leftover = params.LeftCoinVolume - scen1Spent

		if scen1Leftover > 0 {
			pr.Printf("Остаток USDT: %0.2f\n", scen1Leftover)
		}
	} else {
		fmt.Println("\nНе удалось рассчитать покупку: нет подходящих котировок.")
	}

	// 6) Сценарий #2: равное распределение суммы по биржам
	fmt.Printf("\n=== Равное распределение суммы %.2f USDT между доступными биржами ===\n", params.LeftCoinVolume)

	// Определим участвующие биржи (есть стакан с асками по символу)
	participating := make([]string, 0, len(allResults))
	for exName, books := range allResults {
		if ob, ok := books[symbol]; ok && ob != nil && len(ob.Asks) > 0 {
			participating = append(participating, exName)
		}
	}

	var scen2Qty, scen2Spent, scen2Avg, scen2Leftover float64

	if len(participating) == 0 {
		fmt.Println("Нет бирж с ликвидностью для равного распределения.")
	} else {
		split := params.LeftCoinVolume / float64(len(participating))

		for _, exName := range participating {
			ob := allResults[exName][symbol]
			qty, avgPrice := buyQtyFromAsks(ob.Asks, split)
			if qty <= 0 {
				fmt.Printf("%s: недостаточная ликвидность по %s на сумму %.2f USDT\n", exName, symbol, split)
				pr.Printf("  Остаток USDT: %0.2f\n", split)
				scen2Leftover += split
				continue
			}
			fmt.Printf("%s: на %.2f USDT → получим ~ %.8f %s, средняя цена ~ %.8f USDT\n", exName, split, qty, right, avgPrice)
			spent := avgPrice * qty
			leftover := split - spent
			if leftover > 0 {
				pr.Printf("  Остаток USDT: %0.2f\n", leftover)
			}
			scen2Qty += qty
			scen2Spent += spent
			scen2Leftover += leftover
		}

		if scen2Qty > 0 {
			scen2Avg = scen2Spent / scen2Qty
			fmt.Printf("\nИтого (равное распределение): получим ~ %.8f %s, средняя цена ~ %.8f USDT\n", scen2Qty, right, scen2Avg)
			pr.Printf("Итого потрачено: %0.2f USDT\n", scen2Spent)
			if scen2Leftover > 0 {
				pr.Printf("Итого остаток USDT: %0.2f\n", scen2Leftover)
			}
		} else {
			fmt.Println("\nИтого (равное распределение): не удалось купить — нет ликвидности.")
		}
	}

	// 7) Итоговое сравнение сценариев
	fmt.Println("\n=== Итоговое сравнение сценариев ===")
	if scen1Qty > 0 && scen2Qty > 0 {
		// По количеству купленной монеты
		if scen1Qty > scen2Qty {
			diffQty := scen1Qty - scen2Qty
			pct := (diffQty / scen2Qty) * 100.0
			fmt.Printf("Больше монет получаем по сценарию #1 (лучший обмен на всю сумму): +%.8f %s (≈ +%.2f%%)\n", diffQty, right, pct)
		} else if scen2Qty > scen1Qty {
			diffQty := scen2Qty - scen1Qty
			pct := (diffQty / scen1Qty) * 100.0
			fmt.Printf("Больше монет получаем по сценарию #2 (равное распределение): +%.8f %s (≈ +%.2f%%)\n", diffQty, right, pct)
		} else {
			fmt.Println("Оба сценария дают одинаковое количество целевой монеты.")
		}

		// По средней цене (ниже — лучше)
		if scen1Avg > 0 && scen2Avg > 0 {
			if scen1Avg < scen2Avg {
				diff := scen2Avg - scen1Avg
				pct := (diff / scen2Avg) * 100.0
				fmt.Printf("Средняя цена ниже в сценарии #1: −%.8f USDT за 1 %s (≈ −%.2f%%)\n", diff, right, pct)
			} else if scen2Avg < scen1Avg {
				diff := scen1Avg - scen2Avg
				pct := (diff / scen1Avg) * 100.0
				fmt.Printf("Средняя цена ниже в сценарии #2: −%.8f USDT за 1 %s (≈ −%.2f%%)\n", diff, right, pct)
			} else {
				fmt.Println("Средняя цена одинаковая в обоих сценариях.")
			}
		}

		// На всякий случай отразим неиспользованный остаток
		if scen1Leftover > 0 || scen2Leftover > 0 {
			pr.Printf("Остатки USDT — сценарий #1: %0.2f, сценарий #2: %0.2f\n", scen1Leftover, scen2Leftover)
		}
	} else {
		fmt.Println("Недостаточно данных для сравнения: один из сценариев не смог совершить покупку.")
	}

	// (опционально) сохранить результат
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

// ===== Объединение стаканов и печать TOP-10 =====

type combinedLevel struct {
	Exchange string
	Price    float64
	Qty      float64
	rawPrice string
	rawQty   string
}

func buildCombinedOrderBook(all map[string]map[string]*domain.OrderBook, symbol string) (asks []combinedLevel, bids []combinedLevel) {
	for exName, books := range all {
		ob, ok := books[symbol]
		if !ok || ob == nil {
			continue
		}
		for _, a := range ob.Asks {
			p, err1 := strconv.ParseFloat(a.Price, 64)
			q, err2 := strconv.ParseFloat(a.Quantity, 64)
			if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
				continue
			}
			asks = append(asks, combinedLevel{
				Exchange: exName,
				Price:    p,
				Qty:      q,
				rawPrice: a.Price,
				rawQty:   a.Quantity,
			})
		}
		for _, b := range ob.Bids {
			p, err1 := strconv.ParseFloat(b.Price, 64)
			q, err2 := strconv.ParseFloat(b.Quantity, 64)
			if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
				continue
			}
			bids = append(bids, combinedLevel{
				Exchange: exName,
				Price:    p,
				Qty:      q,
				rawPrice: b.Price,
				rawQty:   b.Quantity,
			})
		}
	}

	// asks — по возрастанию (дешевле лучше), bids — по убыванию (дороже лучше)
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })

	return asks, bids
}

func printCombinedTop(symbol string, asks, bids []combinedLevel, topN int) {
	fmt.Printf("\n=== Общий объединённый стакан по %s (TOP-%d) ===\n", symbol, topN)

	fmt.Println("Аски (лучшие цены вверх):")
	for i := 0; i < len(asks) && i < topN; i++ {
		al := asks[i]
		fmt.Printf("  %s - %s (%s)\n", stripTrailingZeros(al.rawPrice), stripTrailingZeros(al.rawQty), al.Exchange)
	}

	fmt.Println("Биды (лучшие цены вверх):")
	for i := 0; i < len(bids) && i < topN; i++ {
		bl := bids[i]
		fmt.Printf("  %s - %s (%s)\n", stripTrailingZeros(bl.rawPrice), stripTrailingZeros(bl.rawQty), bl.Exchange)
	}
}

func stripTrailingZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	if strings.HasSuffix(s, ".") {
		return s + "0"
	}
	return s
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
