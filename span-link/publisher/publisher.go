// Package publisher は、各メッセージを「それぞれ別トレース」の PRODUCER span として publish する。
// publish 時に traceparent をメッセージ属性へ inject し、consumer 側で span link の対象にできるようにする。
package publisher

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// Publish は n 件のメッセージを、それぞれ独立したトレースの PRODUCER span として publish する。
func Publish(ctx context.Context, tracerName, destination string, topic *pubsub.Topic, n int) error {
	tracer := otel.Tracer(tracerName)
	prop := otel.GetTextMapPropagator()

	for i := range n {
		// ★各メッセージを別トレースにするため、親を持たない context.Background() から span を開始する。
		msgCtx, span := tracer.Start(context.Background(), "send "+destination,
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(
				semconv.MessagingSystemGCPPubSub,
				semconv.MessagingDestinationName(destination),
				semconv.MessagingOperationTypeSend,
				semconv.MessagingOperationName("send"),
			),
		)

		msg := &pubsub.Message{
			Data:       fmt.Appendf(nil, "message-%d", i),
			Attributes: map[string]string{},
		}
		// ★ traceparent をメッセージ属性に inject。consumer 側はこれを Extract して link 先にする。
		prop.Inject(msgCtx, propagation.MapCarrier(msg.Attributes))

		id, err := topic.Publish(msgCtx, msg).Get(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "publish failed")
			span.End()
			return err
		}
		span.SetAttributes(semconv.MessagingMessageID(id))
		span.End()
	}
	return nil
}
