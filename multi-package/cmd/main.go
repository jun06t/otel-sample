package main

import (
	"log"
	"net/http"

	"github.com/jun06t/otel-sample/multi-package/service"
	"github.com/jun06t/otel-sample/multi-package/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	_, cleanup, err := telemetry.NewTracerProvider("otel-sample")
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	h := service.NewHandler()

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(h.Alive))
	mux.Handle("/hello", http.HandlerFunc(h.Hello))
	http.ListenAndServe(":8000", otelhttp.NewHandler(mux, "server",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	))
}
