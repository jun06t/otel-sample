package util

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("github.com/jun06t/otel-sample/multi-package/util")

func Operation(ctx context.Context) {
	_, span := tracer.Start(ctx, "op")
	defer span.End()
	time.Sleep(100 * time.Millisecond)
}
