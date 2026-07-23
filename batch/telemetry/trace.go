package telemetry

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func init() {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
}

// NewTracerProvider は TracerProvider をグローバルに設定し、shutdown 関数を返す。
//
// exporterEndpoint が空なら stdout exporter（そのまま go run で span を確認できる）、
// 値があれば OTLP gRPC (insecure) でその宛先（例: jaeger:4317）へ送信する。
func NewTracerProvider(ctx context.Context, exporterEndpoint, serviceName string) (*sdktrace.TracerProvider, func(), error) {
	exporter, err := newExporter(ctx, exporterEndpoint)
	if err != nil {
		return nil, nil, err
	}

	r := NewResource(serviceName, "1.0.0", "local")
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	// ★肝2: BatchSpanProcessor は溜めてから送るが、Shutdown() が内部で
	//        ForceFlush してくれるので、終了前に呼べば取りこぼさない。timeout は十分に。
	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			log.Printf("failed to shutdown tracer provider: %v", err)
		}
	}
	return tp, shutdown, nil
}

func newExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	if endpoint == "" {
		// デモ用: stdout に出力（そのまま実行可能）。
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	}
	// 本番/可視化用: OTLP gRPC で Jaeger 等へ送信。
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
	)
	return otlptrace.New(ctx, client)
}

func NewResource(serviceName, version, environment string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(version),
		attribute.String("environment", environment),
	)
}
