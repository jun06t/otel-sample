package main

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestRequestInfoProcessor(t *testing.T) {
	remoteParent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{1},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	tests := []struct {
		name     string
		parent   func(ctx context.Context, tr trace.Tracer) (context.Context, func())
		wantPlan bool
	}{
		{
			name:     "親がいない span には付ける",
			parent:   func(ctx context.Context, _ trace.Tracer) (context.Context, func()) { return ctx, func() {} },
			wantPlan: true,
		},
		{
			name: "親が別プロセスにいる span には付ける",
			parent: func(ctx context.Context, _ trace.Tracer) (context.Context, func()) {
				return trace.ContextWithRemoteSpanContext(ctx, remoteParent), func() {}
			},
			wantPlan: true,
		},
		{
			name: "同じプロセスに親がいる子 span には付けない",
			parent: func(ctx context.Context, tr trace.Tracer) (context.Context, func()) {
				ctx, span := tr.Start(ctx, "parent")
				return ctx, func() { span.End() }
			},
			wantPlan: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tracetest.NewSpanRecorder()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSpanProcessor(newRequestInfoProcessor()),
				sdktrace.WithSpanProcessor(rec),
			)
			tr := tp.Tracer("test")

			ctx := withRequestInfo(context.Background(), requestInfo{Plan: "premium"})
			ctx, endParent := tt.parent(ctx, tr)
			_, span := tr.Start(ctx, "target")
			span.End()
			endParent()

			var target sdktrace.ReadOnlySpan
			for _, s := range rec.Ended() {
				if s.Name() == "target" {
					target = s
				}
			}
			if target == nil {
				t.Fatal("target span was not recorded")
			}

			got := hasAttr(target.Attributes(), attribute.String("app.user.plan", "premium"))
			if got != tt.wantPlan {
				t.Errorf("app.user.plan attached = %v, want %v", got, tt.wantPlan)
			}
		})
	}
}

func hasAttr(attrs []attribute.KeyValue, want attribute.KeyValue) bool {
	for _, a := range attrs {
		if a == want {
			return true
		}
	}
	return false
}
