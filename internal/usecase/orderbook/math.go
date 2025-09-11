package orderbook

import (
	"strconv"

	"cryptobot/internal/domain"
)

func BuyQtyFromAsks(asks []domain.Order, budget float64) (qty, avgPrice, spent float64) {
	if budget <= 0 || len(asks) == 0 {
		return 0, 0, 0
	}
	var grossSpent float64
	for _, a := range asks {
		p, err1 := strconv.ParseFloat(a.Price, 64)
		q, err2 := strconv.ParseFloat(a.Quantity, 64)
		if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
			continue
		}
		remain := budget - grossSpent
		if remain <= 0 {
			break
		}
		costFull := p * q
		if costFull <= remain {
			grossSpent += costFull
			qty += q
		} else {
			takeQty := remain / p
			if takeQty > 0 {
				grossSpent += takeQty * p
				qty += takeQty
			}
			break
		}
	}
	if qty <= 0 {
		return 0, 0, 0
	}
	spent = grossSpent
	avgPrice = spent / qty
	return
}

func SellFromBids(bids []domain.Order, qty float64) (received, avgPrice float64) {
	if qty <= 0 || len(bids) == 0 {
		return 0, 0
	}
	var soldQty float64
	for _, b := range bids {
		p, err1 := strconv.ParseFloat(b.Price, 64)
		q, err2 := strconv.ParseFloat(b.Quantity, 64)
		if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
			continue
		}
		remain := qty - soldQty
		if remain <= 0 {
			break
		}
		if q <= remain {
			received += q * p
			soldQty += q
		} else {
			received += remain * p
			soldQty += remain
			break
		}
	}
	if soldQty <= 0 {
		return 0, 0
	}
	avgPrice = received / soldQty
	return
}
