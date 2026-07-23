package handler

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// Handler は受信メッセージを処理する。
type Handler struct {
	serviceName string
}

func New(serviceName string) *Handler {
	return &Handler{serviceName: serviceName}
}

// Handle のビジネス処理は subscribe span の子 span になる。
//
// Receive のコールバックに渡ってくる ctx には、SDK が張った
// 「そのメッセージの subscribe span」が入っている。ここで tracer.Start(ctx, ...) すれば
// その span の子として自然にぶら下がる（ctx を引き継ぐだけ）。
func (h *Handler) Handle(ctx context.Context, m *pubsub.Message) error {
	_, span := otel.Tracer(h.serviceName).Start(ctx, "handle-message")
	defer span.End()

	span.SetAttributes(
		attribute.String("message.id", m.ID),
		attribute.Int("message.size", len(m.Data)),
	)
	log.Printf("processing message: id=%s data=%q", m.ID, string(m.Data))
	time.Sleep(30 * time.Millisecond) // 疑似処理
	return nil
}
