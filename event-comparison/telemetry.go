package main

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/contrib/exporters/autoexport"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// telemetry は traces と logs の provider をまとめたもの。
// アプリのコードは tracer / logger をここから受け取り、コンストラクタで各構造体に渡す。
type telemetry struct {
	tracerProvider *sdktrace.TracerProvider
	loggerProvider *sdklog.LoggerProvider
}

// newTelemetry は traces と logs の provider を作る。
func newTelemetry(ctx context.Context) (*telemetry, error) {
	// resource.Default() の telemetry.sdk.* 属性を残したまま service.name を足す。
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, err
	}

	spanProcessor, logProcessor, err := newProcessors(ctx)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		// context のリクエスト文脈を、このサービスのルート span に属性として付ける。
		sdktrace.WithSpanProcessor(newRequestInfoProcessor()),
		sdktrace.WithSpanProcessor(spanProcessor),
		sdktrace.WithResource(res),
	)

	logOpts := []sdklog.LoggerProviderOption{sdklog.WithResource(res)}
	if os.Getenv("COPY_EVENT_NAME_TO_ATTRIBUTE") == "true" {
		// export する processor より前に登録し、書き換えた LogRecord が渡るようにする。
		logOpts = append(logOpts, sdklog.WithProcessor(newEventNameAttrProcessor()))
	}
	logOpts = append(logOpts, sdklog.WithProcessor(logProcessor))
	lp := sdklog.NewLoggerProvider(logOpts...)

	// アプリのコードは provider から直接受け取るが、グローバルの provider を使う
	// 計装ライブラリ(otelhttp など)のためにグローバルにも登録しておく。
	otel.SetTracerProvider(tp)
	otel.SetLoggerProvider(lp)

	return &telemetry{tracerProvider: tp, loggerProvider: lp}, nil
}

// newProcessors は export する processor を作る。
//
// 送り先は autoexport が標準の環境変数から決める。
//   - OTEL_TRACES_EXPORTER / OTEL_LOGS_EXPORTER: otlp / console / none
//   - OTEL_EXPORTER_OTLP_ENDPOINT、OTEL_EXPORTER_OTLP_PROTOCOL など: OTLP の送り先と方式
//
// OTEL_TRACES_EXPORTER が未設定なら stdout に整形して出す。このときは出力の並びで
// 「いつ・どの信号に載るか」を見せたいので同期で export する(LogRecord は Emit した瞬間に、
// span は End した瞬間に出力される)。それ以外は本番と同じくバッチで送る。
func newProcessors(ctx context.Context) (sdktrace.SpanProcessor, sdklog.Processor, error) {
	te, err := autoexport.NewSpanExporter(ctx, autoexport.WithFallbackSpanExporter(
		func(context.Context) (sdktrace.SpanExporter, error) {
			return stdouttrace.New(stdouttrace.WithPrettyPrint())
		},
	))
	if err != nil {
		return nil, nil, err
	}
	le, err := autoexport.NewLogExporter(ctx, autoexport.WithFallbackLogExporter(
		func(context.Context) (sdklog.Exporter, error) {
			return stdoutlog.New(stdoutlog.WithPrettyPrint())
		},
	))
	if err != nil {
		return nil, nil, err
	}

	if os.Getenv("OTEL_TRACES_EXPORTER") == "" {
		return sdktrace.NewSimpleSpanProcessor(te), sdklog.NewSimpleProcessor(le), nil
	}
	return sdktrace.NewBatchSpanProcessor(te), sdklog.NewBatchProcessor(le), nil
}

func (t *telemetry) tracer() trace.Tracer {
	return t.tracerProvider.Tracer(scopeName)
}

func (t *telemetry) logger() log.Logger {
	return t.loggerProvider.Logger(scopeName)
}

// shutdown は溜まっているデータを書き出してから provider を閉じる。
func (t *telemetry) shutdown(ctx context.Context) error {
	return errors.Join(t.tracerProvider.Shutdown(ctx), t.loggerProvider.Shutdown(ctx))
}
