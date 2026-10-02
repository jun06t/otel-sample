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

// styles は同じシナリオを 3 通りの置き場で記録する実装。
var styles = map[string]func(ctx context.Context, userID string, fail bool) error{
	"wide": wideEvent,     // span 属性に集約する
	"span": spanEvent,     // span.AddEvent で親 span に注釈する
	"log":  logBasedEvent, // EventName 付きの LogRecord を出す
}

func main() {
	os.Exit(run())
}

// run は終了コードを返す。os.Exit は defer を実行しないため、
// shutdown は main ではなくここで済ませる。
func run() int {
	style := flag.String("style", "wide", "記録スタイル: wide | span | log")
	fail := flag.Bool("fail", false, "DB クエリをリトライ上限まで失敗させる")
	flag.Parse()

	fetch, ok := styles[*style]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown style %q (wide | span | log)\n", *style)
		return 2
	}

	ctx := context.Background()
	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup telemetry: %v\n", err)
		return 1
	}

	// エラーの記録は各スタイルの中で済んでいる。ここでは終了コードにだけ反映する。
	fetchErr := fetch(ctx, "alice", *fail)

	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := shutdown(sctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown telemetry: %v\n", err)
	}

	if fetchErr != nil {
		fmt.Fprintf(os.Stderr, "fetch user: %v\n", fetchErr)
		return 1
	}
	return 0
}
