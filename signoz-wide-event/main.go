package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jun06t/otel-sample/signoz-wide-event/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	serviceName  = "signoz-wide-event"
	amountLimit  = 10000
	listenAddr   = ":8000"
	defaultAgent = "localhost:4317"
)

var tracer trace.Tracer

func main() {
	ctx := context.Background()
	endpoint := getenv("EXPORTER_ENDPOINT", defaultAgent)

	// traces と logs で同じ resource を共有し、SigNoz 上で service 次元も揃える。
	res := telemetry.NewResource(serviceName, "1.0.0", "local")

	_, traceCleanup, err := telemetry.NewTracerProvider(ctx, endpoint, res, 1.0)
	if err != nil {
		panic(err)
	}
	defer traceCleanup()

	lp, logCleanup, err := telemetry.NewLoggerProvider(ctx, endpoint, res)
	if err != nil {
		panic(err)
	}
	defer logCleanup()

	tracer = otel.Tracer(scopeName)
	logger := NewLogger(lp)
	defer logger.Sync()

	// events は event.name 付き LogRecord(= log-based event)を出すための OTel Logs API の Logger。
	events := lp.Logger(scopeName)

	// span の外(起動ログ)は通常の logger で stdout に出す。
	// これらは相関すべき trace が存在しないので wide event には載せられない。
	logger.Info("starting server", zap.String("addr", listenAddr), zap.String("exporter", endpoint))

	h := &handler{logger: logger, events: events}
	mux := http.NewServeMux()
	mux.Handle("/order", http.HandlerFunc(h.order))

	srv := otelhttp.NewHandler(mux, "server",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	)
	if err := http.ListenAndServe(listenAddr, srv); err != nil {
		logger.Error("server stopped", zap.Error(err))
	}
}

type handler struct {
	logger *zap.Logger
	events otellog.Logger
}

// order は注文処理を模した handler。
//
// 正常系: 作業単位の文脈をすべて span 属性に載せる(= wide event)。途中経過のログ行は出さない。
// 異常系: 「失敗した事実」を span.SetStatus に、「エラーの中身」を logger.Error に振り分ける。
//
//	例: curl 'localhost:8000/order?user=alice&items=2&amount=3000'   -> 正常
//	    curl 'localhost:8000/order?user=bob&items=0&amount=3000'      -> validation error
//	    curl 'localhost:8000/order?user=carol&items=1&amount=99999'   -> charge error
func (h *handler) order(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	userID := getquery(r, "user", "anonymous")
	items := atoi(r.URL.Query().Get("items"), 1)
	amount := atoi(r.URL.Query().Get("amount"), 1000)

	// --- wide event: この作業単位の文脈を高カーディナリティ属性として span に持たせる ---
	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.Int("order.items", items),
		attribute.Int("order.amount", amount),
	)

	if err := h.validate(ctx, items); err != nil {
		h.fail(ctx, span, w, "validation failed", err)
		return
	}
	if err := h.charge(ctx, amount); err != nil {
		h.fail(ctx, span, w, "charge failed", err)
		return
	}

	span.SetAttributes(attribute.String("order.status", "charged"))
	fmt.Fprintf(w, "ok: user=%s items=%d amount=%d\n", userID, items, amount)
}

func (h *handler) validate(ctx context.Context, items int) error {
	_, span := tracer.Start(ctx, "validate")
	defer span.End()
	time.Sleep(20 * time.Millisecond)
	if items <= 0 {
		return errors.New("items must be positive")
	}
	return nil
}

func (h *handler) charge(ctx context.Context, amount int) error {
	_, span := tracer.Start(ctx, "charge")
	defer span.End()
	time.Sleep(50 * time.Millisecond)
	if amount > amountLimit {
		return fmt.Errorf("amount %d exceeds limit %d", amount, amountLimit)
	}
	return nil
}

// fail は異常系の役割分担を示す。
//   - span.SetStatus(Error): 失敗した“事実”を span に一級で刻む(trace が赤くなり status=error で検索できる)。= wide event
//   - logger.Error:          エラーの“中身”(message)を「普通のログ」(severity=ERROR, event.name 無し)として送る。
//   - emitEvent:             同じ内容を「log-based event」(event.name 付き)としても送る。
//
// 下 2 つは同じ logs 信号(= 経路B の substrate)を流れ、違いは event.name の有無だけ。
// あえて span.RecordError は使っていない。OTel は 2026 に span event(RecordException 含む)を
// deprecated とし、イベントは log-based event で表現する方針に切り替えたため、logs 側に寄せている。
func (h *handler) fail(ctx context.Context, span trace.Span, w http.ResponseWriter, msg string, err error) {
	span.SetStatus(codes.Error, msg)
	span.SetAttributes(attribute.String("order.status", "failed"))

	// 普通のログ(event.name 無し)
	Ctx(ctx, h.logger).Error(msg, zap.Error(err))

	// log-based event(event.name 付き) — span.AddEvent の後継形
	emitEvent(ctx, h.events, "order.failed", otellog.SeverityError,
		otellog.String("stage", msg),
		otellog.String("error", err.Error()),
	)

	http.Error(w, msg+": "+err.Error(), http.StatusBadRequest)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getquery(r *http.Request, key, def string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return def
}

func atoi(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
