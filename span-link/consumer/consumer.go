// Package consumer は、まとめて受信した複数メッセージを 1 本の receive span で表現し、
// 各メッセージの発行元トレースへ span link で連結する（messaging semconv 準拠のバッチ処理）。
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

// ConsumeBatch は受信済みメッセージ群を、1 本の receive span（batch pull 操作）と
// その配下の process span（各メッセージの処理）で表現する。
//
// ★ span link は receive span に張る。
//
//	receive は「1 回の操作で複数メッセージをまとめて取り出す」発行元横断の操作であり、
//	span は親を 1 つしか持てないため、複数の発行元トレースへの相関手段は link に限られる
//	（messaging semconv 準拠）。これにより subscribe(receive) 側と publish(send) 側が
//	link で繋がる。
func ConsumeBatch(ctx context.Context, tracerName, destination string, msgs []*pubsub.Message) {
	tracer := otel.Tracer(tracerName)
	prop := otel.GetTextMapPropagator()

	// 各メッセージ属性から発行元の SpanContext を Extract し、link を組み立てる。
	links := make([]trace.Link, 0, len(msgs))
	for _, m := range msgs {
		parentCtx := prop.Extract(context.Background(), propagation.MapCarrier(m.Attributes))
		links = append(links, trace.Link{
			SpanContext: trace.SpanContextFromContext(parentCtx),
			Attributes:  []attribute.KeyValue{semconv.MessagingMessageID(m.ID)},
		})
	}

	// receive span: バッチ取り出し操作。★ここに N 件の link を張り、発行元(publish)と連結する。
	ctx, receiveSpan := tracer.Start(ctx, "receive "+destination,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithLinks(links...), // ★ 1 本の receive span に N 件の link（1:N）
		trace.WithAttributes(
			semconv.MessagingSystemGCPPubSub,
			semconv.MessagingDestinationName(destination),
			semconv.MessagingOperationTypeReceive,
			semconv.MessagingOperationName("receive"),
			semconv.MessagingBatchMessageCount(len(msgs)),
		),
	)
	defer receiveSpan.End()

	log.Printf("received a batch of %d messages (linked to their producers)", len(msgs))

	// 各メッセージを process span（receive span の子）で処理する。
	for _, m := range msgs {
		_, span := tracer.Start(ctx, "process "+destination,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				semconv.MessagingSystemGCPPubSub,
				semconv.MessagingDestinationName(destination),
				semconv.MessagingOperationTypeProcess,
				semconv.MessagingOperationName("process"),
				semconv.MessagingMessageID(m.ID),
			),
		)
		time.Sleep(10 * time.Millisecond) // 疑似処理
		span.End()
	}
}
