# span-link — messaging の publish と process を Span Link で連結する

参照: [OpenTelemetry Semantic Conventions — Messaging Spans](https://opentelemetry.io/docs/specs/semconv/messaging/messaging-spans/)

発行元（publish）と消費側（process）を **parent-child ではなく span link で連結**するサンプル。
消費側の `process` span を**発行元とは別トレース（新規 root）**として開始し、その発行元の
publish span へ **span link を 1 本**張って **1:1 で対応**させる。

発行元と消費側を別トレースに保ったまま相関できるのが span link の役割で、
常駐ワーカーのように**メッセージを 1 件ずつ捌く**実運用に近い形になる。

```
  publisher: N 件を「それぞれ別トレース」で publish（PRODUCER span）
   trace-A            trace-B            trace-C
   [send demo-topic]  [send demo-topic]  [send demo-topic]
        │                  │                  │   ← traceparent を各メッセージ属性へ inject
        │ span link (1:1)  │ span link (1:1)  │ span link (1:1)
        ▼                  ▼                  ▼
   trace-A'           trace-B'           trace-C'
   [process demo-sub] [process demo-sub] [process demo-sub]   ← 各 process は別トレース(新規 root)
        │                  │                  │ OTLP gRPC
        └──────────────────┴──────────────────┘
                           ▼
                       ┌────────┐
                       │ Jaeger │  http://localhost:16686
                       └────────┘
```

## ポイント（messaging semconv 準拠）

- **Producer span**: `SpanKind = PRODUCER`。`send <topic>` 名。publish 時に propagator で `traceparent` をメッセージ属性へ inject（`publisher/publisher.go`）。
- **Process span**: `SpanKind = CONSUMER`。`process <subscription>` 名。`context.Background()` から開始して**発行元とは別トレース（新規 root）**にし、メッセージ属性から Extract した発行元 SpanContext へ `trace.WithLinks(...)` で **link を 1 本**張る（`consumer/consumer.go`）。
- 属性は**公式 semconv 定数**（`go.opentelemetry.io/otel/semconv/v1.40.0` の `MessagingSystemGCPPubSub` / `MessagingOperationType{Send,Process}` 等）を使用。link には `messaging.message.id` を付与。
- リンク先の ID（trace_id / span_id）は **メッセージ属性の `traceparent`** で運ばれる（`00-<trace_id>-<span_id>-<flags>`）。Pub/Sub 固有ではなく、ヘッダ/属性を運べるトランスポート共通の仕組み。

> [!note] parent-child との使い分け
> **publish→process を 1 トレースで線で繋げて俯瞰したい**なら、それは span link ではなく **parent-child（文脈伝播）**で、[`../worker`](../worker) サンプルが該当（SDK 自動計装で publish/subscribe/handle が同一トレース）。
> span link は発行元と消費側を**別トレースに保ったまま**相関する用途で、Jaeger では**タイムラインで線は繋がらず**、span の References（`FOLLOWS_FROM`）からクリックで辿る形になる。

## ファイル構成

| ファイル | 役割 |
|---|---|
| `main.go` | publish → receive → consume の一連を実行 |
| `config.go` | envconfig で設定（Pub/Sub 設定・`BATCH_SIZE` を含む） |
| `telemetry/trace.go` | TracerProvider + propagator。exporter を stdout / OTLP 切替 |
| `publisher/publisher.go` | 各メッセージを別トレースの PRODUCER span で publish + traceparent inject |
| `consumer/consumer.go` | 各メッセージを、発行元へ link 1 本を張った process span（別トレース）で処理 |

## 設定（環境変数 / envconfig）

| 変数 | デフォルト | 説明 |
|---|---|---|
| `SERVICE_NAME` | `example-span-link` | Trace 上のサービス名 |
| `EXPORTER_ENDPOINT` | （空） | 空なら stdout、値があれば OTLP gRPC（例 `jaeger:4317`） |
| `PROJECT_ID` | `demo-project` | Pub/Sub プロジェクト ID |
| `TOPIC_ID` | `demo-topic` | トピック ID |
| `SUBSCRIPTION_ID` | `demo-sub` | サブスクリプション ID |
| `PUBSUB_EMULATOR_HOST` | （空） | 設定するとエミュレータに接続 |
| `BATCH_SIZE` | `3` | まとめて受信して処理するメッセージ数（＝ publish 件数） |

## 実行

### Jaeger + Pub/Sub エミュレータ（docker compose）

```console
$ docker compose up --build
# send(N トレース) と process(N トレース) が出力され、各 process が自分の publish へ link 1 本で紐付く
```

Jaeger UI: http://localhost:16686 。`process demo-sub` span を開くと **References に 1 件の "FOLLOWS_FROM"（span link）** があり、対応する発行元トレース（`send demo-topic`）へ辿れる。

### ローカルで直接（stdout に span を出力）

```console
# エミュレータ起動
$ gcloud beta emulators pubsub start --project=demo-project --host-port=localhost:8085

# 別ターミナル
$ export PUBSUB_EMULATOR_HOST=localhost:8085
$ go mod tidy
$ go run .
# 各 process span の "Links" に 1 件、対応する producer span を指しているのが確認できる
```
