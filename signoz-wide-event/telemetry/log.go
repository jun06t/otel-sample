package telemetry

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewLoggerProvider は OTLP(gRPC) でログ(LogRecord)を送る LoggerProvider を構築する。
// TracerProvider と同じ resource / endpoint を渡すことで、ログとトレースが
// 同一の SigNoz(ClickHouse) ストアに co-locate され、1 画面で相関できる。
func NewLoggerProvider(ctx context.Context, endpoint string, res *resource.Resource) (*sdklog.LoggerProvider, func(), error) {
	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, nil, err
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := lp.Shutdown(ctx); err != nil {
			log.Print(err)
		}
	}
	return lp, cleanup, nil
}
