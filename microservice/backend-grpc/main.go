package main

import (
	"context"
	"log"
	"net"
	"time"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	pb "github.com/jun06t/otel-sample/microservice/proto"
	"github.com/jun06t/otel-sample/microservice/telemetry"
)

const (
	port = ":8080"
)

var tracer = otel.Tracer("github.com/jun06t/otel-sample/microservice/backend")

type server struct{}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Println(in.String())
	Operation(ctx)
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func main() {
	_, cleanup, err := telemetry.NewTracerProvider("backend-grpc")
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(telemetry.NewUnaryServerInterceptor()))
	pb.RegisterGreeterServer(s, &server{})
	err = s.Serve(lis)
	if err != nil {
		log.Fatal(err)
	}
}

func Operation(ctx context.Context) {
	_, span := tracer.Start(ctx, "operation")
	defer span.End()
	time.Sleep(200 * time.Millisecond)
}
