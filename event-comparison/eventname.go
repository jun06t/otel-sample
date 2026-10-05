package main

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// eventNameAttrProcessor は、LogRecord の EventName を event.name 属性にもコピーする。
//
// SigNoz は LogRecord の EventName を保存しない(SigNoz/signoz#8140)ので、SigNoz 上で
// log-based event を名前で絞り込めるようにするための、それまでのつなぎ。
// event.name 属性は semconv では非推奨なので、送り先が EventName に対応したら外す。
//
// SDK は processor を登録順に呼び、OnEmit で同期的に書き換えた内容は次の processor に渡る。
// そのため export する processor より前に登録する。
type eventNameAttrProcessor struct{}

var _ sdklog.Processor = (*eventNameAttrProcessor)(nil)

func newEventNameAttrProcessor() *eventNameAttrProcessor {
	return &eventNameAttrProcessor{}
}

func (p *eventNameAttrProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool {
	return true
}

func (p *eventNameAttrProcessor) OnEmit(_ context.Context, r *sdklog.Record) error {
	if name := r.EventName(); name != "" {
		r.AddAttributes(attribute.String("event.name", name))
	}
	return nil
}

func (p *eventNameAttrProcessor) Shutdown(context.Context) error { return nil }

func (p *eventNameAttrProcessor) ForceFlush(context.Context) error { return nil }
