package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"cloud.google.com/go/pubsub"

	"github.com/jun06t/otel-sample/worker/handler"
	psbootstrap "github.com/jun06t/otel-sample/worker/pubsub"
	"github.com/jun06t/otel-sample/worker/telemetry"
)

func main() {
	// SIGINT/SIGTERM で ctx をキャンセル → Receive が抜ける（graceful shutdown）。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conf, err := LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// ★肝1: グローバルに provider と propagator を設定する（propagator は telemetry の init で設定）。
	//        Pub/Sub SDK の自動計装はこのグローバル設定に乗って動く。
	_, shutdown, err := telemetry.NewTracerProvider(ctx, conf.ExporterEndpoint, conf.ServiceName)
	if err != nil {
		log.Fatalf("failed to init tracer provider: %v", err)
	}
	defer shutdown()

	// ★肝2: SDK の OTel 自動計装を有効化する。
	client, err := pubsub.NewClientWithConfig(ctx, conf.ProjectID, &pubsub.ClientConfig{
		EnableOpenTelemetryTracing: true,
	})
	if err != nil {
		log.Fatalf("failed to create pubsub client: %v", err)
	}
	defer client.Close()

	// デモ用: エミュレータ上に topic / subscription を用意し、テストメッセージを投げる。
	if conf.Bootstrap {
		if err := psbootstrap.Bootstrap(ctx, client, conf.TopicID, conf.SubscriptionID, 3); err != nil {
			log.Fatalf("bootstrap failed: %v", err)
		}
	}

	h := handler.New(conf.ServiceName)
	sub := client.Subscription(conf.SubscriptionID)
	log.Printf("worker started. waiting for messages on %q ... (Ctrl-C で終了)", conf.SubscriptionID)

	// Receive はメッセージごとにコールバックを呼ぶ。
	// 渡ってくる ctx には SDK が張った「そのメッセージの subscribe span」が入っている。
	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		if err := h.Handle(ctx, m); err != nil {
			log.Printf("handle failed: %v", err)
			m.Nack()
			return
		}
		m.Ack()
	})
	if err != nil {
		log.Printf("receive stopped: %v", err)
	}
}
