package main

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"log"
	"time"
)

type BinanceClient struct {
	client *binance.Client
}

func NewBinanceClient(apiKey, secretKey string) *BinanceClient {
	client := binance.NewClient(apiKey, secretKey)
	return &BinanceClient{client: client}
}

// GetExchangeInfo получает информацию о всех торговых парах
func (bc *BinanceClient) GetExchangeInfo() (*binance.ExchangeInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return bc.client.NewExchangeInfoService().Do(ctx)
}

// GetOrderBook получает стакан ордеров для конкретной пары
func (bc *BinanceClient) GetOrderBook(symbol string, limit int) (*binance.DepthResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return bc.client.NewDepthService().
		Symbol(symbol).
		Limit(limit).
		Do(ctx)
}

// GetAllSymbols получает список всех торговых пар
func (bc *BinanceClient) GetAllSymbols() ([]string, error) {
	exchangeInfo, err := bc.GetExchangeInfo()
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, symbol := range exchangeInfo.Symbols {
		if symbol.Status == "TRADING" { // только активные пары
			symbols = append(symbols, symbol.Symbol)
		}
	}

	return symbols, nil
}

// GetOrderBooksForMultipleSymbols получает стаканы для нескольких пар
func (bc *BinanceClient) GetOrderBooksForMultipleSymbols(symbols []string, limit int, delay time.Duration) (map[string]*binance.DepthResponse, error) {
	result := make(map[string]*binance.DepthResponse)

	for _, symbol := range symbols {
		orderBook, err := bc.GetOrderBook(symbol, limit)
		if err != nil {
			log.Printf("Ошибка получения стакана для %s: %v", symbol, err)
			continue
		}

		result[symbol] = orderBook
		time.Sleep(delay) // чтобы не превысить лимиты API
	}

	return result, nil
}

func main() {
	// Инициализация клиента (можно без API ключей для публичных данных)
	client := NewBinanceClient("", "") // для публичных данных ключи не нужны

	fmt.Println("=== Получение списка всех торговых пар ===")

	// Получаем все символы
	symbols, err := client.GetAllSymbols()
	if err != nil {
		log.Fatalf("Ошибка получения списка пар: %v", err)
	}

	fmt.Printf("Найдено %d торговых пар\n", len(symbols))

	// Выводим первые 10 пар для примера
	fmt.Println("\nПервые 10 торговых пар:")
	for i := 0; i < 10 && i < len(symbols); i++ {
		fmt.Printf("%d. %s\n", i+1, symbols[i])
	}

	fmt.Println("\n=== Получение стаканов для нескольких пар ===")

	// Выбираем несколько популярных пар для примера
	selectedSymbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "ADAUSDT", "SOLUSDT"}

	// Получаем стаканы с лимитом 10 ордеров с каждой стороны
	orderBooks, err := client.GetOrderBooksForMultipleSymbols(selectedSymbols, 10, 100*time.Millisecond)
	if err != nil {
		log.Fatalf("Ошибка получения стаканов: %v", err)
	}

	// Выводим информацию о стаканах
	for symbol, orderBook := range orderBooks {
		fmt.Printf("\nСтакан для %s:\n", symbol)
		fmt.Printf("Последнее обновление: %d\n", orderBook.LastUpdateID)

		fmt.Printf("Аски (покупка):\n")
		for i, ask := range orderBook.Asks {
			if i >= 5 { // показываем только первые 5
				break
			}
			fmt.Printf("  Цена: %s, Количество: %s\n", ask.Price, ask.Quantity)
		}

		fmt.Printf("Биды (продажа):\n")
		for i, bid := range orderBook.Bids {
			if i >= 5 {
				break
			}
			fmt.Printf("  Цена: %s, Количество: %s\n", bid.Price, bid.Quantity)
		}
	}
}

// Дополнительные утилитарные функции

// PrintSymbolInfo печатает информацию о символах
func PrintSymbolInfo(symbols []binance.Symbol) {
	for _, symbol := range symbols {
		if symbol.Status == "TRADING" {
			fmt.Printf("Пара: %s, Базовая: %s, Котировка: %s\n",
				symbol.Symbol, symbol.BaseAsset, symbol.QuoteAsset)
		}
	}
}

// FilterSymbolsByQuote фильтрует пары по валюте котировки
func FilterSymbolsByQuote(symbols []binance.Symbol, quoteAsset string) []binance.Symbol {
	var result []binance.Symbol
	for _, symbol := range symbols {
		if symbol.QuoteAsset == quoteAsset && symbol.Status == "TRADING" {
			result = append(result, symbol)
		}
	}
	return result
}
