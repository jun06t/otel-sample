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

	ctx := context.Background()
	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup telemetry: %v\n", err)
		return 1
	}

	var events eventRecorder = logEvents{}
	if *spanEvents {
		events = spanEventRecorder{}
	}
	s := &userService{
		cache:  &cache{},
		db:     &userDB{fail: *fail},
		events: events,
	}

	// エラーの記録は fetchUser の中で済んでいる。ここでは終了コードにだけ反映する。
	_, fetchErr := s.fetchUser(ctx, request{
		UserID:     "alice",
		Country:    "JP",
		Plan:       "premium",
		OSName:     "Android",
		OSVersion:  "14",
		AppName:    "ExampleApp",
		AppVersion: "2.3.1",
		NewProfile: true,
	})

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
