package main

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	tracer = otel.Tracer(scopeName)
	logger = global.Logger(scopeName)
)

// setupTelemetry は traces と logs を stdout に出す provider をグローバルに登録する。
//
// 出力の並びで「いつ・どの信号に載るか」を見せたいので、どちらも同期で export する。
// LogRecord は Emit した瞬間に、span は End した瞬間に出力される。
// (WithSyncer / SimpleProcessor は本番向けではない。本番は Batch を使う)
func setupTelemetry(ctx context.Context) (func(context.Context) error, error) {
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

	otel.SetTracerProvider(tp)
	global.SetLoggerProvider(lp)

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), lp.Shutdown(ctx))
	}, nil
}
