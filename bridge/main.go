package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func main() {
	_, cleanup, err := NewTracerProvider("otel-sample")
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	tracer = otel.Tracer("github.com/jun06t/otel-sample/simple/main")

	h := newHandler()

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(h.alive))
	mux.Handle("/hello", http.HandlerFunc(h.hello))
	http.ListenAndServe(":8000", otelhttp.NewHandler(mux, "server",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	))
}

type handler struct {
}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) alive(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func (h *handler) hello(w http.ResponseWriter, r *http.Request) {
	Operation(r.Context())

	write(r.Context(), "foobar20221010")
	readRows(r.Context(), "foobar")
}

func Operation(ctx context.Context) {
	_, span := tracer.Start(ctx, "op1")
	defer span.End()
	time.Sleep(100 * time.Millisecond)
}
