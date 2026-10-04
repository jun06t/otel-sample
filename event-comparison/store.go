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
type cache struct {
	tracer trace.Tracer
}

func newCache(tracer trace.Tracer) *cache {
	return &cache{tracer: tracer}
}

// get は区間のある操作なので子 span にする。
func (c *cache) get(ctx context.Context, key string) (value string, hit bool, err error) {
	ctx, span := c.tracer.Start(ctx, "cache get")
	defer span.End()

	if err := sleep(ctx, 5*time.Millisecond); err != nil {
		recordSpanError(span, err)
		return "", false, err
	}
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.Bool("cache.hit", hit),
	)
	return "", hit, nil
}

// DBTimeoutError は DB クエリのタイムアウトを表す。
type DBTimeoutError struct{}

func (e *DBTimeoutError) Error() string {
	return "db query timeout"
}

// userDB は DB を模す。1 回目の呼び出しは必ずタイムアウトし、fail=true なら毎回タイムアウトする。
// calls を排他制御していないので、並行に呼ぶことは想定していない。
type userDB struct {
	tracer trace.Tracer
	logger log.Logger
	fail   bool
	calls  int
}

func newUserDB(tracer trace.Tracer, logger log.Logger, fail bool) *userDB {
	return &userDB{tracer: tracer, logger: logger, fail: fail}
}

// selectUser は 1 回の DB 呼び出しを表す。DB client span の規約に従い、
// 呼び出しごとに CLIENT span を作る(リトライはアプリ側の fetchUser が行う)。
//
// この span も wide event の 1 行として、この呼び出し固有の文脈を属性に持つ。
// 何回目の試行か、コネクションプールの状態、返した行数などを載せておくと、
// 「タイムアウトした呼び出しはプールが枯渇していたか」を後から集計できる。
func (db *userDB) selectUser(ctx context.Context, userID string, attempt int) (string, error) {
	ctx, span := db.tracer.Start(ctx, "SELECT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.operation.name", "SELECT"),
			attribute.String("db.collection.name", "users"),
			attribute.String("db.client.connection.pool.name", "users-primary"),
			attribute.Int("app.db.attempt", attempt),
		),
	)
	defer span.End()

	if err := sleep(ctx, 20*time.Millisecond); err != nil {
		recordSpanError(span, err)
		return "", err
	}
	db.calls++
	if db.calls == 1 || db.fail {
		inUse, idle := 10, 0
		err := &DBTimeoutError{}
		// 失敗したのはこの 1 回の呼び出しなので、この span の status と error.type に記録する。
		recordSpanError(span, err)
		span.SetAttributes(
			attribute.Int("app.db.pool.in_use", inUse),
			attribute.Int("app.db.pool.idle", idle),
		)

		// 名前で引かない診断メッセージは、EventName なしの普通の LogRecord にする。
		// 集計に使う数値は上で span 属性にも載せている。
		var r log.Record
		r.SetTimestamp(time.Now())
		r.SetSeverity(log.SeverityDebug)
		r.SetBody(attribute.StringValue(fmt.Sprintf("connection pool exhausted: in_use=%d idle=%d", inUse, idle)))
		db.logger.Emit(ctx, r)

		return "", err
	}
	span.SetAttributes(
		attribute.Int("app.db.pool.in_use", 3),
		attribute.Int("app.db.pool.idle", 7),
		attribute.Int("db.response.returned_rows", 1),
	)
	return "name-of-" + userID, nil
}

// sleep は d だけ待つ。ctx がキャンセルされたら待たずに ctx.Err() を返す。
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// recordSpanError は、その span の操作が失敗した事実を status と error.type に記録する。
func recordSpanError(span trace.Span, err error) {
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(attribute.String("error.type", errorType(err)))
}
