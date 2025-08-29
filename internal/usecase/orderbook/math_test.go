package orderbook

import (
	"testing"

	"cryptobot/internal/domain"
)

func TestBuyQtyFromAsksWithFee(t *testing.T) {
	asks := []domain.Order{
		{Price: "100.0", Quantity: "1.0"},
		{Price: "110.0", Quantity: "2.0"},
	}
	qty, avg, spent := BuyQtyFromAsksWithFee(asks, 100.0, 0.0)
	if qty != 1.0 {
		t.Fatalf("qty=%.8f want=1.0", qty)
	}
	if avg != 100.0 {
		t.Fatalf("avg=%.8f want=100.0", avg)
	}
	if spent != 100.0 {
		t.Fatalf("spent=%.8f want=100.0", spent)
	}

	// с комиссией 1% хватит только на 0.990099... монеты по 100
	qty, avg, spent = BuyQtyFromAsksWithFee(asks, 100.0, 0.01)
	if qty <= 0.99 || qty >= 1.0 {
		t.Fatalf("qty with fee=%.8f want ~0.9901", qty)
	}
	if spent < 99.99 || spent > 100.0 {
		t.Fatalf("spent with fee=%.8f want ~100", spent)
	}
}

func TestSellFromBidsWithFee(t *testing.T) {
	bids := []domain.Order{
		{Price: "100.0", Quantity: "1.0"},
		{Price: "90.0", Quantity: "10.0"},
	}
	usdt, avg := SellFromBidsWithFee(bids, 1.0, 0.0)
	if usdt != 100.0 {
		t.Fatalf("usdt=%.8f want=100", usdt)
	}
	if avg != 100.0 {
		t.Fatalf("avg=%.8f want=100", avg)
	}

	usdt, avg = SellFromBidsWithFee(bids, 1.0, 0.01) // 1% комиссия
	if usdt != 99.0 {
		t.Fatalf("usdt with fee=%.8f want=99", usdt)
	}
	if avg != 99.0 {
		t.Fatalf("avg with fee=%.8f want=99", avg)
	}
}
