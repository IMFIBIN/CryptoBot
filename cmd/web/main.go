package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cryptobot/internal/app/webserver"
)

func main() {
	addr := getEnv("HTTP_ADDR", ":8080")

	// Быстрая проверка, что порт свободен заранее (как и раньше).
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("port busy %s: %v", addr, err)
	}
	_ = ln.Close()

	srv := webserver.New(addr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("server error: %v", err)
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
