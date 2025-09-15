package webserver

import (
	"cryptobot/internal/infra/exchangebooks"
	"cryptobot/internal/transport/httpapi"
	"cryptobot/internal/usecase/planner"
)

func New(addr string) *httpapi.Server {
	// Инфраструктура: тянем стаканы <COIN>/USDT по HTTP с бирж
	repo := exchangebooks.NewHTTPRepo()
	// Чистый use-case планировщика
	svc := planner.New(repo)
	// Адаптер между httpapi и planner.Service
	return httpapi.New(addr, &httpapi.PlannerAdapter{Svc: svc})
}
