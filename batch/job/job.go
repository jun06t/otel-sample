package job

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Run は「1 実行 = 1 root span」を体現する。
// バッチ実行全体を覆う root span を張り、その配下に子 span をぶら下げる。
func Run(ctx context.Context, serviceName string) error {
	tracer := otel.Tracer(serviceName)

	ctx, span := tracer.Start(ctx, "batch.Run") // ← このバッチ実行全体の root span
	defer span.End()

	items, err := fetch(ctx, tracer)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "fetch failed")
		return err
	}
	span.SetAttributes(attribute.Int("batch.item_count", len(items)))

	if err := process(ctx, tracer, items); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "process failed")
		return err
	}
	return nil
}

func fetch(ctx context.Context, tracer trace.Tracer) ([]string, error) {
	_, span := tracer.Start(ctx, "fetch") // root span の子
	defer span.End()
	time.Sleep(50 * time.Millisecond) // 疑似 I/O
	return []string{"a", "b", "c"}, nil
}

func process(ctx context.Context, tracer trace.Tracer, items []string) error {
	if len(items) == 0 {
		return errors.New("no items")
	}
	for _, item := range items {
		_, span := tracer.Start(ctx, "process-item")
		span.SetAttributes(attribute.String("item", item))
		time.Sleep(20 * time.Millisecond) // 疑似処理
		span.End()
	}
	return nil
}
