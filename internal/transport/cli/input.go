package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// InputParams — параметры, собранные интерактивно в CLI.
type InputParams struct {
	// buy | sell
	Action string

	// Левая монета и её объём (что отдаём):
	// - BUY: всегда USDT, пользователь вводит сумму USDT
	// - SELL: выбранная монета, пользователь вводит её количество
	LeftCoinName   string
	LeftCoinVolume float64

	// Правая монета (что получаем):
	// - BUY: выбранная монета (BTC/ETH/…)
	// - SELL: всегда USDT
	RightCoinName string
}

// GetInteractiveParams — опрос пользователя в терминале.
func GetInteractiveParams() InputParams {
	reader := bufio.NewReader(os.Stdin)

	// 0) Выбор действия
	action := askAction(reader)

	// 1) Список монет для выбора
	coins := []string{"BTC", "ETH", "BNB", "ADA", "SOL"}

	params := InputParams{Action: action}

	if action == "buy" {
		// BUY: платим USDT, получаем выбранную монету
		params.LeftCoinName = "USDT"

		fmt.Println("\nНа какую монету хотите обменять USDT?")
		params.RightCoinName = askFromList(reader, coins, 1)

		// Сколько USDT у пользователя
		params.LeftCoinVolume = askFloat(reader, "\nСколько у вас USDT? (Enter = 1000000.0): ", 1_000_000.0)

		// Контекст (дружественное подтверждение выбора)
		fmt.Printf("\nОбмениваем USDT на %s\nДоступно USDT: %s\n",
			params.RightCoinName, humanUSDT(params.LeftCoinVolume))

	} else {
		// SELL: продаём выбранную монету, получаем USDT
		params.RightCoinName = "USDT"

		fmt.Println("\nКакую монету хотите продать за USDT?")
		params.LeftCoinName = askFromList(reader, coins, 1)

		// Сколько монеты у пользователя
		prompt := fmt.Sprintf("\nСколько у вас %s? (Enter = 1.0): ", params.LeftCoinName)
		params.LeftCoinVolume = askFloat(reader, prompt, 1.0)

		// Контекст (дружественное подтверждение выбора)
		fmt.Printf("\nПродаём %s за USDT\nДоступно %s: %.8f\n",
			params.LeftCoinName, params.LeftCoinName, params.LeftCoinVolume)
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
	fmt.Printf("Ваш выбор [1-%d] (Enter = %d): ", len(options), defIndex1)

	raw, _ := r.ReadString('\n')
	raw = strings.TrimSpace(raw)

	idx := defIndex1
	if raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			idx = n
		}
	}
	if idx < 1 || idx > len(options) {
		idx = defIndex1
	}
	return options[idx-1]
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

func humanUSDT(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	intPart, frac := split2(s, ".")
	intPart = withThinSpaces(intPart)
	return intPart + "," + frac // запятая как в примерах
}

func split2(s, sep string) (string, string) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+1:]
}

func withThinSpaces(s string) string {
	// добавим пробелы между тысячами справа налево
	if len(s) <= 3 {
		return s
	}
	var out []byte
	cnt := 0
	for i := len(s) - 1; i >= 0; i-- {
		out = append(out, s[i])
		cnt++
		if cnt%3 == 0 && i != 0 {
			out = append(out, ' ')
		}
	}
	// reverse
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}
