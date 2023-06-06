package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	pb "github.com/jun06t/otel-sample/sampling/proto"
	"github.com/jun06t/otel-sample/sampling/telemetry"
)

var tracer = otel.Tracer("github.com/jun06t/otel-sample/sampling/gateway")

func main() {
	log.Println("Initialize the gateway server...")

	backend := os.Getenv("BACKEND_ADDR")
	otelAddr := os.Getenv("EXPORTER_ENDPOINT")
	ratio, _ := strconv.ParseFloat(os.Getenv("SAMPLING_RATIO"), 10)

	_, cleanup, err := telemetry.NewTracerProvider(context.TODO(), otelAddr, "otel-sample", ratio)
	if err != nil {
		log.Fatal(err)
		time.Sleep(3 * time.Second)
	}
	defer cleanup()

	h := newHandler(backend)

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(h.alive))
	mux.Handle("/hello", http.HandlerFunc(h.hello))

	log.Println("Starting the gateway server...")

	if err := http.ListenAndServe(":8000", telemetry.NewHTTPMiddleware()(mux)); err != nil {
		log.Fatal(err)
	}
}

type handler struct {
	cli pb.GreeterClient
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

	return &handler{
		cli: c,
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
}

func Operation(ctx context.Context) {
	_, span := tracer.Start(ctx, "op1")
	defer span.End()
	time.Sleep(100 * time.Millisecond)
}
