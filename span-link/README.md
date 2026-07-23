# span-link — messaging のバッチ処理を Span Link で連結する

参照: [OpenTelemetry Semantic Conventions — Messaging Spans](https://opentelemetry.io/docs/specs/semconv/messaging/messaging-spans/)

複数メッセージを **1 回の操作でまとめて取り出す**（バッチ receive）場合、その **receive span** は
各メッセージの発行元トレースへ **parent-child ではなく span link で連結**する。
span は親を 1 つしか持てないため、複数の発行元トレースを束ねるバッチ処理では **link が唯一の相関手段**になる。
これにより **subscribe(receive) 側と publish(send) 側が link で繋がる**。

```
  publisher: N 件を「それぞれ別トレース」で publish（PRODUCER span）
       trace-A         trace-B         trace-C
   [send demo-topic] [send demo-topic] [send demo-topic]   ← traceparent を各メッセージ属性へ inject
        │                │                │
        └───────┐        │        ┌───────┘   span link (1:N)
                ▼        ▼        ▼
  consumer (trace-Z):
    [receive demo-sub]  (CLIENT, messaging.batch.message_count=N, Links=[A,B,C])  ← ★ここが link を持つ
        ├─ [process demo-sub]  (CONSUMER, message.id=...)   ← receive の子（各メッセージ処理）
        ├─ [process demo-sub]
        └─ [process demo-sub]
                          │ OTLP gRPC
                          ▼
                      ┌────────┐
                      │ Jaeger │  http://localhost:16686
                      └────────┘
```

## ポイント（messaging semconv 準拠）

- **Producer span**: `SpanKind = PRODUCER`。`send <topic>` 名。publish 時に propagator で `traceparent` をメッセージ属性へ inject（`publisher/publisher.go`）。
- **Batch receive span**: `SpanKind = CLIENT`。`receive <subscription>` 名。各メッセージ属性から発行元 SpanContext を Extract し、`trace.WithLinks(...)` で **N 件の link を receive span に**張る（`consumer/consumer.go`）。これが subscribe 側と publish 側を繋ぐ。
- **Process span**: `SpanKind = CONSUMER`。`process <subscription>` 名。各メッセージの処理で、**receive span の子**として張る。
- 属性は**公式 semconv 定数**（`go.opentelemetry.io/otel/semconv/v1.40.0` の `MessagingSystemGCPPubSub` / `MessagingOperationTypeReceive` 等）を使用。receive に `messaging.batch.message_count`、各 link に `messaging.message.id`。
- receive/process の consumer トレースは発行元とは**別トレース**になり、link で相関する（1 span = 1 parent の制約への回答）。

## ファイル構成

| ファイル | 役割 |
|---|---|
| `main.go` | publish → batch receive → consume の一連を実行 |
| `config.go` | envconfig で設定（Pub/Sub 設定・`BATCH_SIZE` を含む） |
| `telemetry/trace.go` | TracerProvider + propagator。exporter を stdout / OTLP 切替 |
| `publisher/publisher.go` | 各メッセージを別トレースの PRODUCER span で publish + traceparent inject |
| `consumer/consumer.go` | 受信メッセージ群から link を組み立て、receive span（+ 子 process span）を生成 |

## 設定（環境変数 / envconfig）

| 変数 | デフォルト | 説明 |
|---|---|---|
| `SERVICE_NAME` | `example-span-link` | Trace 上のサービス名 |
| `EXPORTER_ENDPOINT` | （空） | 空なら stdout、値があれば OTLP gRPC（例 `jaeger:4317`） |
| `PROJECT_ID` | `demo-project` | Pub/Sub プロジェクト ID |
| `TOPIC_ID` | `demo-topic` | トピック ID |
| `SUBSCRIPTION_ID` | `demo-sub` | サブスクリプション ID |
| `PUBSUB_EMULATOR_HOST` | （空） | 設定するとエミュレータに接続 |
| `BATCH_SIZE` | `3` | 1 回の receive でまとめて取り出す件数（= span link 数） |

## 実行

### Jaeger + Pub/Sub エミュレータ（docker compose）

```console
$ docker compose up --build
# publish(N トレース) → receive(1 span, N link) + process(N 子 span) が Jaeger に出力される
```

Jaeger UI: http://localhost:16686 。`receive demo-sub` span を開くと **References に N 件の "FOLLOWS_FROM"（span link）** が表示され、各発行元トレースへ辿れる。process span は receive span の子として同じトレースにぶら下がる。

### ローカルで直接（stdout に span を出力）

```console
# エミュレータ起動
$ gcloud beta emulators pubsub start --project=demo-project --host-port=localhost:8085

# 別ターミナル
$ export PUBSUB_EMULATOR_HOST=localhost:8085
$ go mod tidy
$ go run .
# receive span の "Links" に N 件、それぞれ別トレースの producer span を指しているのが確認できる
```
