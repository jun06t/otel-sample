package main

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

// newTelemetry は traces と logs を stdout に出す provider を作る。
//
// 出力の並びで「いつ・どの信号に載るか」を見せたいので、どちらも同期で export する。
// LogRecord は Emit した瞬間に、span は End した瞬間に出力される。
// (WithSyncer / SimpleProcessor は本番向けではない。本番は Batch を使う)
func newTelemetry() (*telemetry, error) {
	// resource.Default() の telemetry.sdk.* 属性を残したまま service.name を足す。
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, err
	}

	te, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		// context のリクエスト文脈を、このサービスのルート span に属性として付ける。
		sdktrace.WithSpanProcessor(requestInfoProcessor{}),
		sdktrace.WithSyncer(te),
		sdktrace.WithResource(res),
	)

	le, err := stdoutlog.New(stdoutlog.WithPrettyPrint())
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(le)),
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
