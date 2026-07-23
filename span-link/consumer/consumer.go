// Package consumer は、受信した各メッセージを、その発行元トレースへ span link で
// 1:1 に連結した process span で処理する（messaging semconv 準拠）。
package consumer

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// ConsumeMessages は各メッセージを 1 件ずつ process span で処理する。
//
// ★ 各 process span は「親を持たない新規 root（＝発行元とは別トレース）」として開始し、
//
//	発行元(publish)へは parent-child ではなく span link を 1 本張って 1:1 で紐付ける。
//	発行元と消費側を別トレースに保ったまま相関できるのが span link の役割（messaging semconv 準拠）。
//	常駐ワーカーのようにメッセージを 1 件ずつ捌く実運用に近い形。
func ConsumeMessages(tracerName, destination string, msgs []*pubsub.Message) {
	tracer := otel.Tracer(tracerName)
	prop := otel.GetTextMapPropagator()

	for _, m := range msgs {
		// メッセージ属性から発行元の SpanContext を Extract する。
		producerCtx := prop.Extract(context.Background(), propagation.MapCarrier(m.Attributes))

		// ★ context.Background() から開始 → 発行元とは別トレース（新規 root）。
		//    その発行元へ span link を 1 本張る（publish ↔ process が 1:1 で対応）。
		_, span := tracer.Start(context.Background(), "process "+destination,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithLinks(trace.Link{
				SpanContext: trace.SpanContextFromContext(producerCtx),
				Attributes:  []attribute.KeyValue{semconv.MessagingMessageID(m.ID)},
			}),
			trace.WithAttributes(
				semconv.MessagingSystemGCPPubSub,
				semconv.MessagingDestinationName(destination),
				semconv.MessagingOperationTypeProcess,
				semconv.MessagingOperationName("process"),
				semconv.MessagingMessageID(m.ID),
			),
		)
		log.Printf("processing message id=%s (linked 1:1 to its producer)", m.ID)
		time.Sleep(10 * time.Millisecond) // 疑似処理
		span.End()
	}
}
