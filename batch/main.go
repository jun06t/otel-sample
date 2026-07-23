package main

import (
	"context"
	"log"
	"os"

	"github.com/jun06t/otel-sample/batch/job"
	"github.com/jun06t/otel-sample/batch/telemetry"
)

func main() {
	// ★肝1: os.Exit は defer を実行しない。
	//        終了コードは run() に集約し、flush(Shutdown) を必ず通す。
	os.Exit(run())
}

func run() (exitCode int) {
	ctx := context.Background()

	conf, err := LoadConfig()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		return 1
	}

	tp, shutdown, err := telemetry.NewTracerProvider(ctx, conf.ExporterEndpoint, conf.ServiceName)
	if err != nil {
		log.Printf("failed to init tracer provider: %v", err)
		return 1
	}
	_ = tp

	// ★肝2: 短命プロセスなので終了前に必ず flush + shutdown。
	//        忘れると溜まった span が送信されずに消える。
	//        run() 経由の return なので、この defer は必ず実行される。
	defer shutdown()

	if err := job.Run(ctx, conf.ServiceName); err != nil {
		log.Printf("batch job failed: %v", err)
		return 1 // return 経由なので上の defer(shutdown) は実行される
	}
	return 0
}
