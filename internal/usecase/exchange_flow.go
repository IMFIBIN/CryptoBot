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
)

// Run — основной сценарий:
// 1) интерактивный ввод (выбор BUY/SELL);
// 2) сбор стаканов с бирж;
// 3) печать сводок и сравнение (best ask/bid по биржам);
// 4) сценарий #1: самая выгодная одна биржа;
// 5) сценарий #2: равное распределение по биржам;
// 6) сценарий #3: оптимальное распределение по всем биржам;
// 7) итоговое сравнение сценариев (для выбранного направления).
func Run(cfg domain.Config, exchanges []domain.Exchange) error {
	// 1) Ввод
	params := cli.GetInteractiveParams()
	fmt.Printf("DEBUG: action=%s, base=%s, target=%s, amount=%.8f\n",
		params.Action, params.LeftCoinName, params.RightCoinName, params.LeftCoinVolume)

	// 2) Символ для стаканов <RIGHT>USDT
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
			printOrderBookSummary(ob)
		}
	}

	// 3) Сравнение (best ask/bid) по биржам
	fmt.Println("\n=== Сравнение цен между биржами ===")
	fmt.Printf("\n%s:\n", symbol)
	for exName, obs := range allResults {
		if ob, ok := obs[symbol]; ok && len(ob.Asks) > 0 && len(ob.Bids) > 0 {
			fmt.Printf("  %s: Ask=%s, Bid=%s\n", exName, ob.Asks[0].Price, ob.Bids[0].Price)
		}
	}

	// Ветки BUY/SELL
	if params.Action == "sell" {
		return runSellFlow(params, allResults, right, symbol)
	}
	return runBuyFlow(params, allResults, right, symbol)
}

// ===================== BUY FLOW =====================

func runBuyFlow(params cli.InputParams, allResults map[string]map[string]*domain.OrderBook, right, symbol string) error {
	amountUSDT := params.LeftCoinVolume

	// 4) Сценарий #1 (лучшая ОДНА биржа)
	fmt.Printf("\n=== Сценарий #1 (BUY): покупка %s на %.2f USDT — одна биржа ===\n", right, amountUSDT)
	type quote struct {
		exName   string
		qty      float64
		avgPrice float64
	}
	var best *quote
	for exName, books := range allResults {
		ob, ok := books[symbol]
		if !ok || ob == nil || len(ob.Asks) == 0 {
			continue
		}
		qty, avgPrice, spent := buyQtyFromAsks(ob.Asks, amountUSDT)
		if qty <= 0 {
			continue
		}
		fmt.Printf("%s → получим ~ %.8f %s, средняя цена ~ %.8f USDT\n", exName, qty, right, avgPrice)
		if spent < amountUSDT {
			fmt.Printf("  Лимит стакана исчерпан: потрачено %.2f USDT, остаток %.2f USDT не использован\n", spent, amountUSDT-spent)
		}
		if best == nil || avgPrice < best.avgPrice {
			best = &quote{exName: exName, qty: qty, avgPrice: avgPrice}
		}
	}
	var scen1Qty, scen1Avg float64
	if best != nil {
		fmt.Printf("Лучший обмен: %s → %.8f %s, avg=%.8f\n", best.exName, best.qty, right, best.avgPrice)
		scen1Qty = best.qty
		scen1Avg = best.avgPrice
	}

	// 5) Сценарий #2 (равное распределение)
	fmt.Printf("\n=== Сценарий #2 (BUY): равное распределение %.2f USDT ===\n", amountUSDT)
	var scen2Qty, scen2Avg float64
	participating := make([]string, 0)
	for exName, books := range allResults {
		if ob, ok := books[symbol]; ok && ob != nil && len(ob.Asks) > 0 {
			participating = append(participating, exName)
		}
	}
	if len(participating) > 0 {
		split := amountUSDT / float64(len(participating))
		var totalSpent float64
		var totalLeftover float64

		for _, exName := range participating {
			ob := allResults[exName][symbol]
			qty, avgPrice, spent := buyQtyFromAsks(ob.Asks, split)
			if qty <= 0 {
				fmt.Printf("%s: недостаточная ликвидность по %s на сумму %.2f USDT\n", exName, symbol, split)
				totalLeftover += split
				continue
			}

			fmt.Printf("%s: на %.2f USDT → %.8f %s, avg=%.8f\n", exName, split, qty, right, avgPrice)
			if spent < split {
				fmt.Printf("  Лимит стакана исчерпан: потрачено %.2f USDT, остаток %.2f USDT не использован\n", spent, split-spent)
				totalLeftover += split - spent
			}

			totalSpent += spent
			scen2Qty += qty
		}

		if scen2Qty > 0 {
			scen2Avg := totalSpent / scen2Qty
			fmt.Printf("Итого (равное): %.8f %s, avg=%.8f\n", scen2Qty, right, scen2Avg)
			fmt.Printf("Итого потрачено: %.2f USDT", totalSpent)
			if totalLeftover > 0 {
				fmt.Printf(", остаток: %.2f USDT", totalLeftover)
			}
			fmt.Println()
		} else {
			fmt.Println("Итого (равное): не удалось купить — нет ликвидности.")
		}
	}

	// 6) Сценарий #3 (оптимальное распределение по всем биржам)
	_, buyTotalQty, buyAvgPrice, _, _ := optimizeBuy(allResults, symbol, amountUSDT)
	var scen3Qty, scen3Avg float64
	if buyTotalQty > 0 {
		// без заголовка и детализации — только итог
		fmt.Printf("Итого: %.8f %s, avg=%.8f\n", buyTotalQty, right, buyAvgPrice)
		scen3Qty = buyTotalQty
		scen3Avg = buyAvgPrice
	}

	// 7) Итоговое сравнение #1/#2/#3
	compareBuyScenarios(right, scen1Qty, scen1Avg, scen2Qty, scen2Avg, scen3Qty, scen3Avg)
	return nil
}

