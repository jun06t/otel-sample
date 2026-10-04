package main

import (
	"context"
	"flag"
	"fmt"
	"os"
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
	flag.Parse()

	tel, err := newTelemetry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup telemetry: %v\n", err)
		return 1
	}
	tracer, logger := tel.tracer(), tel.logger()

	var events eventRecorder = newLogEventRecorder(logger)
	if *spanEvents {
		events = newSpanEventRecorder()
	}
	s := newUserService(tracer, newCache(tracer), newUserDB(tracer, logger, *fail), events)

	ctx := context.Background()

	// HTTP サーバーなら middleware が行う処理。テレメトリーの次元だけを context に入れ、
	// 処理の入力である user ID は引数で渡す。
	ctx = withRequestInfo(ctx, requestInfo{
		Country:    "JP",
		Plan:       "premium",
		OSName:     "Android",
		OSVersion:  "14",
		AppName:    "ExampleApp",
		AppVersion: "2.3.1",
		NewProfile: true,
	})

	// エラーの記録は fetchUser の中で済んでいる。ここでは終了コードにだけ反映する。
	_, fetchErr := s.fetchUser(ctx, "alice")

	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := tel.shutdown(sctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown telemetry: %v\n", err)
	}

	if fetchErr != nil {
		fmt.Fprintf(os.Stderr, "fetch user: %v\n", fetchErr)
		return 1
	}
	return 0
}
