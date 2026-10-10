package main

import (
	"log"
	"net/http"
	"os"
	"strings"
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
	srv := server.New(store, password, demo, webDir)
	log.Printf("RevenueLeak OS %s: demo=%v stripe=%v", addr, demo, os.Getenv("STRIPE_RESTRICTED_KEY") != "")
	httpServer := &http.Server{Addr: addr, Handler: srv.Router(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	log.Fatal(httpServer.ListenAndServe())
}
