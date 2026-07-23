# worker — 常駐ワーカーの OTel 計装（パターン②, Pub/Sub メッセージ駆動）

常駐ワーカー（Deployment, キュー/トピック購読）向けの計装サンプル。
**1 メッセージ = 1 トレース**。span を張るのは自分ではなく **Pub/Sub SDK**（`EnableOpenTelemetryTracing`）。

```
  publisher (デモは自分で publish)
       │  traceparent をメッセージ属性に inject（propagator）
       ▼
  ┌─────────┐   demo-topic publish (Producer, SDK自動)
  │ Pub/Sub │
  │ emulator│
  └────┬────┘
       │ Receive
       ▼
  worker: sub.Receive(ctx, cb)
    └─ span: demo-sub subscribe / process (Consumer, SDK自動)   ← ctx に載って渡ってくる
         └─ span: handle-message (自前, subscribe span の子)
                          │ OTLP gRPC (EXPORTER_ENDPOINT)
                          ▼
                      ┌────────┐
                      │ Jaeger │  http://localhost:16686
                      └────────┘
```

## 特徴 / 肝

1. **`otel.SetTracerProvider()` + `otel.SetTextMapPropagator()` をグローバルに設定する**（`telemetry/trace.go`）。SDK 自動計装はこのグローバル設定に乗る。propagator（W3C TraceContext）設定で発行元と購読側のトレースが繋がる。**設定を忘れると span が出ない / 別トレースに分断**される。
2. Pub/Sub client を `EnableOpenTelemetryTracing: true` で作る（`main.go`）。
3. `Receive` のコールバックに来る `ctx` には「そのメッセージの subscribe span」が入っている。ビジネス処理で `tracer.Start(ctx, ...)` すれば **subscribe span の子**として自然にぶら下がる（`handler/handler.go`）。
4. `SIGINT/SIGTERM` で `ctx` をキャンセル → `Receive` が抜ける → `defer` で `Shutdown()`（graceful shutdown）。

> [!important] 「1 Subscription = 1 root span」ではない
> 粒度は subscription ではなく**メッセージ単位**。1 subscription が 1000 件受ければ subscribe span も 1000 本できる。高スループットではサンプリング設計を検討する。

## ファイル構成

| ファイル | 役割 |
|---|---|
| `main.go` | signal handling, provider/propagator 設定, `Receive` ループ, graceful shutdown |
| `config.go` | envconfig で設定を読み込む（Pub/Sub 設定含む） |
| `telemetry/trace.go` | TracerProvider + propagator セットアップ。exporter を stdout / OTLP で切替 |
| `handler/handler.go` | `handle-message` span を張るビジネス処理 |
| `pubsub/bootstrap.go` | デモ用の topic/subscription 作成 + テストメッセージ publish（本番は不要） |

## 設定（環境変数 / envconfig）

| 変数 | デフォルト | 説明 |
|---|---|---|
| `SERVICE_NAME` | `example-worker` | Trace 上のサービス名 |
| `EXPORTER_ENDPOINT` | （空） | 空なら stdout、値があれば OTLP gRPC（例 `jaeger:4317`） |
| `PROJECT_ID` | `demo-project` | Pub/Sub プロジェクト ID |
| `TOPIC_ID` | `demo-topic` | トピック ID |
| `SUBSCRIPTION_ID` | `demo-sub` | サブスクリプション ID |
| `PUBSUB_EMULATOR_HOST` | （空） | 設定するとエミュレータに接続（ライブラリが自動参照） |
| `BOOTSTRAP` | `true` | 起動時に topic/sub 作成 + テストメッセージ publish（デモ用） |

## 実行

### Jaeger + Pub/Sub エミュレータ（docker compose）

```console
$ docker compose up --build
# publish → subscribe(SDK自動) → handle-message(自前) の span 列が Jaeger に連結表示される
```

ブラウザで Jaeger UI: http://localhost:16686 （Service: `example-worker`）

### ローカルで直接（エミュレータを自前起動）

```console
# 初回のみ
$ gcloud components install pubsub-emulator

# ターミナルA: エミュレータ起動
$ gcloud beta emulators pubsub start --project=demo-project --host-port=localhost:8085

# ターミナルB: ワーカー起動（EXPORTER_ENDPOINT 未設定なら stdout に span 出力）
$ export PUBSUB_EMULATOR_HOST=localhost:8085
$ go mod tidy
$ go run .
# Ctrl-C で graceful shutdown し、残りの span も flush される
```

## 落とし穴

- propagator 未設定 → 発行元と購読側が別トレースに分断。
- provider をグローバル設定し忘れ → SDK は no-op tracer を使い span が一切出ない。
- 「1 Subscription = 1 span」だと思い込む → 実際はメッセージ単位。高スループット時はサンプリング設計を。
