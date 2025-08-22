package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/adshao/go-binance/v2"
)

// Общие структуры данных
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

// Bybit API Response Structures
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
		client:   &http.Client{Timeout: 15 * time.Second},
		config:   config,
		category: category,
	}
}

func (b *BybitHTTPClient) makeRequest(url string) ([]byte, error) {
	resp, err := b.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// Bybit implementation with HTTP
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
	for _, symbol := range response.Result.List {
		if symbol.Status == "Trading" {
			symbols = append(symbols, symbol.Symbol)
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

	return b.convertOrderBook(symbol, &response), nil
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

func (b *BybitExchange) convertOrderBook(symbol string, response *BybitOrderBookResponse) *OrderBook {
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

	return ob
}

// Binance implementation
type BinanceExchange struct {
	client *binance.Client
	config Config
}

func NewBinanceExchange(config Config) *BinanceExchange {
	client := binance.NewClient("", "") // публичные данные
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
	for _, symbol := range exchangeInfo.Symbols {
		if symbol.Status == "TRADING" {
			symbols = append(symbols, symbol.Symbol)
		}
	}

	return symbols, nil
}

func (b *BinanceExchange) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	binanceOrderBook, err := b.client.NewDepthService().
		Symbol(symbol).
		Limit(limit).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance: ошибка стакана для %s: %w", symbol, err)
	}

	return b.convertOrderBook(symbol, binanceOrderBook), nil
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

func (b *BinanceExchange) convertOrderBook(symbol string, binanceOB *binance.DepthResponse) *OrderBook {
	ob := &OrderBook{
		Symbol:    symbol,
		Exchange:  b.Name(),
		Timestamp: time.Now().UnixMilli(),
	}

	for _, ask := range binanceOB.Asks {
		ob.Asks = append(ob.Asks, Order{Price: ask.Price, Quantity: ask.Quantity})
	}

	for _, bid := range binanceOB.Bids {
		ob.Bids = append(ob.Bids, Order{Price: bid.Price, Quantity: bid.Quantity})
	}

	return ob
}

// ExchangeManager управляет несколькими биржами
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

// Утилитные функции
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

func main() {
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

		// Получаем стаканы для общих символов
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

		// Выводим краткую информацию
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