func compareBuyScenarios(right string, scen1Qty, scen1Avg, scen2Qty, scen2Avg, scen3Qty, scen3Avg float64) {
	fmt.Println("\n=== Итоговое сравнение сценариев (BUY) #1, #2 и #3 ===")
	type scen struct {
		id   int
		name string
		qty  float64
		avg  float64
	}
	all := []scen{
		{1, "Сценарий #1 (лучшая биржа)", scen1Qty, scen1Avg},
		{2, "Сценарий #2 (равное)", scen2Qty, scen2Avg},
		{3, "Сценарий #3 (оптимальное)", scen3Qty, scen3Avg},
	}
	valid := make([]scen, 0, 3)
	for _, s := range all {
		if s.qty > 0 {
			valid = append(valid, s)
		}
	}
	if len(valid) < 2 {
		fmt.Println("Недостаточно данных для сравнения.")
		return
	}
	// лучший по количеству
	best := valid[0]
	for _, s := range valid[1:] {
		if s.qty > best.qty {
			best = s
		}
	}
	fmt.Printf("Лучший по количеству: %s — %.8f %s (avg=%.8f)\n", best.name, best.qty, right, best.avg)
	for _, s := range valid {
		if s.id == best.id {
			continue
		}
		diff := best.qty - s.qty
		pct := (diff / s.qty) * 100
		fmt.Printf("  Преимущество над %s: +%.8f %s (≈ %.4f%%)\n", s.name, diff, right, pct)
	}
	// лучшая средняя цена (ниже — лучше)
	bestPrice := valid[0]
	for _, s := range valid[1:] {
		if s.avg > 0 && s.avg < bestPrice.avg {
			bestPrice = s
		}
	}
	if bestPrice.avg > 0 {
		fmt.Printf("Лучшая средняя цена: %s — %.8f USDT\n", bestPrice.name, bestPrice.avg)
	}
}

