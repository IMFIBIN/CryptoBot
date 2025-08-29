package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"cryptobot/internal/shared/format"
)

type InputParams struct {
	Action         string // "buy" или "sell"
	LeftCoinName   string
	LeftCoinChain  string
	RightCoinName  string
	RightCoinChain string
	LeftCoinVolume float64 // для buy: сумма USDT; для sell: количество монеты RightCoinName
}

func GetInteractiveParams() InputParams {
	reader := bufio.NewReader(os.Stdin)

	// 0) Выбор действия
	action := "buy"
	for {
		fmt.Println("Выберите действие:")
		fmt.Println("1) Купить монету за USDT")
		fmt.Println("2) Продать монету за USDT")
		fmt.Print("Ваш выбор [1-2] (Enter = 1): ")

		actRaw, _ := reader.ReadString('\n')
		actRaw = strings.TrimSpace(actRaw)

		if actRaw == "" || actRaw == "1" {
			action = "buy"
			break
		}
		if actRaw == "2" {
			action = "sell"
			break
		}
		fmt.Println("Введите 1 или 2, либо нажмите Enter для значения по умолчанию.")
	}

	// 1) Базовая валюта — USDT
	base := "USDT"
	fmt.Println("Выберите валюту, которой платите:")
	fmt.Println("1) USDT")
	fmt.Print("Ваш выбор [1] (Enter = USDT): ")
	// читаем, но пока поддерживаем только USDT
	_, _ = reader.ReadString('\n')
	base = "USDT"

	// 2) Выбор монеты
	coins := []string{"BTC", "ETH", "BNB", "ADA", "SOL"}
	choice := 1
	for {
		if action == "buy" {
			fmt.Println("\nНа какую монету хотите обменять USDT?")
		} else {
			fmt.Println("\nКакую монету хотите продать за USDT?")
		}
		for i, c := range coins {
			fmt.Printf("%d) %s\n", i+1, c)
		}
		fmt.Print("Ваш выбор [1-5] (Enter = BTC): ")

		choiceRaw, _ := reader.ReadString('\n')
		choiceRaw = strings.TrimSpace(choiceRaw)

		if choiceRaw == "" {
			choice = 1
			break
		}
		if n, err := strconv.Atoi(choiceRaw); err == nil && n >= 1 && n <= len(coins) {
			choice = n
			break
		}
		fmt.Println("Введите число от 1 до 5 или нажмите Enter для значения по умолчанию.")
	}
	right := coins[choice-1]

	// 3) Объём
	var amount float64
	if action == "buy" {
		// Покупка: спрашиваем сумму USDT
		defAmount := 100_000_000.0
		amount = defAmount
		for {
			fmt.Printf("\nСколько у вас %s? (Enter = %s): ", base, format.FloatRU(defAmount, 1))
			amountRaw, _ := reader.ReadString('\n')
			amountRaw = strings.TrimSpace(amountRaw)

			if amountRaw == "" {
				amount = defAmount
				break
			}

			normalized := strings.ReplaceAll(amountRaw, " ", "")
			normalized = strings.ReplaceAll(normalized, ".", "")
			normalized = strings.ReplaceAll(normalized, ",", ".")
			if v, err := strconv.ParseFloat(normalized, 64); err == nil && v > 0 {
				amount = v
				break
			}
			fmt.Println("Ошибка: введите положительное число (например, 12345.67 или 12,34).")
		}
	} else {
		// Продажа: спрашиваем количество выбранной монеты
		defQty := 1.0
		amount = defQty
		for {
			fmt.Printf("\nСколько у вас %s? (Enter = %s): ", right, format.FloatRU(defQty, 1))
			amountRaw, _ := reader.ReadString('\n')
			amountRaw = strings.TrimSpace(amountRaw)

			if amountRaw == "" {
				amount = defQty
				break
			}

			normalized := strings.ReplaceAll(amountRaw, " ", "")
			normalized = strings.ReplaceAll(normalized, ".", "")
			normalized = strings.ReplaceAll(normalized, ",", ".")
			if v, err := strconv.ParseFloat(normalized, 64); err == nil && v > 0 {
				amount = v
				break
			}
			fmt.Println("Ошибка: введите положительное число (например, 1.2345 или 0,5).")
		}
	}

	params := InputParams{
		Action:         action,
		LeftCoinName:   base, // оплачиваемая валюта по-умолчанию — USDT (для SELL это валюта получения)
		LeftCoinChain:  "SPOT",
		RightCoinName:  right, // целевая монета
		RightCoinChain: "SPOT",
		LeftCoinVolume: amount,
	}

	// 4) Подтверждение выбора — человекочитаемо
	if action == "buy" {
		fmt.Printf("\nОбмениваем %s на %s\n", params.LeftCoinName, params.RightCoinName)
		fmt.Printf("Доступно %s: %s\n", params.LeftCoinName, format.FloatRU(params.LeftCoinVolume, 2))
	} else {
		fmt.Printf("\nПродаём %s за %s\n", params.RightCoinName, params.LeftCoinName)
		fmt.Printf("Доступно %s: %s\n", params.RightCoinName, format.FloatRU(params.LeftCoinVolume, 4))
	}

	return params
}
