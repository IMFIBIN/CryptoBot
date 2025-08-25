package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2"
)

// ================== Общие структуры данных ==================

type OrderBook struct {
	Symbol    string
	Exchange  string
	Timestamp int64
	Asks      []Order
	Bids      []Order
}

type Order struct {
	Price    string
	Quantity string
}

type Exchange interface {
	Name() string
	GetSymbols() ([]string, error)
	GetOrderBook(symbol string, limit int) (*OrderBook, error)
	GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*OrderBook, error)
}

// Конфигурация
type Config struct {
	DelayMS int `json:"delay_ms"`
	Limit   int `json:"limit"`
}

// ================== Интерактивный ввод (Действие 1) ==================

type InputParams struct {
	LeftCoinName   string
	LeftCoinChain  string
	RightCoinName  string
	RightCoinChain string
	LeftCoinVolume float64
}

// Форматирование числа по-русски: 100.000.000,0
func formatFloatRU(v float64, decimals int) string {
	s := fmt.Sprintf("%.*f", decimals, v) // "100000000.0"
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	// расставляем точки в целой части
	var out []byte
	cnt := 0
	for i := len(intPart) - 1; i >= 0; i-- {
		out = append(out, intPart[i])
		cnt++
		if cnt%3 == 0 && i != 0 {
			out = append(out, '.')
		}
	}
	// разворот
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if decimals == 0 {
		return string(out)
	}
	return string(out) + "," + frac
}

func getInteractiveParams() InputParams {
	reader := bufio.NewReader(os.Stdin)

	// 1) Выбор базовой валюты (пока только USDT; оставлено место для расширения)
	fmt.Println("Выберите валюту, которой платите:")
	fmt.Println("1) USDT  (пока доступна)")
	fmt.Print("Ваш выбор [1] (Enter = USDT): ")
	baseRaw, _ := reader.ReadString('\n')
	baseRaw = strings.TrimSpace(baseRaw)

	base := "USDT"
	if baseRaw == "1" || baseRaw == "" {
		base = "USDT"
	} else {
		// зарезервировано для будущих валют
		base = "USDT"
	}

	// 2) Куда меняем USDT — 5 монет
	coins := []string{"BTC", "ETH", "BNB", "ADA", "SOL"}
	fmt.Println("\nНа какую монету хотите обменять USDT?")
	for i, c := range coins {
		fmt.Printf("%d) %s\n", i+1, c)
	}
	fmt.Print("Ваш выбор [1-5] (Enter = BTC): ")
	choiceRaw, _ := reader.ReadString('\n')
	choiceRaw = strings.TrimSpace(choiceRaw)

	choice := 1 // по умолчанию BTC
	if choiceRaw != "" {
		if n, err := strconv.Atoi(choiceRaw); err == nil && n >= 1 && n <= len(coins) {
			choice = n
		}
	}
	right := coins[choice-1]

	// 3) Сколько базовой валюты (USDT) — с красивым форматом
	defAmount := 100_000_000.0
	fmt.Printf("\nСколько у вас %s? (Enter = %s): ", base, formatFloatRU(defAmount, 1))
	amountRaw, _ := reader.ReadString('\n')
	amountRaw = strings.TrimSpace(amountRaw)

	amount := defAmount
	if amountRaw != "" {
		// позволяем ввод "100.000.000,0" / "100000000" / "100,5" и т.п.
		normalized := strings.ReplaceAll(amountRaw, " ", "")
		normalized = strings.ReplaceAll(normalized, ".", "")
		normalized = strings.ReplaceAll(normalized, ",", ".")
		if v, err := strconv.ParseFloat(normalized, 64); err == nil && v > 0 {
			amount = v
		}
	}

	params := InputParams{
		LeftCoinName:   base,
		LeftCoinChain:  "SPOT",
		RightCoinName:  right,
		RightCoinChain: "SPOT",
		LeftCoinVolume: amount,
	}

	// подтверждение выбора — красивый формат
	fmt.Printf("\nВы выбрали: платить %s и купить %s\n", params.LeftCoinName, params.RightCoinName)
	fmt.Printf("Доступно %s: %s\n", params.LeftCoinName, formatFloatRU(params.LeftCoinVolume, 2))

	return params
}

// ================== Bybit API ==================

type BybitSymbolResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []BybitSymbol `json:"list"`
	} `json:"result"`
}

type BybitSymbol struct {
	Symbol    string `json:"symbol"`
	BaseCoin  string `json:"baseCoin"`
	QuoteCoin string `json:"quoteCoin"`
	Status    string `json:"status"`
}

type BybitOrderBookResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Symbol string     `json:"s"`
		Bids   [][]string `json:"b"`
		Asks   [][]string `json:"a"`
		Ts     int64      `json:"ts"`
	} `json:"result"`
}

// HTTP клиент для Bybit
type BybitHTTPClient struct {
	baseURL  string
	client   *http.Client
	config   Config
	category string
}

