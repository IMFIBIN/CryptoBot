package orderbook

import (
	"strconv"

	"cryptobot/internal/domain"
	"cryptobot/internal/usecase/fees"
)

// BuyQtyFromAsksWithFee — считаем сколько монет купим на бюджет "budgetNet" USDT,
// учитывая комиссию "f". Возвращаем:
//
//	qty         — кол-во монет,
//	avgPrice    — средняя цена (USDT за 1 монету), ВКЛЮЧАЯ комиссию,
//	spentNet    — сколько реально списали (с комиссией),
//	feeTotal    — сумма комиссии.
func BuyQtyFromAsksWithFee(asks []domain.Order, budgetNet float64, f fees.Fee) (qty, avgPrice, spentNet, feeTotal float64) {
	if budgetNet <= 0 || len(asks) == 0 {
		return 0, 0, 0, 0
	}
	// Комиссию применим к суммарному "gross" (стоимость без комиссии).
	// Рассчитываем лимит "gross", который укладывается в "budgetNet".
	grossCap := f.InvertBuy(budgetNet)

	var grossSpent float64
	for _, a := range asks {
		p, err1 := strconv.ParseFloat(a.Price, 64)
		q, err2 := strconv.ParseFloat(a.Quantity, 64)
		if err1 != nil || err2 != nil || p <= 0 || q <= 0 {
			continue
		}
		needGross := p * q
		remain := grossCap - grossSpent
		if remain <= 0 {
			break
		}
		if needGross <= remain {
			grossSpent += needGross
			qty += q
		} else {
			// добираем частично
			takeQty := remain / p
			if takeQty > 0 {
				grossSpent += takeQty * p
				qty += takeQty
			}
			break
		}
	}

	if qty <= 0 {
		return 0, 0, 0, 0
	}
	net, fee := f.ApplyBuy(grossSpent)
	spentNet = net
	feeTotal = fee
	avgPrice = spentNet / qty
	return
}

// SellFromBidsWithFee — продаём qty монет, считаем выручку с комиссией.
// Возвращаем:
//
//	receivedNet — сколько получим (после комиссии),
//	avgPrice    — средняя цена продажи за 1 монету (после комиссии),
//	feeTotal    — комиссия в USDT.
func SellFromBidsWithFee(bids []domain.Order, qty float64, f fees.Fee) (receivedNet, avgPrice, feeTotal float64) {
	if qty <= 0 || len(bids) == 0 {
		return 0, 0, 0
	}
	var soldQty float64
	var grossProceeds float64
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
			grossProceeds += q * p
			soldQty += q
		} else {
			grossProceeds += remain * p
			soldQty += remain
			break
		}
	}
	if soldQty <= 0 {
		return 0, 0, 0
	}
	net, fee := f.ApplySell(grossProceeds)
	receivedNet = net
	feeTotal = fee
	avgPrice = receivedNet / soldQty
	return
}
