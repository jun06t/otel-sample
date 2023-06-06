package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	pb "github.com/jun06t/otel-sample/sampling/proto"
	"github.com/jun06t/otel-sample/sampling/telemetry"
)

const (
	port = ":8080"
)

var tracer = otel.Tracer("github.com/jun06t/otel-sample/sampling/backend")

type server struct{}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Println(in.String())
	Operation(ctx)
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func main() {
	log.Println("Initialize the backend server...")

	otelAddr := os.Getenv("EXPORTER_ENDPOINT")
	ratio, _ := strconv.ParseFloat(os.Getenv("SAMPLING_RATIO"), 10)
	_, cleanup, err := telemetry.NewTracerProvider(context.TODO(), otelAddr, "backend-grpc", ratio)
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
