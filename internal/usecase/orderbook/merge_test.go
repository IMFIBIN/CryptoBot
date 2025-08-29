package orderbook

import (
	"testing"

	"cryptobot/internal/domain"
)

func ob(asks, bids [][2]string) *domain.OrderBook {
	ob := &domain.OrderBook{}
	for _, a := range asks {
		ob.Asks = append(ob.Asks, domain.Order{Price: a[0], Quantity: a[1]})
	}
	for _, b := range bids {
		ob.Bids = append(ob.Bids, domain.Order{Price: b[0], Quantity: b[1]})
	}
	return ob
}

func TestCombinedAsks(t *testing.T) {
	books := map[string]*domain.OrderBook{
		"EX1": ob([][2]string{{"101", "1"}, {"100", "1"}}, nil),
		"EX2": ob([][2]string{{"99", "2"}}, nil),
	}
	levels := CombinedAsks(books)
	if len(levels) != 3 {
		t.Fatalf("levels=%d want=3", len(levels))
	}
	if levels[0].Price != 99 {
		t.Fatalf("best ask price=%.2f want=99", levels[0].Price)
	}
}

func TestCombinedBids(t *testing.T) {
	books := map[string]*domain.OrderBook{
		"EX1": ob(nil, [][2]string{{"101", "1"}, {"100", "1"}}),
		"EX2": ob(nil, [][2]string{{"99", "2"}}),
	}
	levels := CombinedBids(books)
	if len(levels) != 3 {
		t.Fatalf("levels=%d want=3", len(levels))
	}
	if levels[0].Price != 101 {
		t.Fatalf("best bid price=%.2f want=101", levels[0].Price)
	}
}
