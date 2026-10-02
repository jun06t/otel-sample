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

// request は 1 回のユーザー取得リクエストの文脈。
// 実際のサービスなら認証情報や User-Agent、feature flag の評価結果から組み立てる。
type request struct {
	UserID     string
	Country    string // ISO 3166-1 alpha-2
	Plan       string
	OSName     string
	OSVersion  string
	AppName    string
	AppVersion string
	NewProfile bool // feature flag: 新しいプロフィール画面
}

// startAttributes は開始時に分かるリクエスト文脈を返す。
//
// wide event の考え方では、span 1 本を 1 行、属性を列とみなし、何に使うか分からなくても
// 関係しそうな文脈は全部載せておく。障害調査のときに、事前に用意していない組み合わせ
// (例: OS のバージョン × アプリのバージョン)で絞り込めるようにするため。
// cardinality が高い値(user.id など)も避けない。
func (r request) startAttributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("user.id", r.UserID),
		attribute.String("user_agent.name", r.AppName),
		attribute.String("user_agent.version", r.AppVersion),
		attribute.String("user_agent.os.name", r.OSName),
		attribute.String("user_agent.os.version", r.OSVersion),
		attribute.String("app.user.country", r.Country),
		attribute.String("app.user.plan", r.Plan),
		attribute.Bool("app.feature.new_profile", r.NewProfile),
	}
}

type userService struct {
	cache  *cache
	db     *userDB
	events eventRecorder
}

// fetchUser は cache を引き、miss なら DB をリトライ付きで引く。
//
// どの span も wide event の 1 行として属性(列)を太らせる。このメイン span には
// リクエスト全体の文脈(開始時に分かるものと、cache.hit や DB の呼び出し回数・合計時間など
// 終わるまで分からない結果)を載せ、子 span(store.go)にはその操作固有の属性を載せる。
// 別々の span にある属性は同時に条件にできないので、集計軸にしたい属性は同じ span に載せる。
//
// それ以外の置き場は semconv の Events ガイダンスに従って選んでいる。
//
//	区間と境界がある操作              → span(fetch user / cache get / SELECT users)
//	操作全体の性質で、独自の時刻が不要 → span 属性(user.id, user_agent.*, cache.hit, retry.count, error.type など)
//	名前の付いた時点の出来事          → event(db.query.retry, user.fetch.exception)
//	名前で引かない診断メッセージ      → 普通の LogRecord(store.go の selectUser)
//
// 開始時に分かる属性は tracer.Start で渡す。sampler が判断に使えるのは
// span 作成時にある属性だけなので、後から SetAttributes すると sampling に効かない。
func (s *userService) fetchUser(ctx context.Context, req request) (name string, err error) {
	ctx, span := tracer.Start(ctx, "fetch user",
		trace.WithAttributes(req.startAttributes()...),
	)
	defer span.End()

	var (
		cacheHit   bool
		retries    int
		dbCalls    int
		dbDuration time.Duration
	)

	// span.End より先に実行される(defer は LIFO)。
	defer func() {
		// 終わるまで分からない操作全体の性質は、終了時にまとめてこの span の属性に書く。
		span.SetAttributes(
			attribute.Bool("cache.hit", cacheHit),
			attribute.Int("retry.count", retries),
			attribute.Int("app.db.call.count", dbCalls),
			attribute.Int64("app.db.duration_ms", dbDuration.Milliseconds()),
		)
		if err != nil {
			// 失敗の事実は status と error.type、中身は exception event に分ける。
			span.SetStatus(codes.Error, "fetch user failed")
			span.SetAttributes(attribute.String("error.type", errorType(err)))
			s.events.exception(ctx, "user.fetch.exception", err)
		}
	}()

	if name, cacheHit = s.cache.get(ctx, cacheKey(req.UserID)); cacheHit {
		return name, nil
	}

	for attempt := 1; ; attempt++ {
		start := time.Now()
		name, err = s.db.selectUser(ctx, req.UserID, attempt)
		dbCalls++
		dbDuration += time.Since(start)
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
