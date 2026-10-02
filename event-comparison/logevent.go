package main

import (
	"context"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// logBasedEvent は起きた瞬間を EventName 付きの LogRecord として logs 信号に出す。
//
// span.go の AddEvent / RecordError をそのまま置き換えた形。出力は span 1 本と、
// それに trace_id / span_id で紐づく LogRecord が複数件になる。
// LogRecord は Emit した時点で出力され、span は End した時点で出力される。
func logBasedEvent(ctx context.Context, userID string, fail bool) error {
	ctx, span := tracer.Start(ctx, spanName,
		trace.WithAttributes(attribute.String("user.id", userID)),
	)
	defer span.End()

	if _, ok := cacheGet(ctx, cacheKey(userID)); !ok {
		emitEvent(ctx, "cache.miss", log.SeverityInfo, nil,
			attribute.String("cache.key", cacheKey(userID)),
		)
	}

	for attempt := 1; ; attempt++ {
		_, err := dbQuery(ctx, userID, attempt, fail)
		if err == nil {
			return nil
		}
		if attempt == maxAttempts {
			// 失敗の事実は span の status と error.type に、エラーの中身は exception ログに分ける。
			span.SetStatus(codes.Error, "fetch user failed")
			span.SetAttributes(attribute.String("error.type", errorType(err)))
			emitException(ctx, "user.fetch.exception", err)
			return err
		}
		// リトライで回復しうるエラーなので WARN。メッセージは Body に入れる。
		emitEvent(ctx, "db.query.retry", log.SeverityWarn, err,
			attribute.Int("retry.attempt", attempt),
			attribute.String("error.type", errorType(err)),
		)
	}
}

// emitEvent は EventName 付きの LogRecord(= log-based event)を出す。
// err があれば、そのメッセージを Body に入れる。
// ctx に span があれば trace_id / span_id が自動で付く。
func emitEvent(ctx context.Context, name string, sev log.Severity, err error, attrs ...attribute.KeyValue) {
	var r log.Record
	r.SetEventName(name) // これが空なら普通のログ
	r.SetTimestamp(time.Now())
	r.SetSeverity(sev)
	if err != nil {
		r.SetBody(attribute.StringValue(err.Error()))
	}
	r.AddAttributes(attrs...)
	logger.Emit(ctx, r)
}

// emitException は span.RecordError の置き換え。
// semconv に従い、EventName は「操作名 + .exception」にし、exception.* 属性を付ける。
// 呼び出し元に返すエラーなので ERROR にする。
func emitException(ctx context.Context, name string, err error) {
	emitEvent(ctx, name, log.SeverityError, err,
		attribute.String("exception.type", errorType(err)),
		attribute.String("exception.message", err.Error()),
		attribute.String("exception.stacktrace", string(debug.Stack())),
	)
}
