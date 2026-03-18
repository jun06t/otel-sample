package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpcCodes "google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"

	pb "github.com/jun06t/otel-sample/tail-sampling/proto"
	"github.com/jun06t/otel-sample/tail-sampling/telemetry"
)

const (
	port = ":8080"
)

var tracer = otel.Tracer("github.com/jun06t/otel-sample/tail-sampling/backend")

type server struct{}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Println(in.String())

	r := rand.Float64()
	switch {
	case r < 0.1:
		// 10% error
		span := trace.SpanFromContext(ctx)
		span.SetStatus(codes.Error, "simulated error")
		span.RecordError(fmt.Errorf("simulated internal error"))
		return nil, grpcStatus.Error(grpcCodes.Internal, "simulated error")
	case r < 0.2:
		// 10% slow response (1-3s)
		delay := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
		time.Sleep(delay)
	}

	Operation(ctx)
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func main() {
	log.Println("Initialize the backend server...")

	otelAddr := os.Getenv("EXPORTER_ENDPOINT")
	_, cleanup, err := telemetry.NewTracerProvider(context.TODO(), otelAddr, "backend-grpc")
	if err != nil {
		log.Fatal(err)
		time.Sleep(3 * time.Second)
	}
	defer cleanup()

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal(err)
		time.Sleep(3 * time.Second)
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(telemetry.NewUnaryServerInterceptor()))
	pb.RegisterGreeterServer(s, &server{})

	log.Println("Starting the backend server...")
	err = s.Serve(lis)
	if err != nil {
		log.Fatal(err)
		time.Sleep(3 * time.Second)
	}
}

func Operation(ctx context.Context) {
	_, span := tracer.Start(ctx, "operation")
	defer span.End()
	time.Sleep(200 * time.Millisecond)
}
