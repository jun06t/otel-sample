// Package consumer は、まとめて受信した複数メッセージを 1 本の process span で処理する。
// parent-child ではなく span link で各メッセージの発行元トレースへ連結するのがポイント。
package consumer

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// ProcessBatch は受信済みメッセージ群を 1 本の CONSUMER span で処理する。
//
// span は各メッセージの発行元 SpanContext への span link を持つ。
// span は 1 つの親しか持てないため、複数メッセージ（＝複数の発行元トレース）を
// 束ねて処理するバッチ処理では link が唯一の相関手段になる（messaging semconv 準拠）。
func ProcessBatch(ctx context.Context, tracerName, destination string, msgs []*pubsub.Message) {
	tracer := otel.Tracer(tracerName)
	prop := otel.GetTextMapPropagator()

	// 各メッセージ属性から発行元の SpanContext を Extract し、link を組み立てる。
	links := make([]trace.Link, 0, len(msgs))
	for _, m := range msgs {
		parentCtx := prop.Extract(context.Background(), propagation.MapCarrier(m.Attributes))
		links = append(links, trace.Link{
			SpanContext: trace.SpanContextFromContext(parentCtx),
			Attributes: []attribute.KeyValue{
				attribute.String("messaging.message.id", m.ID),
			},
		})
	}

	_, span := tracer.Start(ctx, "process "+destination,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithLinks(links...), // ★ 1 本の span に N 件の link
		trace.WithAttributes(
			attribute.String("messaging.system", "gcp_pubsub"),
			attribute.String("messaging.destination.name", destination),
			attribute.String("messaging.operation.type", "process"),
			attribute.String("messaging.operation.name", "process"),
			attribute.Int("messaging.batch.message_count", len(msgs)),
		),
	)
	defer span.End()

	log.Printf("processing batch of %d messages with span links", len(msgs))
	time.Sleep(30 * time.Millisecond) // 疑似バッチ処理
}
