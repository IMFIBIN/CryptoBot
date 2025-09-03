package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cryptobot/internal/transport/httpapi"
)

// --- Временная заглушка, чтобы фронт заработал сразу ---

type MockFlow struct{}

func (MockFlow) Plan(ctx context.Context, req httpapi.PlanRequest) (httpapi.PlanResponse, error) {
	_ = ctx

	// "Цены" фиктивные, но разнообразим сценарии
	priceBinance := 1.001
	priceOKX := 1.002
	priceBybit := 1.003
	feeRate := 0.001 // 0.1%

	var legs []httpapi.PlanLeg
	switch req.Scenario {
	case "best_single":
		// всё на лучшую цену
		legs = []httpapi.PlanLeg{
			{Exchange: "binance", Amount: req.Amount, Price: priceBinance},
		}
	case "equal_split":
		// поровну на три биржи
		a := req.Amount / 3.0
		legs = []httpapi.PlanLeg{
			{Exchange: "binance", Amount: a, Price: priceBinance},
			{Exchange: "okx", Amount: a, Price: priceOKX},
			{Exchange: "bybit", Amount: a, Price: priceBybit},
		}
	default: // "optimal"
		// 50%/30%/20%
		legs = []httpapi.PlanLeg{
			{Exchange: "binance", Amount: req.Amount * 0.5, Price: priceBinance},
			{Exchange: "okx", Amount: req.Amount * 0.3, Price: priceOKX},
			{Exchange: "bybit", Amount: req.Amount * 0.2, Price: priceBybit},
		}
	}

	var cost, totalFees float64
	for i := range legs {
		leg := &legs[i]
		leg.Fee = leg.Amount * leg.Price * feeRate
		cost += leg.Amount * leg.Price
		totalFees += leg.Fee
	}
	vwap := cost / req.Amount

	return httpapi.PlanResponse{
		Base: req.Base, Quote: req.Quote, Amount: req.Amount,
		Scenario: req.Scenario, VWAP: vwap,
		TotalCost: cost, TotalFees: totalFees,
		Legs: legs, GeneratedAt: time.Now().Format("15:04 02.01.2006"),
	}, nil
}

// --- Заготовка для подключения реального usecase ---
/*
func newServerWithRealFlow(addr string) *httpapi.Server {
// TODO: тут создаём реальные адаптеры бирж и сценарий, как в CLI.
// Оборачиваем ваш usecase в тип, реализующий httpapi.FlowFacade.
flow := yourFlowFacade{}
return httpapi.New(addr, flow)
}
*/

func newServerWithMockFlow(addr string) *httpapi.Server {
	return httpapi.New(addr, MockFlow{})
}

func main() {
	addr := getEnv("HTTP_ADDR", ":8080")

	// Проверяем, что порт свободен
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("port busy %s: %v", addr, err)
	}
	_ = ln.Close()

	srv := newServerWithMockFlow(addr)
	// srv := newServerWithRealFlow(addr) // ← переключи сюда после wiring

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("server stopped: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
