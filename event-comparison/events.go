package main

import (
	"context"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// eventRecorder は「名前の付いた時点の出来事」を記録する。
//
// logEvents(既定)と spanEventRecorder(-span-events)は、同じ event を
// 別の API で書くだけで、記録する内容は同じ。違いは出力される信号と形だけ。
type eventRecorder interface {
	// event は名前の付いた時点の出来事を記録する。err があればそのメッセージも残す。
	event(ctx context.Context, name string, sev log.Severity, err error, attrs ...attribute.KeyValue)
	// exception は呼び出し元に返す例外を記録する。name は「操作名 + .exception」。
	exception(ctx context.Context, name string, err error)
}

// logEvents は EventName 付きの LogRecord(= log-based event)として logs 信号に出す。
// ctx に span があれば trace_id / span_id が自動で付く。
type logEvents struct{}

func (logEvents) event(ctx context.Context, name string, sev log.Severity, err error, attrs ...attribute.KeyValue) {
	var r log.Record
	r.SetEventName(name) // これが空なら普通のログ
	r.SetTimestamp(time.Now())
	r.SetSeverity(sev)
	if err != nil {
		// 非推奨の error.message 属性ではなく Body に入れる。
		r.SetBody(attribute.StringValue(err.Error()))
	}
	r.AddAttributes(attrs...)
	logger.Emit(ctx, r)
}

// exception は semconv の Exceptions in logs に従い、exception.* 属性を付けて ERROR で出す。
func (l logEvents) exception(ctx context.Context, name string, err error) {
	l.event(ctx, name, log.SeverityError, err,
		attribute.String("exception.type", errorType(err)),
		attribute.String("exception.message", err.Error()),
		attribute.String("exception.stacktrace", string(debug.Stack())),
	)
}

// spanEventRecorder は従来の span event(AddEvent / RecordError)として、ctx の span に注釈する。
//
// OTel は 2026-03 に Span Event API を段階的に非推奨にする方針を発表し、
// 新しいイベントには Logs API を推奨している。比較のために残している実装。
// span event には severity も Body もないので、sev と err のメッセージは記録されない
// (エラーの種類は呼び出し側が error.type 属性で渡している)。
type spanEventRecorder struct{}

func (spanEventRecorder) event(ctx context.Context, name string, _ log.Severity, _ error, attrs ...attribute.KeyValue) {
	trace.SpanFromContext(ctx).AddEvent(name, trace.WithAttributes(attrs...))
}

// exception は "exception" という名前の span event になる。name は span event では使えない。
func (spanEventRecorder) exception(ctx context.Context, _ string, err error) {
	trace.SpanFromContext(ctx).RecordError(err, trace.WithStackTrace(true))
}