// ===================== SELL FLOW =====================

func runSellFlow(params cli.InputParams, allResults map[string]map[string]*domain.OrderBook, right, symbol string) error {
	amountCoin := params.LeftCoinVolume // пользователь ввёл количество монеты right для продажи

	// 4) Сценарий #1 (лучшая ОДНА биржа)
	fmt.Printf("\n=== Сценарий #1 (SELL): продажа %.8f %s — одна биржа ===\n", amountCoin, right)
	type quote struct {
		exName   string
		usdt     float64
		avgPrice float64 // средняя цена продажи (USDT за 1 монету)
	}
	var best *quote
	for exName, books := range allResults {
		ob, ok := books[symbol]
		if !ok || ob == nil || len(ob.Bids) == 0 {
			continue
		}
		usdt, avgPrice := sellQtyFromBids(ob.Bids, amountCoin)
		if usdt <= 0 {
			continue
		}
		fmt.Printf("%s → получим ~ %.2f USDT, средняя цена ~ %.8f USDT\n", exName, usdt, avgPrice)
		if best == nil || avgPrice > best.avgPrice { // для продажи выше цена — лучше
			best = &quote{exName: exName, usdt: usdt, avgPrice: avgPrice}
		}
	}
	var scen1USDT, scen1Avg float64
	if best != nil {
		fmt.Printf("Лучший обмен: %s → %.2f USDT, avg=%.8f\n", best.exName, best.usdt, best.avgPrice)
		scen1USDT = best.usdt
		scen1Avg = best.avgPrice
	}

	// 5) Сценарий #2 (равное распределение)
	fmt.Printf("\n=== Сценарий #2 (SELL): равное распределение %.8f %s ===\n", amountCoin, right)
	var scen2USDT, scen2Avg float64
	participating := make([]string, 0)
	for exName, books := range allResults {
		if ob, ok := books[symbol]; ok && ob != nil && len(ob.Bids) > 0 {
			participating = append(participating, exName)
		}
	}
	if len(participating) > 0 {
		split := amountCoin / float64(len(participating))
		var totalSold float64
		for _, exName := range participating {
			ob := allResults[exName][symbol]
			usdt, avgPrice := sellQtyFromBids(ob.Bids, split)
			if usdt <= 0 {
				continue
			}
			fmt.Printf("%s: продаём %.8f %s → %.2f USDT, avg=%.8f\n", exName, split, right, usdt, avgPrice)
			scen2USDT += usdt
			totalSold += split
		}
		if totalSold > 0 {
			// средняя цена продажи = общая выручка / общее проданное количество
			scen2Avg = scen2USDT / totalSold
			fmt.Printf("Итого (равное): %.2f USDT, avg=%.8f\n", scen2USDT, scen2Avg)
		}
	}

	// 6) Сценарий #3 (оптимальное распределение)
	fmt.Printf("\n=== Сценарий #3 (SELL): оптимальное распределение %.8f %s ===\n", amountCoin, right)
	_, totalUSDT, sellAvgPrice, soldQty, _ := optimizeSell(allResults, symbol, amountCoin)
	var scen3USDT, scen3Avg float64
	if soldQty > 0 {
		fmt.Printf("Итого: %.2f USDT, avg=%.8f\n", totalUSDT, sellAvgPrice)
		scen3USDT = totalUSDT
		scen3Avg = sellAvgPrice
	}

	// 7) Итоговое сравнение #1/#2/#3 для SELL (целевая метрика — максимальная выручка)
	compareSellScenarios(right, scen1USDT, scen1Avg, scen2USDT, scen2Avg, scen3USDT, scen3Avg, amountCoin)
	return nil
}

