package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2"
)

// Конфигурация с API ключами
type Config struct {
	DelayMS       int
	Limit         int
	BinanceApiKey string
	BinanceSecret string
	BybitApiKey   string
	BybitSecret   string
}

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
	GetFilteredSymbols() ([]string, error)
	GetOrderBook(symbol string, limit int) (*OrderBook, error)
	GetMultipleOrderBooks(symbols []string, limit int, delay time.Duration) (map[string]*OrderBook, error)
}

// Bybit API Structures
type BybitSymbolResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []BybitSymbol `json:"list"`
	} `json:"result"`
}

type BybitSymbol struct {
	Symbol      string `json:"symbol"`
	Status      string `json:"status"`
	MinOrderAmt string `json:"minOrderAmt"`
	BaseCoin    string `json:"baseCoin"`
	QuoteCoin   string `json:"quoteCoin"`
}

type BybitCoinInfoResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Rows []BybitCoinInfo `json:"rows"`
	} `json:"result"`
}

type BybitCoinInfo struct {
	Coin             string `json:"coin"`
	DepositStatus    int    `json:"depositStatus"`
	WithdrawalStatus int    `json:"withdrawalStatus"`
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

// Binance API Structures
type BinanceCoinInfo struct {
	Coin              string `json:"coin"`
	DepositAllEnable  bool   `json:"depositAllEnable"`
	WithdrawAllEnable bool   `json:"withdrawAllEnable"`
}

// HTTP клиент для Bybit с авторизацией
type BybitHTTPClient struct {
	baseURL   string
	client    *http.Client
	config    Config
	apiKey    string
	secretKey string
}

func NewBybitHTTPClient(config Config) *BybitHTTPClient {
	return &BybitHTTPClient{
		baseURL:   "https://api.bybit.com",
		client:    &http.Client{Timeout: 15 * time.Second},
		config:    config,
		apiKey:    config.BybitApiKey,
		secretKey: config.BybitSecret,
	}
}

func (b *BybitHTTPClient) generateSignature(params string) string {
	mac := hmac.New(sha256.New, []byte(b.secretKey))
	mac.Write([]byte(params))
	return hex.EncodeToString(mac.Sum(nil))
}

func (b *BybitHTTPClient) makeAuthenticatedRequest(endpoint string, params map[string]string) ([]byte, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	// Собираем параметры для подписи
	var paramStr string
	for k, v := range params {
		if paramStr != "" {
			paramStr += "&"
		}
		paramStr += k + "=" + v
	}
	paramStr += "&timestamp=" + timestamp

	signature := b.generateSignature(paramStr)
	url := fmt.Sprintf("%s%s?%s&signature=%s", b.baseURL, endpoint, paramStr, signature)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-BAPI-API-KEY", b.apiKey)
	req.Header.Add("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Add("X-BAPI-SIGN", signature)
	req.Header.Add("X-BAPI-RECV-WINDOW", "5000")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func (b *BybitHTTPClient) makePublicRequest(url string) ([]byte, error) {
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

// Bybit implementation
type BybitExchange struct {
	httpClient *BybitHTTPClient
	config     Config
}

func NewBybitExchange(config Config) *BybitExchange {
	return &BybitExchange{
		httpClient: NewBybitHTTPClient(config),
		config:     config,
	}
}

func (b *BybitExchange) Name() string {
	return "Bybit"
}

func (b *BybitExchange) getAvailableCoins() (map[string]bool, error) {
	if b.config.BybitApiKey == "" || b.config.BybitSecret == "" {
		return nil, fmt.Errorf("требуются API ключи Bybit для проверки ввода/вывода")
	}

	endpoint := "/v5/asset/coin/query-info"
	data, err := b.httpClient.makeAuthenticatedRequest(endpoint, map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса информации о монетах: %w", err)
	}

	var response BybitCoinInfoResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("ошибка парсинга информации о монетах: %w", err)
	}

	if response.RetCode != 0 {
		return nil, fmt.Errorf("API error: %s", response.RetMsg)
	}

	availableCoins := make(map[string]bool)
	for _, coin := range response.Result.Rows {
		if coin.DepositStatus == 1 && coin.WithdrawalStatus == 1 {
			availableCoins[coin.Coin] = true
		}
	}

	return availableCoins, nil
}

func (b *BybitExchange) GetFilteredSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/v5/market/instruments-info?category=spot", b.httpClient.baseURL)

	data, err := b.httpClient.makePublicRequest(url)
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

	// Получаем информацию о доступных монетах (требует API ключи)
	var availableCoins map[string]bool
	if b.config.BybitApiKey != "" && b.config.BybitSecret != "" {
		availableCoins, err = b.getAvailableCoins()
		if err != nil {
			log.Printf("Предупреждение: не удалось получить информацию о монетах Bybit: %v", err)
		}
	}

	var filteredSymbols []string
	for _, symbol := range response.Result.List {
		// Базовые фильтры
		if !strings.HasSuffix(symbol.Symbol, "USDT") ||
			symbol.Status != "Trading" ||
			symbol.MinOrderAmt == "0" {
			continue
		}

		// Дополнительная проверка ввода/вывода если есть API ключи
		if availableCoins != nil {
			baseAvailable := availableCoins[symbol.BaseCoin]
			quoteAvailable := availableCoins[symbol.QuoteCoin]
			if !baseAvailable || !quoteAvailable {
				continue
			}
		}

		filteredSymbols = append(filteredSymbols, symbol.Symbol)
	}

	if len(filteredSymbols) > 10 {
		filteredSymbols = filteredSymbols[:10]
	}

	return filteredSymbols, nil
}

func (b *BybitExchange) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	url := fmt.Sprintf("%s/v5/market/orderbook?category=spot&symbol=%s&limit=%d",
		b.httpClient.baseURL, symbol, limit)

	data, err := b.httpClient.makePublicRequest(url)
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

// Binance implementation
type BinanceExchange struct {
	client *binance.Client
	config Config
}

func NewBinanceExchange(config Config) *BinanceExchange {
	client := binance.NewClient(config.BinanceApiKey, config.BinanceSecret)
	return &BinanceExchange{
		client: client,
		config: config,
	}
}

func (b *BinanceExchange) Name() string {
	return "Binance"
}

func (b *BinanceExchange) getAvailableCoins() (map[string]bool, error) {
	if b.config.BinanceApiKey == "" || b.config.BinanceSecret == "" {
		return nil, fmt.Errorf("требуются API ключи Binance для проверки ввода/вывода")
	}

	ctx := context.Background()
	coins, err := b.client.NewGetAllCoinsInfoService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения информации о монетах: %w", err)
	}

	availableCoins := make(map[string]bool)
	for _, coin := range coins {
		if coin.DepositAllEnable && coin.WithdrawAllEnable {
			availableCoins[coin.Coin] = true
		}
	}

	return availableCoins, nil
}

func (b *BinanceExchange) GetFilteredSymbols() ([]string, error) {
	ctx := context.Background()
	exchangeInfo, err := b.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance: ошибка получения информации: %w", err)
	}

	// Получаем информацию о доступных монетах (требует API ключи)
	var availableCoins map[string]bool
	if b.config.BinanceApiKey != "" && b.config.BinanceSecret != "" {
		availableCoins, err = b.getAvailableCoins()
		if err != nil {
			log.Printf("Предупреждение: не удалось получить информацию о монетах Binance: %v", err)
		}
	}

	var filteredSymbols []string
	for _, symbol := range exchangeInfo.Symbols {
		// Базовые фильтры
		if !strings.HasSuffix(symbol.Symbol, "USDT") ||
			symbol.Status != "TRADING" ||
			!symbol.IsSpotTradingAllowed {
			continue
		}

		// Дополнительная проверка ввода/вывода если есть API ключи
		if availableCoins != nil {
			baseCoin := strings.TrimSuffix(symbol.Symbol, "USDT")
			if !availableCoins[baseCoin] || !availableCoins["USDT"] {
				continue
			}
		}

		filteredSymbols = append(filteredSymbols, symbol.Symbol)
	}

	if len(filteredSymbols) > 10 {
		filteredSymbols = filteredSymbols[:10]
	}

	return filteredSymbols, nil
}

