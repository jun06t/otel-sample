package main

import (
	"context"
	"time"

	otellog "go.opentelemetry.io/otel/log"
)

// emitEvent は event.name を持つ LogRecord(= log-based event)を emit する。
//
// zap では event.name を表現できないため、ここだけ OTel Logs API を直接使う。
// これは「普通のログ」と同じ logs 信号(= 経路B の substrate)を流れ、違いは event.name の
// 有無だけ。event.name が付いた LogRecord は event record として解釈される
// (span.AddEvent の後継。OTel は 2026 に span event API を deprecated とし、こちらへ移行)。
//
// ctx に載った span から trace_id/span_id が LogRecord に自動で紐づくため、SigNoz 上で
// その span と同じ画面に並ぶ。
func emitEvent(ctx context.Context, logger otellog.Logger, name string, sev otellog.Severity, attrs ...otellog.KeyValue) {
	var rec otellog.Record
	rec.SetEventName(name) // これが付くと event record になる
	rec.SetSeverity(sev)
	rec.SetTimestamp(time.Now())
	rec.AddAttributes(attrs...)
	logger.Emit(ctx, rec)
}