func compareSellScenarios(right string, scen1USDT, scen1Avg, scen2USDT, scen2Avg, scen3USDT, scen3Avg, totalQty float64) {
	fmt.Println("\n=== Итоговое сравнение сценариев (SELL) #1, #2 и #3 ===")
	type scen struct {
		id   int
		name string
		usd  float64 // сколько USDT получили (чем больше — тем лучше)
		avg  float64 // средняя цена продажи (чем выше — тем лучше)
	}
	all := []scen{
		{1, "Сценарий #1 (лучшая биржа)", scen1USDT, scen1Avg},
		{2, "Сценарий #2 (равное)", scen2USDT, scen2Avg},
		{3, "Сценарий #3 (оптимальное)", scen3USDT, scen3Avg},
	}
	valid := make([]scen, 0, 3)
	for _, s := range all {
		if s.usd > 0 {
			valid = append(valid, s)
		}
	}
	if len(valid) < 2 {
		fmt.Println("Недостаточно данных для сравнения.")
		return
	}
	// лучший по выручке
	best := valid[0]
	for _, s := range valid[1:] {
		if s.usd > best.usd {
			best = s
		}
	}
	fmt.Printf("Лучшая выручка: %s — %.2f USDT (avg=%.8f)\n", best.name, best.usd, best.avg)
	for _, s := range valid {
		if s.id == best.id {
			continue
		}
		diff := best.usd - s.usd
		pct := (diff / s.usd) * 100
		fmt.Printf("  Преимущество над %s: +%.2f USDT (≈ %.4f%%)\n", s.name, diff, pct)
	}
	// лучшая средняя цена продажи
	bestPrice := valid[0]
	for _, s := range valid[1:] {
		if s.avg > bestPrice.avg {
			bestPrice = s
		}
	}
	if bestPrice.avg > 0 {
		fmt.Printf("Лучшая средняя цена продажи: %s — %.8f USDT за 1 %s\n", bestPrice.name, bestPrice.avg, right)
	}
}

// ===================== Общие алгоритмы =====================

type tradeStep struct {
	Exchange string
	Price    float64
	Qty      float64 // куплено/продано монет
	Usdt     float64 // потрачено (buy) / получено (sell)
}

// BUY: объединяем asks по возрастанию цены и идём сверху
func optimizeBuy(all map[string]map[string]*domain.OrderBook, symbol string, amountUSDT float64) (steps []tradeStep, totalQty, avgPrice, totalSpent, leftover float64) {
	if amountUSDT <= 0 {
		return nil, 0, 0, 0, 0
	}
	levels := combinedAsks(all, symbol)
	remain := amountUSDT

	for _, lv := range levels {
		if remain <= 0 {
			break
		}
		maxSpend := lv.Price * lv.Qty
		if maxSpend <= 0 {
			continue
		}
		if remain >= maxSpend {
			steps = append(steps, tradeStep{lv.Exchange, lv.Price, lv.Qty, maxSpend})
			totalQty += lv.Qty
			totalSpent += maxSpend
			remain -= maxSpend
		} else {
			q := remain / lv.Price
			steps = append(steps, tradeStep{lv.Exchange, lv.Price, q, remain})
			totalQty += q
			totalSpent += remain
			remain = 0
			break
		}
	}
	if totalQty > 0 {
		avgPrice = totalSpent / totalQty
	}
	leftover = remain
	return
}

// SELL: объединяем bids по убыванию цены и идём сверху
func optimizeSell(all map[string]map[string]*domain.OrderBook, symbol string, amountCoin float64) (steps []tradeStep, totalUSDT, avgPrice, totalSold, leftover float64) {
	if amountCoin <= 0 {
		return nil, 0, 0, 0, 0
	}
	levels := combinedBids(all, symbol)
	remain := amountCoin

	for _, lv := range levels {
		if remain <= 0 {
			break
		}
		q := lv.Qty
		if q <= 0 {
			continue
		}
		if remain >= q {
			usdt := q * lv.Price
			steps = append(steps, tradeStep{lv.Exchange, lv.Price, q, usdt})
			totalUSDT += usdt
			totalSold += q
			remain -= q
		} else {
			usdt := remain * lv.Price
			steps = append(steps, tradeStep{lv.Exchange, lv.Price, remain, usdt})
			totalUSDT += usdt
			totalSold += remain
			remain = 0
			break
		}
	}
	if totalSold > 0 {
		avgPrice = totalUSDT / totalSold
	}
	leftover = remain
	return
}

