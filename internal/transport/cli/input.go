package cli

import (
	"bufio"
	"cryptobot/internal/shared/format"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// InputParams — параметры, собранные интерактивно в CLI.
type InputParams struct {
	Action         string
	LeftCoinName   string
	LeftCoinVolume float64
	RightCoinName  string
}

// GetInteractiveParams — опрос пользователя в терминале.
func GetInteractiveParams() InputParams {
	reader := bufio.NewReader(os.Stdin)

	action := askAction(reader)

	coins := []string{"BTC", "ETH", "BNB", "SOL", "XRP", "ADA", "DOGE", "TON", "TRX", "DOT"}

	params := InputParams{Action: action}

	if action == "buy" {
		params.LeftCoinName = "USDT"

		fmt.Println("\nНа какую монету хотите обменять USDT?")
		params.RightCoinName = askFromList(reader, coins, 1)

		params.LeftCoinVolume = askFloat(reader, "\nСколько у вас USDT? (Enter = 1000000.0): ", 1_000_000.0)

		fmt.Printf("\nОбмениваем USDT на %s\nДоступно USDT: %s\n",
			params.RightCoinName, format.FloatRU(params.LeftCoinVolume, 2))

	} else {
		params.RightCoinName = "USDT"

		fmt.Println("\nКакую монету хотите продать за USDT?")
		params.LeftCoinName = askFromList(reader, coins, 1)

		prompt := fmt.Sprintf("\nСколько у вас %s? (Enter = 1.0): ", params.LeftCoinName)
		params.LeftCoinVolume = askFloat(reader, prompt, 1.0)

		fmt.Printf("\nПродаём %s за USDT\nДоступно %s: %s\n",
			params.LeftCoinName, params.LeftCoinName, format.FloatRU(params.LeftCoinVolume, 8))
	}

	return params
}

func askAction(r *bufio.Reader) string {
	for {
		fmt.Println("Выберите действие:")
		fmt.Println("1) Купить монету за USDT")
		fmt.Println("2) Продать монету за USDT")
		fmt.Print("Ваш выбор [1-2] (Enter = 1): ")

		raw, _ := r.ReadString('\n')
		raw = strings.TrimSpace(raw)

		switch raw {
		case "", "1":
			return "buy"
		case "2":
			return "sell"
		default:
			fmt.Println("Введите 1 или 2, либо нажмите Enter для значения по умолчанию.")
		}
	}
}

func askFromList(r *bufio.Reader, options []string, defIndex1 int) string {
	for i, c := range options {
		fmt.Printf("%d) %s\n", i+1, c)
	}
	fmt.Printf("Ваш выбор [1-%d] или тикер (Enter = %d): ", len(options), defIndex1)

	raw, _ := r.ReadString('\n')
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return options[defIndex1-1]
	}
	// пробуем как номер
	if n, err := strconv.Atoi(raw); err == nil && n >= 1 && n <= len(options) {
		return options[n-1]
	}
	// пробуем как тикер
	up := strings.ToUpper(raw)
	for _, it := range options {
		if up == it {
			return it
		}
	}
	fmt.Println("Некорректный выбор, используем значение по умолчанию.")
	return options[defIndex1-1]
}

func askFloat(r *bufio.Reader, prompt string, def float64) float64 {
	for {
		fmt.Print(prompt)
		raw, _ := r.ReadString('\n')
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return def
		}
		// поддержим запятую как разделитель
		raw = strings.ReplaceAll(raw, ",", ".")
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			return v
		}
		fmt.Println("Введите число (например, 1000 или 0,5).")
	}
}
