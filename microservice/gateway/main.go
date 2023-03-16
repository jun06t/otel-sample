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
	"google.golang.org/grpc"

	pb "github.com/jun06t/otel-sample/microservice/proto"
	"github.com/jun06t/otel-sample/microservice/telemetry"
)

var tracer = otel.Tracer("github.com/jun06t/otel-sample/microservice/gateway")

func main() {
	backend := os.Getenv("BACKEND_ADDR")

	_, cleanup, err := telemetry.NewTracerProvider("otel-sample")
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	h := newHandler(backend)

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(h.alive))
	mux.Handle("/hello", http.HandlerFunc(h.hello))
	http.ListenAndServe(":8000", telemetry.NewHTTPMiddleware()(mux))
}

type handler struct {
	cli  pb.GreeterClient
	hcli http.Client
}

func newHandler(addr string) *handler {
	conn, err := grpc.Dial(addr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(telemetry.NewUnaryClientInterceptor()),
	)
	if err != nil {
		log.Fatal(err)
	}
	c := pb.NewGreeterClient(conn)

	hc := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	return &handler{
		cli:  c,
		hcli: hc,
	}
}

func (h *handler) alive(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func (h *handler) hello(w http.ResponseWriter, r *http.Request) {
	req := &pb.HelloRequest{
		Name: "alice",
		Age:  10,
		Man:  true,
	}
	_, err := h.cli.SayHello(r.Context(), req)
	if err != nil {
		log.Fatal(err)
	}

	Operation(r.Context())

	hreq, err := http.NewRequestWithContext(r.Context(), "GET", "http://httpbin.org/delay/2", nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := h.hcli.Do(hreq)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
}

func Operation(ctx context.Context) {
	_, span := tracer.Start(ctx, "op1")
	defer span.End()
	time.Sleep(100 * time.Millisecond)
}