func (b *BinanceExchange) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	ctx := context.Background()
	binanceOrderBook, err := b.client.NewDepthService().
		Symbol(symbol).
		Limit(limit).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance: ошибка стакана для %s: %w", symbol, err)
	}

	return b.convertOrderBook(symbol, binanceOrderBook), nil
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

// ExchangeManager
type ExchangeManager struct {
	exchanges []Exchange
	config    Config
}

func NewExchangeManager(config Config) *ExchangeManager {
	return &ExchangeManager{
		config: config,
		exchanges: []Exchange{
			NewBinanceExchange(config),
			NewBybitExchange(config),
		},
	}
}

func (em *ExchangeManager) GetExchangeNames() []string {
	var names []string
	for _, exchange := range em.exchanges {
		names = append(names, exchange.Name())
	}
	return names
}

func printOrderBookSummary(ob *OrderBook) {
	fmt.Printf("\n=== %s - %s ===\n", ob.Exchange, ob.Symbol)
	fmt.Printf("Время: %d\n", ob.Timestamp)
	fmt.Printf("Аски (TOP 3):\n")
	for i := 0; i < 3 && i < len(ob.Asks); i++ {
		fmt.Printf("  %s - %s\n", ob.Asks[i].Price, ob.Asks[i].Quantity)
	}
	fmt.Printf("Биды (TOP 3):\n")
	for i := 0; i < 3 && i < len(ob.Bids); i++ {
		fmt.Printf("  %s - %s\n", ob.Bids[i].Price, ob.Bids[i].Quantity)
	}
}

