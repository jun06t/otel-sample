# SigNoz Wide Event

zap のログと分散トレースを **1 つのストア(SigNoz / ClickHouse)に co-locate** し、
1 画面で属性とログを相関して見る「Observability 2.0 / wide event」型のサンプルです。

## Observability 1.0 と 2.0

| | Obs 1.0（三本柱） | Obs 2.0（wide event / このサンプル） |
|---|---|---|
| ログの居場所 | traces と logs が別ストア（Tempo/Jaeger と Loki など） | traces / logs が同一ストア（SigNoz=ClickHouse）に同居 |
| 相関 | `trace_id` を跨ぐハイパーリンクで別 UI へ行き来 | 同一レコードのフィールド。相関という作業が無い |
| リクエストの文脈 | ログ行に構造化フィールドで散らす | **span 属性に集約（= wide event）** |

このサンプルは「リクエストの文脈は span 属性へ」「timestamp+severity+message を持つ“ログ”は
trace context で span に紐づく LogRecord へ」という役割分担を実装しています。

## アーキテクチャ

```
┌────────────────────────────┐
│  app (:8000)               │
│   trace: OTLP gRPC ────────┼──┐
│   log(zap→otelzap): OTLP ──┼──┤ 同一 endpoint
└────────────────────────────┘  │
                                 ▼
                    ┌─────────────────────────┐
                    │ SigNoz 内蔵 OTel Collector │
                    │        (:4317)           │
                    └────────────┬────────────┘
                                 ▼
                    ┌─────────────────────────┐
                    │  SigNoz (ClickHouse)     │  spans / logs 同居
                    │  UI (:8080/:3301)        │  1 画面で相関
                    └─────────────────────────┘
```

app は traces と logs を **同じ resource・同じ OTLP エンドポイント**で送るため、SigNoz 上で
service 次元でも `trace_id` でも突き合わせられます。

## 計装のポイント

### 正常系 = wide event（`main.go` の `order`）

途中経過のログ行は出さず、作業単位の文脈をすべて span 属性に載せます。

```go
span.SetAttributes(
    attribute.String("user.id", userID),
    attribute.Int("order.items", items),
    attribute.Int("order.amount", amount),
)
// ... 成功したら
span.SetAttributes(attribute.String("order.status", "charged"))
```

### 異常系 = 役割分担（`main.go` の `fail`）

- **失敗した“事実”** → `span.SetStatus(codes.Error, …)`（trace が赤くなり `status=error` で検索できる）= wide event
- **エラーの“中身”(message)** → `logger.Error(...)`（普通のログ。`event.name` 無しの LogRecord）
- **同じ内容を log-based event としても** → `emitEvent(...)`（`event.name` 付きの LogRecord）

```go
span.SetStatus(codes.Error, msg)
span.SetAttributes(attribute.String("order.status", "failed"))

// 普通のログ(event.name 無し)
Ctx(ctx, h.logger).Error(msg, zap.Error(err))

// log-based event(event.name 付き) — span.AddEvent の後継形
emitEvent(ctx, h.events, "order.failed", otellog.SeverityError,
    otellog.String("stage", msg),
    otellog.String("error", err.Error()),
)
```

> あえて `span.RecordError` は使っていません。OpenTelemetry は 2026 に span event
> （`RecordException` 含む）を deprecated とし、イベントは **log-based event** で表現する方針に
> 切り替えました。本サンプルはその方針に沿ってエラーを logs 側に寄せています。

### 普通のログ vs log-based event（`events.go`）

`logger.Error(...)` と `emitEvent(...)` は **同じ logs 信号（OTLP logs）を流れる LogRecord** で、
違いは **`event.name` の有無だけ**です。

- **普通のログ**: severity + message 中心。`event.name` なし。zap 経由（otelzap）。
- **log-based event**: `event.name`（例 `order.failed`）を持つ。「時点の出来事」を表す。
  zap では `event.name` を表現できないため、`emitEvent` だけ OTel Logs API を直接使用。

どちらも ctx の span から `trace_id`/`span_id` が自動で紐づき、SigNoz 上でその span と同じ画面に並びます。

> なお `event.name` を持つ LogRecord が「log-based event」で、これは wide event（= span 属性）とは
> 別物です。wide event は traces 信号（span を太らせる）、log-based event は logs 信号（時点の出来事）。

