package orderbook

import (
	"strconv"

	"cryptobot/internal/domain"
)

// BuyQtyFromAsksWithFee — сколько монеты можно купить на сумму usdt с учётом комиссии fee (на ношинал).
// Комиссия увеличивает реальную трату: cost = p*q*(1+fee).
// Возвращает: qty, avgPrice, spent (реально потрачено с комиссией).
func BuyQtyFromAsksWithFee(asks []domain.Order, usdt, fee float64) (qty, avgPrice, spent float64) {
	if usdt <= 0 || len(asks) == 0 {
		return 0, 0, 0
	}
	mul := 1 + fee
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
		levelCost := p * q * mul
		if remain >= levelCost {
			qty += q
			spent += levelCost
		} else {
			// частично: remain = p*q'*(1+fee) => q' = remain / (p*(1+fee))
			part := remain / (p * mul)
			if part > 0 {
				qty += part
				spent += remain
			}
			break
		}
	}
	if qty > 0 {
		avgPrice = spent / qty // уже с комиссией
	}
	return qty, avgPrice, spent
}

// SellFromBidsWithFee — сколько USDT можно получить, продав qty монеты с комиссией fee (снимается с ношинала).
// Комиссия уменьшает выручку: usdt = p*q*(1-fee).
func SellFromBidsWithFee(bids []domain.Order, qty, fee float64) (amountUSDT, avgPrice float64) {
	if qty <= 0 {
		return 0, 0
	}
	var sold float64
	mul := 1 - fee
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
			amountUSDT += q * p * mul
			sold += q
		} else {
			amountUSDT += remain * p * mul
			sold += remain
			break
		}
	}
	if sold > 0 {
		avgPrice = amountUSDT / sold
	}
	return amountUSDT, avgPrice
}
