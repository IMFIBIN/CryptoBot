package fees

// Fee — абстракция комиссии.
// Все операции считаются в валюте котировки (USDT).
//
// BUY:
//
//	gross — сумма без комиссии (чистая стоимость купленных монет).
//	ApplyBuy(gross) -> net, fee — сколько реально спишется (net) и сколько из этого комиссия (fee).
//	InvertBuy(net)  -> gross — сколько можно купить на "net" с учётом комиссии.
//
// SELL:
//
//	gross — валовая выручка без комиссии (цена*кол-во).
//	ApplySell(gross) -> net, fee — сколько придёт на счёт (net) и сколько удержано комиссией (fee).
type Fee interface {
	ApplyBuy(gross float64) (net float64, fee float64)
	InvertBuy(net float64) (gross float64)
	ApplySell(gross float64) (net float64, fee float64)
	Describe() string
}

// ===== Относительная комиссия (процент от оборота) =====

type Relative struct{ Pct float64 } // напр. 0.001 = 0.1%

func NewRelative(pct float64) Relative { return Relative{Pct: pct} }

func (r Relative) ApplyBuy(gross float64) (net, fee float64) {
	fee = gross * r.Pct
	return gross + fee, fee
}

func (r Relative) InvertBuy(net float64) (gross float64) {
	// net = gross*(1+pct) => gross = net/(1+pct)
	return net / (1 + r.Pct)
}

func (r Relative) ApplySell(gross float64) (net, fee float64) {
	fee = gross * r.Pct
	return gross - fee, fee
}

func (r Relative) Describe() string { return "relative" }

// ===== Абсолютная комиссия (фикс в USDT за сделку) =====

type Absolute struct{ Amount float64 } // напр. 1.0 USDT за сделку

func NewAbsolute(amount float64) Absolute { return Absolute{Amount: amount} }

func (a Absolute) ApplyBuy(gross float64) (net, fee float64) {
	fee = a.Amount
	return gross + fee, fee
}

func (a Absolute) InvertBuy(net float64) (gross float64) {
	// net = gross + A => gross = net - A (не меньше 0)
	g := net - a.Amount
	if g < 0 {
		return 0
	}
	return g
}

func (a Absolute) ApplySell(gross float64) (net, fee float64) {
	fee = a.Amount
	n := gross - fee
	if n < 0 {
		n = 0
	}
	return n, fee
}

func (a Absolute) Describe() string { return "absolute" }