func NewBybitHTTPClient(config Config, category string) *BybitHTTPClient {
	return &BybitHTTPClient{
		baseURL:  "https://api.bybit.com",
		client:   &http.Client{Timeout: 12 * time.Second},
		config:   config,
		category: category,
	}
}

// общий ретраер с простым экспоненциальным бэкоффом
func withRetry(attempts int, sleep time.Duration, op func() error) error {
	var err error
	backoff := sleep
	for i := 0; i < attempts; i++ {
		if err = op(); err == nil {
			return nil
		}
		time.Sleep(backoff)
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
	return err
}

func (b *BybitHTTPClient) makeRequest(url string) ([]byte, error) {
	var respBody []byte
	err := withRetry(3, 1*time.Second, func() error {
		resp, err := b.client.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP error: %s", resp.Status)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		respBody = body
		return nil
	})
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

type BybitExchange struct {
	httpClient *BybitHTTPClient
	config     Config
}

func NewBybitExchange(config Config, category string) *BybitExchange {
	return &BybitExchange{
		httpClient: NewBybitHTTPClient(config, category),
		config:     config,
	}
}

func (b *BybitExchange) Name() string {
	return "Bybit"
}

func (b *BybitExchange) GetSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/v5/market/instruments-info?category=spot", b.httpClient.baseURL)
	data, err := b.httpClient.makeRequest(url)
	if err != nil {
		return nil, fmt.Errorf("bybit: ошибка запроса: %w", err)
	}
	var response BybitSymbolResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("bybit: ошибка парсинга JSON: %w", err)
	}
	if response.RetCode != 0 {
		return nil, fmt.Errorf("bybit: API error: %s", response.RetMsg)
	}
	var symbols []string
	for _, s := range response.Result.List {
		if s.Status == "Trading" {
			symbols = append(symbols, s.Symbol)
		}
	}
	return symbols, nil
}

func (b *BybitExchange) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	url := fmt.Sprintf("%s/v5/market/orderbook?category=spot&symbol=%s&limit=%d",
		b.httpClient.baseURL, symbol, limit)
	data, err := b.httpClient.makeRequest(url)
	if err != nil {
		return nil, fmt.Errorf("bybit: ошибка запроса стакана: %w", err)
	}
	var response BybitOrderBookResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("bybit: ошибка парсинга стакана: %w", err)
	}
	if response.RetCode != 0 {
		return nil, fmt.Errorf("bybit: API error: %s", response.RetMsg)
	}
	ob := &OrderBook{
		Symbol:    symbol,
		Exchange:  b.Name(),
		Timestamp: response.Result.Ts,
	}
	for _, ask := range response.Result.Asks {
		if len(ask) >= 2 {
			ob.Asks = append(ob.Asks, Order{Price: ask[0], Quantity: ask[1]})
		}
	}
	for _, bid := range response.Result.Bids {
		if len(bid) >= 2 {
			ob.Bids = append(ob.Bids, Order{Price: bid[0], Quantity: bid[1]})
		}
	}
	return ob, nil
}

func (b *BybitExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*OrderBook, error) {
	result := make(map[string]*OrderBook)
	for _, symbol := range symbols {
		orderBook, err := b.GetOrderBook(symbol, limit)
		if err != nil {
			log.Printf("Bybit: ошибка для %s: %v", symbol, err)
			continue
		}
		result[symbol] = orderBook
		time.Sleep(delay)
	}
	return result, nil
}

// ================== Binance API ==================

type BinanceExchange struct {
	client *binance.Client
	config Config
}

func NewBinanceExchange(config Config) *BinanceExchange {
	client := binance.NewClient("", "") // публичные данные
	// важный таймаут HTTP‑клиента
	client.HTTPClient = &http.Client{Timeout: 12 * time.Second}
	return &BinanceExchange{
		client: client,
		config: config,
	}
}

func (b *BinanceExchange) Name() string {
	return "Binance"
}

func (b *BinanceExchange) GetSymbols() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	exchangeInfo, err := b.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance: ошибка получения информации: %w", err)
	}
	var symbols []string
	for _, s := range exchangeInfo.Symbols {
		if s.Status == "TRADING" {
			symbols = append(symbols, s.Symbol)
		}
	}
	return symbols, nil
}

func (b *BinanceExchange) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	var depth *binance.DepthResponse
	// ретраи DepthService с контекстом-таймаутом
	err := withRetry(3, 1*time.Second, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		depth, err = b.client.NewDepthService().Symbol(symbol).Limit(limit).Do(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("binance: ошибка стакана для %s: %w", symbol, err)
	}

	ob := &OrderBook{
		Symbol:    symbol,
		Exchange:  b.Name(),
		Timestamp: time.Now().UnixMilli(),
	}
	for _, a := range depth.Asks {
		ob.Asks = append(ob.Asks, Order{Price: a.Price, Quantity: a.Quantity})
	}
	for _, d := range depth.Bids {
		ob.Bids = append(ob.Bids, Order{Price: d.Price, Quantity: d.Quantity})
	}
	return ob, nil
}

