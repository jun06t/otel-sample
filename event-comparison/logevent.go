package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// logBasedEvent は起きた瞬間を event.name 付きの LogRecord として logs 信号に出す。
//
// span.go の AddEvent をそのまま置き換えた形。出力は span 1 本と、それに
// trace_id / span_id で紐づく LogRecord が複数件になる。
// LogRecord は Emit した時点で出力され、span は End した時点で出力される。
func logBasedEvent(ctx context.Context, userID string, fail bool) error {
	ctx, span := tracer.Start(ctx, "GET /users/{id}",
		trace.WithAttributes(attribute.String("user.id", userID)),
	)
	defer span.End()

	if _, ok := cacheGet(ctx, cacheKey(userID)); !ok {
		emitEvent(ctx, "cache.miss", log.SeverityInfo,
			log.String("cache.key", cacheKey(userID)),
		)
	}

	for attempt := 1; ; attempt++ {
		_, err := dbQuery(ctx, userID, attempt, fail)
		if err == nil {
			return nil
		}
		if attempt == maxAttempts {
			// 失敗の事実は span status に、エラーの中身は ERROR ログに分ける。
			span.SetStatus(codes.Error, "fetch user failed")
			emitError(ctx, err)
			return err
		}
		emitEvent(ctx, "retry", log.SeverityWarn,
			log.Int("retry.attempt", attempt),
			log.String("error.message", err.Error()),
		)
	}
}

// emitEvent は event.name 付きの LogRecord(= log-based event)を出す。
// ctx に span があれば trace_id / span_id が自動で付く。
func emitEvent(ctx context.Context, name string, sev log.Severity, attrs ...log.KeyValue) {
	var r log.Record
	r.SetEventName(name) // これが無ければ普通のログ
	r.SetTimestamp(time.Now())
	r.SetSeverity(sev)
	r.AddAttributes(attrs...)
	logger.Emit(ctx, r)
}

// emitError は span.RecordError の置き換え。
// exception.* 属性を持つ ERROR ログ(event.name なし)を出す。
func emitError(ctx context.Context, err error) {
	var r log.Record
	r.SetTimestamp(time.Now())
	r.SetSeverity(log.SeverityError)
	r.SetBody(log.StringValue(err.Error()))
	r.AddAttributes(
		log.String("exception.type", fmt.Sprintf("%T", err)),
		log.String("exception.message", err.Error()),
	)
	logger.Emit(ctx, r)
}
