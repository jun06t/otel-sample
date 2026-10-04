package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	serviceName = "event-comparison"
	scopeName   = "github.com/jun06t/otel-sample/event-comparison"
)

func main() {
	os.Exit(run())
}

// run は終了コードを返す。os.Exit は defer を実行しないため、
// shutdown は main ではなくここで済ませる。
func run() int {
	fail := flag.Bool("fail", false, "DB クエリをリトライ上限まで失敗させる")
	spanEvents := flag.Bool("span-events", false, "event を Logs API ではなく span event(AddEvent / RecordError)で記録する")
	requests := flag.Int("requests", 1, "送るリクエスト数。1 なら毎回同じ 1 リクエスト、2 以上なら文脈をばらつかせる。0 なら止めるまで送り続ける")
	interval := flag.Duration("interval", 0, "リクエストの間隔")
	seed := flag.Uint64("seed", 1, "文脈をばらつかせる乱数のシード")
	flag.Parse()

	// Ctrl+C や docker stop で止められるようにする。待ち処理は ctx のキャンセルに従う。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// EXPORTER_ENDPOINT があれば OTLP で送り、なければ stdout に出す。
	endpoint := os.Getenv("EXPORTER_ENDPOINT")
	tel, err := newTelemetry(ctx, endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup telemetry: %v\n", err)
		return 1
	}
	tracer, logger := tel.tracer(), tel.logger()

	var events eventRecorder = newLogEventRecorder(logger)
	if *spanEvents {
		events = newSpanEventRecorder()
	}

	rnd := rand.New(rand.NewPCG(*seed, *seed))
	var sent, failed int
	var lastErr error
	for *requests == 0 || sent < *requests {
		if ctx.Err() != nil {
			break
		}
		sc := fixedScenario(*fail)
		if *requests != 1 {
			sc = randomScenario(rnd, *fail)
		}
		s := newUserService(tracer, newCache(tracer, sc.cacheHit), newUserDB(tracer, logger, sc.dbTimeouts), events)

		// HTTP サーバーなら middleware が行う処理。テレメトリーの次元だけを context に入れ、
		// 処理の入力である user ID は引数で渡す。
		rctx := withRequestInfo(ctx, sc.info)

		// エラーの記録は fetchUser の中で済んでいる。ここでは数えるだけ。
		if _, lastErr = s.fetchUser(rctx, sc.userID); lastErr != nil {
			failed++
		}
		sent++

		if err := sleep(ctx, *interval); err != nil {
			break
		}
	}

	// ctx はキャンセル済みかもしれないので、shutdown には別の ctx を使う。
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tel.shutdown(sctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown telemetry: %v\n", err)
	}

	if *requests == 1 {
		if lastErr != nil {
			fmt.Fprintf(os.Stderr, "fetch user: %v\n", lastErr)
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "sent=%d failed=%d\n", sent, failed)
	return 0
}
