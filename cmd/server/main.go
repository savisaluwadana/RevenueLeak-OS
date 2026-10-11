package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"revenueleak-os/internal/core"
	"revenueleak-os/internal/server"
)

func main() {
	demo := strings.EqualFold(os.Getenv("DEMO_MODE"), "true")
	password := os.Getenv("ADMIN_PASSWORD")
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if password == "" && !demo {
		log.Fatal("ADMIN_PASSWORD required unless DEMO_MODE=true")
	}
	if password == "" && demo && !strings.HasPrefix(addr, "127.0.0.1:") && !strings.HasPrefix(addr, "localhost:") {
		log.Fatal("unauthenticated demo must bind only to localhost; configure ADMIN_PASSWORD for shared hosting")
	}
	dataPath := os.Getenv("DATA_FILE")
	if dataPath == "" {
		dataPath = "./data/revenueleak.json"
	}
	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "./web"
	}
	store, err := core.Open(dataPath)
	if err != nil {
		log.Fatal(err)
	}
	if demo && len(store.Snapshot().Contracts) == 0 {
		if err = store.Update(func(d *core.Database) error { core.SeedDemo(d, time.Now()); return nil }); err != nil {
			log.Fatal(err)
		}
	}
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if rawInterval := os.Getenv("AUTO_SCAN_INTERVAL"); rawInterval != "" {
		interval, parseErr := time.ParseDuration(rawInterval)
		if parseErr != nil || interval < time.Hour {
			log.Fatal("AUTO_SCAN_INTERVAL must be a duration of at least 1h (example: 24h)")
		}
		go core.RunScheduledScans(shutdownCtx, store, interval, 7, func(err error) {
			log.Printf("scheduled scan failed: %v", err)
		})
		log.Printf("automatic reconciliation enabled: interval=%s, grace=7d", interval)
	}
	srv := server.New(store, password, demo, webDir)
	log.Printf("RevenueLeak OS %s: demo=%v stripe=%v", addr, demo, os.Getenv("STRIPE_RESTRICTED_KEY") != "")
	httpServer := &http.Server{Addr: addr, Handler: srv.Router(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	go func() {
		<-shutdownCtx.Done()
		stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(stopCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