### zap → OTLP（`logger.go`）

`zapcore.NewTee` で 2 系統に書きます。

- **stdout(JSON)**: ローカル確認・span 外（起動など）のログ用
- **otelzap**: `LogRecord` として OTLP で SigNoz に送信

otelzap は公式ブリッジ `go.opentelemetry.io/contrib/bridges/otelzap` です
（span.AddEvent に書く同名の uptrace 版とは別物）。このブリッジは
**フィールドで渡した `context.Context` を検出して LogRecord の trace_id/span_id を埋める**仕様です。
その raw context を stdout に出すと汚いので、`dropFieldCore` で stdout からだけ除去し、
人間が追えるよう `trace_id`/`span_id` は文字列でも載せています。

## 使い方

### 1. SigNoz を起動（別途）

SigNoz は ClickHouse を含む複数コンテナ構成です。現在の公式手順は Foundry(`foundryctl`) で、
**git clone は不要**です（従来の `git clone → deploy/docker` は v0.130.0 で deprecated）。

```bash
# 1. foundryctl を入れる
curl -fsSL https://signoz.io/foundry.sh | bash

# 2. casting.yaml を作成
cat > casting.yaml <<'YAML'
apiVersion: v1alpha1
kind: Installation
metadata:
  name: signoz
spec:
  deployment:
    flavor: compose
    mode: docker
YAML

# 3. 検証→生成→起動を一括
foundryctl cast -f casting.yaml
```

- UI: <http://localhost:8080>
- OTLP: gRPC `:4317` / HTTP `:4318`

自前で起動したくない場合は **SigNoz Cloud**（マネージド）に OTLP 送信する選択肢もあります。
最新手順は公式ドキュメント <https://signoz.io/docs/install/docker/> を参照。

### 2. app を起動

**ローカル実行（最短）**。host に公開された OTLP `:4317` に直接送る:

```bash
EXPORTER_ENDPOINT=localhost:4317 go run .
```

**Docker で起動**する場合は、app を **SigNoz のネットワークに相乗り**させ、ingester の
コンテナ名へ直接送る（`docker-compose.yml` は既定でそう設定済み）:

```bash
docker compose up --build
```

- 送信先は `signoz-ingester:4317`（`host.docker.internal` は IPv6 解決で届かないことがあるため使わない）。
- `docker-compose.yml` は外部ネットワーク `signoz-network` に join する。foundry が作る
  ネットワーク名が異なる場合は `docker network ls` で確認し、compose の `networks.signoz.name` を合わせる。
   ingester の DNS 名も併せて確認する場合は `docker inspect <ingester> --format '{{json .NetworkSettings.Networks}}'`。

### 3. リクエストを送る

```bash
# 正常（wide event: span 属性が付く）
curl 'localhost:8000/order?user=alice&items=2&amount=3000'

# validation error（items <= 0）
curl 'localhost:8000/order?user=bob&items=0&amount=3000'

# charge error（amount > 10000）
curl 'localhost:8000/order?user=carol&items=1&amount=99999'
```

### 4. SigNoz で確認

- **Traces**: `user.id` や `order.amount` など高カーディナリティ属性で絞り込み（slice & dice）。
- **エラー trace**: `status=error` で失敗リクエストを抽出。
- **1 画面相関**: エラー span を開くと、その span に紐づくエラーログ（message）が
  同じ画面で確認できる（Obs 1.0 のような別 UI への行き来が不要）。

## 注意点・トレードオフ

- **バックエンド依存**: この 1 画面相関は SigNoz(ClickHouse) が spans/logs を co-locate し
  高カーディナリティクエリを許すから成立します。Jaeger では span event の表示はできても
  この体験（高カーディナリティ slice & dice）は得られません。
- **サンプリング**: 本サンプルは `fraction=1.0`（全件）。tail sampling などで trace を落とすと、
  wide event に載せた文脈も一緒に失われます。エラーは logs 側にも残しておくと安全です。
- **span 外のログ**: 起動ログやバッチなど active span が無いログは wide event に載せられず、
  通常の logger（stdout / OTLP）で出します。本サンプルの起動ログがその例です。
- **バージョン整合**: otel 本体 v1.x に対し、Logs SDK / 公式 otelzap は別モジュール（v0.x）です。
  `go.mod` は `go mod tidy` で解決したバージョンに揃えています。
