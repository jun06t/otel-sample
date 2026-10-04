package main

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
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
//
// endpoint が空なら stdout に出す。出力の並びで「いつ・どの信号に載るか」を見せたいので、
// どちらも同期で export する(LogRecord は Emit した瞬間に、span は End した瞬間に出力される)。
//
// endpoint があれば、OTLP gRPC で SigNoz などのバックエンドに送る。こちらは本番と同じくバッチで送る。
func newTelemetry(ctx context.Context, endpoint string) (*telemetry, error) {
	// resource.Default() の telemetry.sdk.* 属性を残したまま service.name を足す。
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, err
	}

	var (
		spanProcessor sdktrace.SpanProcessor
		logProcessor  sdklog.Processor
	)
	if endpoint == "" {
		te, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		le, err := stdoutlog.New(stdoutlog.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		spanProcessor = sdktrace.NewSimpleSpanProcessor(te)
		logProcessor = sdklog.NewSimpleProcessor(le)
	} else {
		te, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		le, err := otlploggrpc.New(ctx,
			otlploggrpc.WithEndpoint(endpoint),
			otlploggrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		spanProcessor = sdktrace.NewBatchSpanProcessor(te)
		logProcessor = sdklog.NewBatchProcessor(le)
	}

	tp := sdktrace.NewTracerProvider(
		// context のリクエスト文脈を、このサービスのルート span に属性として付ける。
		sdktrace.WithSpanProcessor(newRequestInfoProcessor()),
		sdktrace.WithSpanProcessor(spanProcessor),
		sdktrace.WithResource(res),
	)
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(logProcessor),
		sdklog.WithResource(res),
	)

	// アプリのコードは provider から直接受け取るが、グローバルの provider を使う
	// 計装ライブラリ(otelhttp など)のためにグローバルにも登録しておく。
	otel.SetTracerProvider(tp)
	global.SetLoggerProvider(lp)

	return &telemetry{tracerProvider: tp, loggerProvider: lp}, nil
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
