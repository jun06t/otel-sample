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

// cache は常に miss する cache を模す。
type cache struct{}

// get は区間のある操作なので子 span にする。
func (c *cache) get(ctx context.Context, key string) (string, bool) {
	_, span := tracer.Start(ctx, "cache get")
	defer span.End()

	time.Sleep(5 * time.Millisecond)
	hit := false
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.Bool("cache.hit", hit),
	)
	return "", hit
}

// DBTimeoutError は DB クエリのタイムアウトを表す。
type DBTimeoutError struct{}

func (e *DBTimeoutError) Error() string {
	return "db query timeout"
}

// userDB は DB を模す。1 回目の呼び出しは必ずタイムアウトし、fail=true なら毎回タイムアウトする。
type userDB struct {
	fail  bool
	calls int
}

// selectUser は 1 回の DB 呼び出しを表す。DB client span の規約に従い、
// 呼び出しごとに CLIENT span を作る(リトライはアプリ側の fetchUser が行う)。
func (db *userDB) selectUser(ctx context.Context, userID string) (string, error) {
	ctx, span := tracer.Start(ctx, "SELECT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.operation.name", "SELECT"),
			attribute.String("db.collection.name", "users"),
		),
	)
	defer span.End()

	time.Sleep(20 * time.Millisecond)
	db.calls++
	if db.calls == 1 || db.fail {
		err := &DBTimeoutError{}
		// 失敗したのはこの 1 回の呼び出しなので、この span の status と error.type に記録する。
		span.SetStatus(codes.Error, "query timeout")
		span.SetAttributes(attribute.String("error.type", errorType(err)))

		// 名前で引かない診断メッセージは、EventName なしの普通の LogRecord にする。
		var r log.Record
		r.SetTimestamp(time.Now())
		r.SetSeverity(log.SeverityDebug)
		r.SetBody(attribute.StringValue(fmt.Sprintf("connection pool exhausted: in_use=%d idle=%d", 10, 0)))
		logger.Emit(ctx, r)

		return "", err
	}
	return "name-of-" + userID, nil
}
