package httpapi

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"cryptobot/internal/infra/exchangebooks"
)

// RateResponse — ответ на /api/rate: mid = USDT за 1 <coin>.
type RateResponse struct {
	Coin      string   `json:"coin"`
	Mid       float64  `json:"mid"`                 // USDT per 1 <coin>
	Exchanges []string `json:"exchanges,omitempty"` // список бирж, у которых взяли цены
}

// handleRate обрабатывает GET /api/rate?coin=ETH
// Возвращает mid-цену (USDT за 1 <coin>). Для USDT -> 1.
func (s *Server) handleRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	coin := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("coin")))
	if coin == "" {
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "missing 'coin' query param"})
		return
	}
	if coin == "USDT" {
		_ = json.NewEncoder(w).Encode(RateResponse{Coin: "USDT", Mid: 1})
		return
	}

	repo := exchangebooks.NewHTTPRepo()

	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	// Берём стаканы <COIN>/USDT, глубины 5 достаточно для mid
	books, _, err := repo.FetchAllBooks(ctx, coin, 5)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "failed to fetch orderbooks: " + err.Error()})
		return
	}
	if len(books) == 0 {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "no books for " + coin + "/USDT"})
		return
	}

	var mids []float64
	var exs []string

	for _, b := range books {
		bestAsk := math.Inf(1)
		if len(b.Asks) > 0 {
			bestAsk = b.Asks[0].Price
		}
		bestBid := math.Inf(-1)
		if len(b.Bids) > 0 {
			bestBid = b.Bids[0].Price
		}
		switch {
		case !math.IsInf(bestAsk, 1) && !math.IsInf(bestBid, -1) && bestAsk > 0 && bestBid > 0:
			mids = append(mids, (bestAsk+bestBid)/2)
			exs = append(exs, b.Exchange)
		case !math.IsInf(bestAsk, 1) && bestAsk > 0:
			mids = append(mids, bestAsk)
			exs = append(exs, b.Exchange)
		case !math.IsInf(bestBid, -1) && bestBid > 0:
			mids = append(mids, bestBid)
			exs = append(exs, b.Exchange)
		}
	}

	if len(mids) == 0 {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "no top-of-book prices for " + coin + "/USDT"})
		return
	}

	sort.Float64s(mids)
	var median float64
	n := len(mids)
	if n%2 == 1 {
		median = mids[n/2]
	} else {
		median = (mids[n/2-1] + mids[n/2]) / 2
	}

	resp := RateResponse{
		Coin:      coin,
		Mid:       median,
		Exchanges: exs,
	}
	_ = json.NewEncoder(w).Encode(resp)
}