func (b *BinanceExchange) GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*OrderBook, error) {
	result := make(map[string]*OrderBook)

	for _, symbol := range symbols {
		orderBook, err := b.GetOrderBook(symbol, limit)
		if err != nil {
			log.Printf("Binance: ошибка для %s: %v", symbol, err)
			continue
		}

		result[symbol] = orderBook
		time.Sleep(delay)
	}

	return result, nil
}

// ================== ExchangeManager ==================

type ExchangeManager struct {
	exchanges []Exchange
	config    Config
}

func NewExchangeManager(config Config) *ExchangeManager {
	return &ExchangeManager{
		config: config,
		exchanges: []Exchange{
			NewBinanceExchange(config),
			NewBybitExchange(config, "spot"),
		},
	}
}

func (em *ExchangeManager) AddExchange(exchange Exchange) {
	em.exchanges = append(em.exchanges, exchange)
}

func (em *ExchangeManager) GetExchangeNames() []string {
	var names []string
	for _, exchange := range em.exchanges {
		names = append(names, exchange.Name())
	}
	return names
}

// ================== Утилиты печати ==================

func printOrderBookSummary(ob *OrderBook, showDetails bool) {
	fmt.Printf("\n=== %s - %s ===\n", ob.Exchange, ob.Symbol)
	fmt.Printf("Время: %d\n", ob.Timestamp)

	if showDetails {
		fmt.Printf("Аски (TOP 3):\n")
		for i := 0; i < 3 && i < len(ob.Asks); i++ {
			fmt.Printf("  %s - %s\n", ob.Asks[i].Price, ob.Asks[i].Quantity)
		}
		fmt.Printf("Биды (TOP 3):\n")
		for i := 0; i < 3 && i < len(ob.Bids); i++ {
			fmt.Printf("  %s - %s\n", ob.Bids[i].Price, ob.Bids[i].Quantity)
		}
	} else {
		fmt.Printf("Аски: %d записей, Биды: %d записей\n", len(ob.Asks), len(ob.Bids))
		if len(ob.Asks) > 0 && len(ob.Bids) > 0 {
			fmt.Printf("Спред: %s\n", calculateSpread(ob.Asks[0].Price, ob.Bids[0].Price))
		}
	}
}

func calculateSpread(askPrice, bidPrice string) string {
	ask := parseFloat(askPrice)
	bid := parseFloat(bidPrice)
	if ask == 0 {
		return "N/A"
	}
	spreadPercent := (ask - bid) / ask * 100
	return fmt.Sprintf("%.4f%%", spreadPercent)
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func saveResultsToFile(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, jsonData, 0644)
}

// ================== main ==================

func main() {
	// Интерактивный ввод
	params := getInteractiveParams()
	fmt.Printf("DEBUG: base=%s, target=%s, amount=%s\n",
		params.LeftCoinName, params.RightCoinName, formatFloatRU(params.LeftCoinVolume, 2))

	// Остальной код — без функциональных изменений
	config := Config{
		DelayMS: 100,
		Limit:   10,
	}
	manager := NewExchangeManager(config)

	fmt.Println("=== Крипто-биржи Монитор ===")
	fmt.Printf("Доступные биржи: %v\n", manager.GetExchangeNames())

	// Общие символы для сравнения
	commonSymbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "ADAUSDT", "SOLUSDT"}

	// Собираем данные со всех бирж
	allResults := make(map[string]map[string]*OrderBook)

	for _, exchange := range manager.exchanges {
		fmt.Printf("\n=== Работа с %s ===\n", exchange.Name())

		orderBooks, err := exchange.GetMultipleOrderBooks(
			commonSymbols,
			config.Limit,
			time.Duration(config.DelayMS)*time.Millisecond,
		)
		if err != nil {
			log.Printf("Ошибка получения данных с %s: %v", exchange.Name(), err)
			continue
		}

		allResults[exchange.Name()] = orderBooks

		fmt.Printf("Успешно получено стаканов: %d\n", len(orderBooks))
		for _, ob := range orderBooks {
			printOrderBookSummary(ob, false)
		}
	}

	// Сравниваем цены между биржами
	fmt.Println("\n=== Сравнение цен между биржами ===")
	for _, symbol := range commonSymbols {
		fmt.Printf("\n%s:\n", symbol)
		for exchangeName, orderBooks := range allResults {
			if ob, exists := orderBooks[symbol]; exists && len(ob.Asks) > 0 && len(ob.Bids) > 0 {
				fmt.Printf("  %s: Ask=%s, Bid=%s\n",
					exchangeName, ob.Asks[0].Price, ob.Bids[0].Price)
			}
		}
	}

	// Детальный просмотр конкретного символа
	fmt.Println("\n=== Детальная информация по BTCUSDT ===")
	for _, orderBooks := range allResults {
		if ob, exists := orderBooks["BTCUSDT"]; exists {
			printOrderBookSummary(ob, true)
		}
	}
}
