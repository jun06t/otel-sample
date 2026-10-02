package main

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// spanEvent は起きた瞬間を span.AddEvent で親 span に注釈する。
//
// 出力は span 1 本で、その Events 配列に cache.miss / retry / exception が並ぶ。
// OTel は 2026-03 に仕様として Span Event API(Go では AddEvent / RecordError)を
// deprecated とし、時点の出来事は log-based event(logevent.go)へ移行する方針。
// ただし Go SDK v1.44 の時点では、これらのメソッドに Deprecated 表記はまだ付いていない。
func spanEvent(ctx context.Context, userID string, fail bool) error {
	ctx, span := tracer.Start(ctx, "GET /users/{id}",
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
			span.RecordError(err)
			span.SetStatus(codes.Error, "fetch user failed")
			return err
		}
		span.AddEvent("retry",
			trace.WithAttributes(
				attribute.Int("retry.attempt", attempt),
				attribute.String("error.message", err.Error()),
			),
		)
	}
}
