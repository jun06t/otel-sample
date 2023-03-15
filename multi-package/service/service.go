package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jun06t/otel-sample/multi-package/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

type handler struct {
	cli http.Client
}

var tracer = otel.Tracer("github.com/jun06t/otel-sample/multi-package/service")

func NewHandler() *handler {
	hc := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	return &handler{
		cli: hc,
	}
}

func (h *handler) Alive(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func (h *handler) Hello(w http.ResponseWriter, r *http.Request) {
	Operation1(r.Context())

	hreq, err := http.NewRequestWithContext(r.Context(), "GET", "http://httpbin.org/delay/1", nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := h.cli.Do(hreq)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	util.Operation(r.Context())
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
