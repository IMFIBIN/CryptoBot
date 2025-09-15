package httpapi

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

//go:embed webui/*
var embeddedFS embed.FS

type FlowFacade interface {
	Plan(ctx context.Context, req PlanRequest) (PlanResponse, error)
}

type Server struct {
	addr   string
	flow   FlowFacade
	server *http.Server
}

func New(addr string, flow FlowFacade) *Server { return &Server{addr: addr, flow: flow} }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/plan", s.handlePlan)
	mux.HandleFunc("/api/symbols", s.handleSymbols) // только USDT как quote

	// static
	sub, err := fs.Sub(embeddedFS, "webui")
	if err != nil {
		log.Printf("embed sub error: %v", err)
		mux.Handle("/", http.FileServer(http.FS(embeddedFS)))
	} else {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}

	return withCORS(mux)
}

func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("HTTP server listening on %s", s.addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var req PlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid JSON: " + err.Error()})
		return
	}

	// Нормализация
	req.Base = strings.ToUpper(strings.TrimSpace(req.Base))
	req.Quote = strings.ToUpper(strings.TrimSpace(req.Quote))
	req.Scenario = strings.TrimSpace(req.Scenario)

	if req.Base == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "base is required"})
		return
	}
	if req.Quote == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "quote is required"})
		return
	}
	if strings.EqualFold(req.Base, req.Quote) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "Нельзя выбирать одинаковые монеты (выберите разные в полях «Отдаёте» и «Получаете»)."})
		return
	}
	if req.Amount <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "amount must be > 0"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	res, err := s.flow.Plan(ctx, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleSymbols(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	all := []string{
		"USDT",
		"BTC", "ETH", "BNB", "SOL", "XRP", "ADA", "DOGE", "TON", "TRX", "DOT",
	}

	all = uniqStrings(all)
	sort.Strings(all)

	resp := map[string]any{
		"bases":  all,
		"quotes": all,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func uniqStrings(xs []string) []string {
	seen := make(map[string]struct{}, len(xs))
	out := make([]string, 0, len(xs))
	for _, v := range xs {
		v = strings.ToUpper(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
