package main

import (
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type handler struct {
	tracer trace.Tracer
	logger *slog.Logger
}

// newHandler builds a handler wired to the globally configured otel providers.
func newHandler() *handler {
	return &handler{
		tracer: otel.Tracer("handler"),
		logger: otelslog.NewLogger("handler"),
	}
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"service":"duffel", "status":"running"}`))
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *handler) ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}
