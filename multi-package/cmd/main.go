package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jun06t/otel-sample/multi-package/service"
	"github.com/jun06t/otel-sample/multi-package/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	backend := os.Getenv("BACKEND_ADDR")

	_, cleanup, err := telemetry.NewTracerProvider("otel-sample")
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	h := service.NewHandler(backend)

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(h.Alive))
	mux.Handle("/hello", otelhttp.NewHandler(http.HandlerFunc(h.Hello), "server"))
	http.ListenAndServe(":8000", mux)
}
