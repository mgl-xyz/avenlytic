package main

import (
	"context"
	"embed"
	"flag"
	"github.com/avenlytic/avenlytic/internal/server"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed web/*
var webFS embed.FS

func main() {
	listen := flag.String("listen", ":9080", "HTTP listen address")
	token := flag.String("token", os.Getenv("AGENT_TOKEN"), "Agent bearer token (can also be configured in the Web UI)")
	data := flag.String("data", "data/avenlytic.db", "SQLite database path")
	flag.Parse()
	if *token == "" {
		log.Println("INFO: Agent token is not configured yet; complete first-run setup in the Web UI")
	}
	app := server.NewAt(*token, *data)
	defer func() {
		if err := app.Close(); err != nil {
			log.Printf("close state database: %v", err)
		}
	}()
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	api := app.Handler()
	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.HandleFunc("GET /healthz", app.Health)
	mux.Handle("/", app.UIHandler(spaFallback(http.FileServer(http.FS(sub)))))
	srv := &http.Server{Addr: *listen, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go app.RunMonitor(ctx.Done())
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("Avenlytic listening on %s", *listen)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func spaFallback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login", "/hosts", "/alerts", "/rules", "/channels", "/settings":
			routeRequest := r.Clone(r.Context())
			routeURL := *r.URL
			routeURL.Path = "/"
			routeURL.RawPath = ""
			routeRequest.URL = &routeURL
			next.ServeHTTP(w, routeRequest)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
