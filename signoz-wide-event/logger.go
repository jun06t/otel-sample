package main

import (
	"context"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	scopeName = "github.com/jun06t/otel-sample/signoz-wide-event"
	// contextKey は otelzap ブリッジに context を渡すためのフィールドキー。
	// stdout には出さないので dropFieldCore で除去する。
	contextKey = "context"
)

// NewLogger は 2 系統に書き込む Tee ロガーを返す。
//   - stdout(JSON): ローカル確認や span 外(起動など)のログ用
//   - otelzap:      LogRecord として OTLP で SigNoz に送り、trace context で span と相関させる
//
// otelzap は公式ブリッジ (go.opentelemetry.io/contrib/bridges/otelzap)。
// 同名の uptrace 版 (span.AddEvent に書く) とは別物なので注意。
func NewLogger(lp *sdklog.LoggerProvider) *zap.Logger {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "timestamp"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// stdout 側は otelzap 用の raw context フィールドを除去してから出力する。
	stdoutCore := dropFieldCore{
		Core: zapcore.NewCore(
			zapcore.NewJSONEncoder(encCfg),
			zapcore.AddSync(os.Stdout),
			zapcore.InfoLevel,
		),
		key: contextKey,
	}
	otelCore := otelzap.NewCore(scopeName, otelzap.WithLoggerProvider(lp))

	return zap.New(zapcore.NewTee(stdoutCore, otelCore), zap.AddCaller())
}

// Ctx は現在の span に紐づく子ロガーを返す。
//
//   - otelzap 側: フィールドで渡した context.Context をブリッジが検出し、送信する
//     LogRecord のネイティブな trace_id / span_id を埋める(=SigNoz が相関に使う)。
//   - stdout 側: 上記 context は dropFieldCore で除去し、人間が追えるよう
//     trace_id / span_id を文字列フィールドとして載せる。
func Ctx(ctx context.Context, l *zap.Logger) *zap.Logger {
	fields := []zap.Field{zap.Any(contextKey, ctx)} // otelzap が拾って自動相関する
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(fields,
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}
	return l.With(fields...)
}

// dropFieldCore は指定キーのフィールドを除去してから内側の Core に委譲する zapcore.Core。
// otelzap にだけ渡したい raw context を stdout の JSON から除くために使う。
type dropFieldCore struct {
	zapcore.Core
	key string
}

func (c dropFieldCore) With(fields []zapcore.Field) zapcore.Core {
	return dropFieldCore{Core: c.Core.With(c.filter(fields)), key: c.key}
}

// Check は自身(ラッパ)を CheckedEntry に登録し、Write が必ずこの Core を通るようにする。
func (c dropFieldCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c dropFieldCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	return c.Core.Write(ent, c.filter(fields))
}

func (c dropFieldCore) filter(fields []zapcore.Field) []zapcore.Field {
	out := make([]zapcore.Field, 0, len(fields))
	for _, f := range fields {
		if f.Key == c.key {
			continue
		}
		out = append(out, f)
	}
	return out
}
