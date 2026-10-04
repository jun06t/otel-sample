package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/log/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestFetchUserStopsOnCancel(t *testing.T) {
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	logger := noop.NewLoggerProvider().Logger("test")
	s := newUserService(tracer, newCache(tracer), newUserDB(tracer, logger, true), newLogEventRecorder(logger))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := s.fetchUser(ctx, "alice")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("fetchUser took %v; want it to stop shortly after the 30ms deadline", elapsed)
	}
}