// ===== Вспомогательные: объединение уровней =====

type combinedLevel struct {
	Exchange string
	Price    float64
	Qty      float64
}

func combinedAsks(all map[string]map[string]*domain.OrderBook, symbol string) []combinedLevel {
	var asks []combinedLevel
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
			asks = append(asks, combinedLevel{exName, p, q})
		}
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price }) // дешевле лучше
	return asks
}

func combinedBids(all map[string]map[string]*domain.OrderBook, symbol string) []combinedLevel {
	var bids []combinedLevel
	for exName, books := range all {
		ob, ok := books[symbol]
		if !ok || ob == nil {
			continue
		}
		for _, b := range ob.Bids {
			p, err1 := strconv.ParseFloat(b.Price, 64)
			q, err2 := strconv.ParseFloat(b.Quantity, 64)
			if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
				continue
			}
			bids = append(bids, combinedLevel{exName, p, q})
		}
	}
	sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price }) // дороже лучше
	return bids
}

// ===== Расчётные утилиты для сценариев #1 и #2 =====

// buyQtyFromAsks рассчитывает, сколько целевой монеты можно купить
// на сумму usdt. Возвращает: количество, среднюю цену и реально потраченное USDT.
func buyQtyFromAsks(asks []domain.Order, usdt float64) (qty, avgPrice, spent float64) {
	if usdt <= 0 || len(asks) == 0 {
		return 0, 0, 0
	}

	for _, a := range asks {
		p, err1 := strconv.ParseFloat(a.Price, 64)
		q, err2 := strconv.ParseFloat(a.Quantity, 64)
		if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
			continue
		}
		remain := usdt - spent
		if remain <= 0 {
			break
		}
		maxSpend := p * q
		if remain >= maxSpend {
			qty += q
			spent += maxSpend
		} else {
			part := remain / p
			qty += part
			spent += remain
			break
		}
	}
	if qty > 0 {
		avgPrice = spent / qty
	}
	return qty, avgPrice, spent
}

// sellQtyFromBids — сколько USDT можно выручить, продав qty монеты на ОДНОЙ бирже
func sellQtyFromBids(bids []domain.Order, qty float64) (usdt float64, avgPrice float64) {
	if qty <= 0 {
		return 0, 0
	}
	var sold float64
	for _, b := range bids {
		p, err1 := strconv.ParseFloat(b.Price, 64)
		q, err2 := strconv.ParseFloat(b.Quantity, 64)
		if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
			continue
		}
		remain := qty - sold
		if remain <= 0 {
			break
		}
		if remain >= q {
			usdt += q * p
			sold += q
		} else {
			usdt += remain * p
			sold += remain
			break
		}
	}
	if sold > 0 {
		avgPrice = usdt / sold
	}
	return usdt, avgPrice
}

// ===== Краткая сводка по бирже (без TOP-3) =====

func printOrderBookSummary(ob *domain.OrderBook) {
	fmt.Printf("\n=== %s - %s ===\n", ob.Exchange, ob.Symbol)
	ts := ob.Timestamp
	var t time.Time
	if ts > 1e12 {
		t = time.UnixMilli(ts)
	} else {
		t = time.Unix(ts, 0)
	}
	fmt.Printf("Время: %s\n", t.Format("15:04 02.01.2006"))
	if len(ob.Asks) > 0 && len(ob.Bids) > 0 {
		fmt.Printf("Ask=%s, Bid=%s\n", ob.Asks[0].Price, ob.Bids[0].Price)
	}
}

// ===== Служебное =====

func saveResultsToFile(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, jsonData, 0644)
}