func main() {
	config := Config{
		DelayMS:       100,
		Limit:         10,
		BinanceApiKey: "ВАШ_BINANCE_API_KEY",                  // Замените на свои ключи
		BinanceSecret: "ВАШ_BINANCE_SECRET_KEY",               // Замените на свои ключи
		BybitApiKey:   "VRpoz69CijtWoiY2NX ",                  // Замените на свои ключи
		BybitSecret:   "B6crmVReLfQuorCqAaFEvAL2vDa8wzRVp3QD", // Замените на свои ключи
	}

	manager := NewExchangeManager(config)

	fmt.Println("=== Крипто-биржи Монитор ===")
	fmt.Printf("Доступные биржи: %v\n", manager.GetExchangeNames())

	exchangeSymbols := make(map[string][]string)

	for _, exchange := range manager.exchanges {
		fmt.Printf("\n=== Получение пар с %s ===\n", exchange.Name())

		symbols, err := exchange.GetFilteredSymbols()
		if err != nil {
			log.Printf("Ошибка получения пар с %s: %v", exchange.Name(), err)
			continue
		}

		exchangeSymbols[exchange.Name()] = symbols
		fmt.Printf("Найдено пар с доступным вводом/выводом: %d\n", len(symbols))

		if len(symbols) > 0 {
			fmt.Println("Список пар:")
			for i, symbol := range symbols {
				fmt.Printf("  %d. %s\n", i+1, symbol)
			}
		}
	}

	fmt.Printf("\n=== Получение стаканов ===\n")

	for _, exchange := range manager.exchanges {
		symbols := exchangeSymbols[exchange.Name()]
		if len(symbols) == 0 {
			continue
		}

		fmt.Printf("\n=== %s ===\n", exchange.Name())

		orderBooks, err := exchange.GetMultipleOrderBooks(symbols, config.Limit, time.Duration(config.DelayMS)*time.Millisecond)
		if err != nil {
			log.Printf("Ошибка получения данных с %s: %v", exchange.Name(), err)
			continue
		}

		fmt.Printf("Успешно получено стаканов: %d\n", len(orderBooks))
		for _, ob := range orderBooks {
			printOrderBookSummary(ob)
		}
	}

	fmt.Println("\n=== Завершено ===")
}
