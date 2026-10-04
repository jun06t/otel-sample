package main

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// requestInfo は、テレメトリーの次元にだけ使うリクエスト文脈。
//
// HTTP サーバーなら middleware で、User-Agent、アクセストークンのクレーム、
// geo ヘッダーなどから組み立てて context に入れる。
// 処理の入力(user ID)や分岐に使う値はここに入れず、引数で渡す。
type requestInfo struct {
	Country    string // ISO 3166-1 alpha-2
	Plan       string
	OSName     string
	OSVersion  string
	AppName    string
	AppVersion string
	NewProfile bool // feature flag「新しいプロフィール画面」の評価結果
}

// requestInfoKey は context のキー。パッケージ間の衝突を避けるため、外から見えない独自型にする。
type requestInfoKey struct{}

// withRequestInfo は info を持つ context を返す。
func withRequestInfo(ctx context.Context, info requestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey{}, info)
}

// requestInfoFrom は ctx に入っている requestInfo を返す。
func requestInfoFrom(ctx context.Context) (requestInfo, bool) {
	info, ok := ctx.Value(requestInfoKey{}).(requestInfo)
	return info, ok
}

// attributes は wide event の列として span に載せる属性を返す。
//
// 何に使うか分からなくても関係しそうな文脈は全部載せておく。障害調査のときに、
// 事前に用意していない組み合わせ(例: OS のバージョン × アプリのバージョン)で絞り込めるようにするため。
func (i requestInfo) attributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("user_agent.name", i.AppName),
		attribute.String("user_agent.version", i.AppVersion),
		attribute.String("user_agent.os.name", i.OSName),
		attribute.String("user_agent.os.version", i.OSVersion),
		attribute.String("app.user.country", i.Country),
		attribute.String("app.user.plan", i.Plan),
		attribute.Bool("app.feature.new_profile", i.NewProfile),
	}
}

// requestInfoProcessor は、context に入っている requestInfo を、このサービスのルート span に属性として付ける。
// 各関数は requestInfo を意識しなくてよい。
//
// OnStart は子 span を含むすべての span で 1 回ずつ呼ばれるが、属性を付けるのはルート span だけにしている。
// リクエスト文脈は 1 リクエストに 1 回あれば足り、全 span に付けるとその分だけデータ量が増えるため。
// その代わり、子 span(SELECT users など)をリクエスト文脈で絞り込むことはできない。
//
// ルート span は「親 span がいない、または親が別プロセスにいる」span とする。HTTP サーバーでは
// 上流から trace context が伝播されてくるので、親がいないかだけで判定すると入口の span にも付かなくなる。
//
// OnStart は sampler の判断の後に、記録される span に対してだけ呼ばれる。
// ここで付けた属性は head sampling の判断には使えない(tail sampling なら使える)。
type requestInfoProcessor struct{}

var _ sdktrace.SpanProcessor = (*requestInfoProcessor)(nil)

func newRequestInfoProcessor() *requestInfoProcessor {
	return &requestInfoProcessor{}
}

func (p *requestInfoProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if sc := s.Parent(); sc.IsValid() && !sc.IsRemote() {
		// 同じプロセスに親がいる子 span には付けない。
		return
	}
	if info, ok := requestInfoFrom(parent); ok {
		s.SetAttributes(info.attributes()...)
	}
}

func (p *requestInfoProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

func (p *requestInfoProcessor) Shutdown(context.Context) error { return nil }

func (p *requestInfoProcessor) ForceFlush(context.Context) error { return nil }
