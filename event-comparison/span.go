package main

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// spanEvent は起きた瞬間を span.AddEvent で親 span に注釈する。
//
// 出力は span 1 本で、その Events 配列に cache.miss / db.query.retry / exception が並ぶ。
// OTel は 2026-03 に Span Event API(Go では AddEvent / RecordError)を段階的に
// 非推奨にする方針を発表し、新しいイベントには Logs API(logevent.go)を推奨している。
// Go SDK v1.45 の時点では、これらのメソッドに Deprecated 表記はまだ付いていない。
func spanEvent(ctx context.Context, userID string, fail bool) error {
	ctx, span := tracer.Start(ctx, spanName,
		trace.WithAttributes(attribute.String("user.id", userID)),
	)
	defer span.End()

	if _, ok := cacheGet(ctx, cacheKey(userID)); !ok {
		span.AddEvent("cache.miss",
			trace.WithAttributes(attribute.String("cache.key", cacheKey(userID))),
		)
	}

	for attempt := 1; ; attempt++ {
		_, err := dbQuery(ctx, userID, attempt, fail)
		if err == nil {
			return nil
		}
		if attempt == maxAttempts {
			// "exception" という名前の span event になる。
			span.RecordError(err, trace.WithStackTrace(true))
			// 失敗の事実は status と error.type で表す。
			span.SetStatus(codes.Error, "fetch user failed")
			span.SetAttributes(attribute.String("error.type", errorType(err)))
			return err
		}
		// リトライで回復しうるエラーなので、span の status は変えない。
		span.AddEvent("db.query.retry",
			trace.WithAttributes(
				attribute.Int("retry.attempt", attempt),
				attribute.String("error.type", errorType(err)),
			),
		)
	}
}
