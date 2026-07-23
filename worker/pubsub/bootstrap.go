// Package pubsub はデモ用のブートストラップ処理を提供する。
// 本番では topic/subscription は事前に用意されるため、この処理は不要。
package pubsub

import (
	"context"
	"log"

	"cloud.google.com/go/pubsub"
)

// Bootstrap はエミュレータ用に topic / subscription を用意し、テストメッセージを publish する。
// 既に存在する場合は無視して続行する。
func Bootstrap(ctx context.Context, client *pubsub.Client, topicID, subscriptionID string, numMessages int) error {
	topic, err := client.CreateTopic(ctx, topicID)
	if err != nil {
		topic = client.Topic(topicID) // 既に存在する場合
	}

	if _, err := client.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{
		Topic: topic,
	}); err != nil {
		log.Printf("create subscription (既存なら無視): %v", err)
	}

	// テストメッセージを publish（publish 側にも span が付く）。
	for range numMessages {
		topic.Publish(ctx, &pubsub.Message{Data: []byte("hello")})
	}
	topic.Stop() // publish のフラッシュを待つ
	return nil
}
