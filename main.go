package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/riandyrn/otelchi"
)

// listenAddr is the address the HTTP server binds to.
const listenAddr = ":8080"

func main() {
	// load .env before anything else reads configuration so that local runs pick
	// up the same OTEL_* variables a deployment would inject. A load failure is
	// fatal because everything downstream depends on that configuration.
	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	ctx := context.Background()

	// bring telemetry up first so failures during the rest of startup are
	// observable. The deferred shutdown flushes buffered telemetry on the way
	// out; it runs last, after the server has already stopped accepting traffic.
	shutdown, err := initTelemetry(ctx)
	if err != nil {
		log.Fatalf("failed to initialize telemetry: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("failed to shutdown telemetry: %v", err)
		}
	}()

	// newHandler wires the request handlers to the now-initialized telemetry
	// providers, so it must run after initTelemetry.
	h := newHandler()

	router := chi.NewRouter()
	// otelchi produces a span per request and names it from the matched route
	// pattern, giving every handler tracing without per-handler boilerplate.
	router.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(router)))
	router.Get("/health", h.health)
	router.Get("/status", h.status)
	router.Get("/ping", h.ping)

	server := &http.Server{
		Addr:    listenAddr,
		Handler: router,
	}

	// run the server in its own goroutine so main can block on the signal channel
	// and coordinate shutdown. ErrServerClosed is the expected result of a clean
	// Shutdown, so only unexpected errors are fatal.
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to listen and serve: %v", err)
		}
	}()

	log.Println("server started on", listenAddr)

	// block until the process is asked to terminate. A buffered channel is used
	// so the signal is not missed if it arrives before this goroutine parks.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("server stopping")

	// bound the drain so a stuck in-flight request can't block exit forever;
	// Shutdown stops accepting new connections and waits for active ones to
	// finish, up to this deadline.
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("failed to shutdown server: %v", err)
	}
}
