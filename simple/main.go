package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func main() {
	backend := os.Getenv("BACKEND_ADDR")

	_, cleanup, err := NewTracerProvider("otel-sample")
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	tracer = otel.Tracer("github.com/jun06t/otel-sample/simple/main")

	h := newHandler(backend)

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(h.alive))
	mux.Handle("/hello", otelhttp.NewHandler(http.HandlerFunc(h.hello), "server"))
	http.ListenAndServe(":8000", mux)
}

type handler struct {
	cli http.Client
}

func newHandler(addr string) *handler {
	hc := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	return &handler{
		cli: hc,
	}
}

func (h *handler) alive(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func (h *handler) hello(w http.ResponseWriter, r *http.Request) {
	Operation1(r.Context())
	Operation2(r.Context())

	hreq, err := http.NewRequestWithContext(r.Context(), "GET", "http://httpbin.org/delay/1", nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := h.cli.Do(hreq)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	Operation3(r.Context())
}

func Operation1(ctx context.Context) {
	_, span := tracer.Start(ctx, "op1")
	defer span.End()
	time.Sleep(100 * time.Millisecond)
}

func Operation2(ctx context.Context) {
	_, span := tracer.Start(ctx, "op2")
	defer span.End()
	time.Sleep(100 * time.Millisecond)
}

func Operation3(ctx context.Context) {
	_, span := tracer.Start(ctx, "op3")
	defer span.End()
	time.Sleep(100 * time.Millisecond)
}
