package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

const (
	serviceName = "event-comparison"
	scopeName   = "github.com/jun06t/otel-sample/event-comparison"
)

// styles は同じシナリオを 3 通りの置き場で記録する実装。
var styles = map[string]func(ctx context.Context, userID string, fail bool) error{
	"wide": wideEvent,     // span 属性に集約する
	"span": spanEvent,     // span.AddEvent で親 span に注釈する
	"log":  logBasedEvent, // event.name 付き LogRecord を出す
}

func main() {
	style := flag.String("style", "wide", "記録スタイル: wide | span | log")
	fail := flag.Bool("fail", false, "DB クエリをリトライ上限まで失敗させる")
	flag.Parse()

	run, ok := styles[*style]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown style %q (wide | span | log)\n", *style)
		os.Exit(2)
	}

	ctx := context.Background()
	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		panic(err)
	}
	defer shutdown(ctx)

	// エラーは各スタイルの中で記録済みなので、ここでは捨てる。
	_ = run(ctx, "alice", *fail)
}
