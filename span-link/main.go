package main

import (
	"context"
	"log"
	"sync"

	"cloud.google.com/go/pubsub"

	"github.com/jun06t/otel-sample/span-link/consumer"
	"github.com/jun06t/otel-sample/span-link/publisher"
	"github.com/jun06t/otel-sample/span-link/telemetry"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	conf, err := LoadConfig()
	if err != nil {
		return err
	}

	_, shutdown, err := telemetry.NewTracerProvider(ctx, conf.ExporterEndpoint, conf.ServiceName)
	if err != nil {
		return err
	}
	defer shutdown()

	client, err := pubsub.NewClient(ctx, conf.ProjectID)
	if err != nil {
		return err
	}
	defer client.Close()

	topic, sub, err := setup(ctx, client, conf.TopicID, conf.SubscriptionID)
	if err != nil {
		return err
	}
	defer topic.Stop()

	// 1) N 件のメッセージを、それぞれ別トレースの PRODUCER span として publish。
	log.Printf("publishing %d messages (each in its own trace) ...", conf.BatchSize)
	if err := publisher.Publish(ctx, conf.ServiceName, conf.TopicID, topic, conf.BatchSize); err != nil {
		return err
	}

	// 2) BatchSize 件をまとめて受信する。
	msgs, err := receiveBatch(ctx, sub, conf.BatchSize)
	if err != nil {
		return err
	}

	// 3) 各メッセージを 1 件ずつ process span で処理する。
	//    各 process span は別トレースになり、その発行元(publish)へ span link で 1:1 に紐付く。
	//    destination は消費側なのでサブスクリプションを渡す。
	consumer.ConsumeMessages(conf.ServiceName, conf.SubscriptionID, msgs)

	log.Printf("done. consumed %d messages (each process linked 1:1 to its producer)", len(msgs))
	return nil
}

// setup はデモ用に topic / subscription を用意する（本番では事前作成済み）。
func setup(ctx context.Context, client *pubsub.Client, topicID, subID string) (*pubsub.Topic, *pubsub.Subscription, error) {
	topic, err := client.CreateTopic(ctx, topicID)
	if err != nil {
		topic = client.Topic(topicID) // 既に存在する場合
	}
	sub, err := client.CreateSubscription(ctx, subID, pubsub.SubscriptionConfig{Topic: topic})
	if err != nil {
		sub = client.Subscription(subID) // 既に存在する場合
	}
	return topic, sub, nil
}

// receiveBatch は batchSize 件を受信して返す。
// 溜まった時点で Receive をキャンセルして抜ける。
func receiveBatch(ctx context.Context, sub *pubsub.Subscription, batchSize int) ([]*pubsub.Message, error) {
	sub.ReceiveSettings.Synchronous = true
	sub.ReceiveSettings.MaxOutstandingMessages = batchSize

	var (
		mu        sync.Mutex
		collected = make([]*pubsub.Message, 0, batchSize)
	)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := sub.Receive(cctx, func(_ context.Context, m *pubsub.Message) {
		mu.Lock()
		collected = append(collected, m)
		done := len(collected) >= batchSize
		mu.Unlock()

		m.Ack()
		if done {
			cancel() // 必要数そろったら Receive を終了させる
		}
	})
	if err != nil {
		return nil, err
	}
	return collected, nil
}
