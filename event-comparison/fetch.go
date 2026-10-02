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

// maxAttempts は DB クエリの最大試行回数。
const maxAttempts = 3

type userService struct {
	cache  *cache
	db     *userDB
	events eventRecorder
}

// fetchUser は cache を引き、miss なら DB をリトライ付きで引く。
//
// 置き場は semconv の Events ガイダンスに従って選んでいる。
//
//	区間と境界がある操作              → span(fetch user / cache get / SELECT users)
//	操作全体の性質で、独自の時刻が不要 → span 属性(user.id, cache.hit, retry.count, error.type)
//	名前の付いた時点の出来事          → event(db.query.retry, user.fetch.exception)
//	名前で引かない診断メッセージ      → 普通の LogRecord(store.go の selectUser)
//
// 開始時に分かる属性(user.id)は tracer.Start で渡す。sampler が判断に使えるのは
// span 作成時にある属性だけなので、後から SetAttributes すると sampling に効かない。
func (s *userService) fetchUser(ctx context.Context, userID string) (name string, err error) {
	ctx, span := tracer.Start(ctx, "fetch user",
		trace.WithAttributes(attribute.String("user.id", userID)),
	)
	defer span.End()

	var (
		cacheHit bool
		retries  int
	)

	// span.End より先に実行される(defer は LIFO)。
	defer func() {
		// 終わるまで分からない操作全体の性質は、終了時にまとめてこの span の属性に書く。
		span.SetAttributes(
			attribute.Bool("cache.hit", cacheHit),
			attribute.Int("retry.count", retries),
		)
		if err != nil {
			// 失敗の事実は status と error.type、中身は exception event に分ける。
			span.SetStatus(codes.Error, "fetch user failed")
			span.SetAttributes(attribute.String("error.type", errorType(err)))
			s.events.exception(ctx, "user.fetch.exception", err)
		}
	}()

	if name, cacheHit = s.cache.get(ctx, cacheKey(userID)); cacheHit {
		return name, nil
	}

	for attempt := 1; ; attempt++ {
		name, err = s.db.selectUser(ctx, userID)
		if err == nil || attempt == maxAttempts {
			return name, err
		}

		// リトライの判断は「その回の時刻と属性(待ち時間など)」が要る時点の出来事なので event にする。
		// 回復しうるエラーなので、この span の status は変えない。
		backoff := time.Duration(attempt) * 10 * time.Millisecond
		s.events.event(ctx, "db.query.retry", log.SeverityWarn, err,
			attribute.Int("retry.attempt", attempt+1),
			attribute.Int64("retry.backoff_ms", backoff.Milliseconds()),
			attribute.String("error.type", errorType(err)),
		)
		retries++
		time.Sleep(backoff)
	}
}

// errorType は error.type / exception.type に入れる値を返す。
// span.RecordError が付ける exception.type と同じく、Go の型名を使う。
func errorType(err error) string {
	return fmt.Sprintf("%T", err)
}

func cacheKey(userID string) string {
	return "user:" + userID
}
