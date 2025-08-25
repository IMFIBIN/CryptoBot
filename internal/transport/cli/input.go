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
	LeftCoinName   string
	LeftCoinChain  string
	RightCoinName  string
	RightCoinChain string
	LeftCoinVolume float64
}

func GetInteractiveParams() InputParams {
	reader := bufio.NewReader(os.Stdin)

	// 1) Базовая валюта (пока только USDT) — с повтором до корректного ввода
	base := "USDT"
	for {
		fmt.Println("Выберите валюту, которой платите:")
		fmt.Println("1) USDT")
		fmt.Print("Ваш выбор [1] (Enter = USDT): ")

		baseRaw, _ := reader.ReadString('\n')
		baseRaw = strings.TrimSpace(baseRaw)

		if baseRaw == "" || baseRaw == "1" {
			base = "USDT"
			break
		}
		fmt.Println("Пока доступна только USDT. Нажмите Enter или введите 1.")
	}

	// 2) На какую монету хотим обменять — 5 монет, с повтором
	coins := []string{"BTC", "ETH", "BNB", "ADA", "SOL"}
	choice := 1
	for {
		fmt.Println("\nНа какую монету хотите обменять USDT?")
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

	// 3) Сколько у нас USDT — пусто = дефолт, иначе требуем > 0 (с повтором)
	defAmount := 100_000_000.0
	amount := defAmount
	for {
		fmt.Printf("\nСколько у вас %s? (Enter = %s): ", base, format.FloatRU(defAmount, 1))
		amountRaw, _ := reader.ReadString('\n')
		amountRaw = strings.TrimSpace(amountRaw)

		if amountRaw == "" {
			amount = defAmount
			break
		}

		// поддерживаем "100.000.000,0" / "100000000" / "100,5" и т.п.
		normalized := strings.ReplaceAll(amountRaw, " ", "")
		normalized = strings.ReplaceAll(normalized, ".", "")
		normalized = strings.ReplaceAll(normalized, ",", ".")
		if v, err := strconv.ParseFloat(normalized, 64); err == nil && v > 0 {
			amount = v
			break
		}
		fmt.Println("Ошибка: введите положительное число (например, 12345.67 или 12,34).")
	}

	params := InputParams{
		LeftCoinName:   base,
		LeftCoinChain:  "SPOT",
		RightCoinName:  right,
		RightCoinChain: "SPOT",
		LeftCoinVolume: amount,
	}

	// подтверждение выбора — красиво
	fmt.Printf("\nОбмениваем %s на %s\n", params.LeftCoinName, params.RightCoinName)
	fmt.Printf("Доступно %s: %s\n", params.LeftCoinName, format.FloatRU(params.LeftCoinVolume, 2))

	return params
}
