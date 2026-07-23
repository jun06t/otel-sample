# batch — Run-to-completion バッチの OTel 計装（パターン①）

短命バッチ（K8s Job / CronJob, cron 起動）向けの計装サンプル。
**1 実行 = 1 トレース**。アプリが自分で root span を 1 本張り、**終了前に必ず flush** する。

```
┌─────────────────────────── batch process (起動→処理→終了) ───────────────────────────┐
│                                                                                       │
│   run()                                                                               │
│    └─ span: batch.Run  (root, batch.item_count=N)                                     │
│         ├─ span: fetch                                                                │
│         └─ span: process-item ×N                                                      │
│                                                                                       │
│   defer shutdown()  ← ★ここで BatchSpanProcessor を flush（忘れると span 消失）        │
└──────────────────────────────────────┬────────────────────────────────────────────────┘
                                        │ OTLP gRPC (EXPORTER_ENDPOINT)
                                        ▼
                                    ┌────────┐
                                    │ Jaeger │  http://localhost:16686
                                    └────────┘
```

## 特徴 / 肝

1. **`os.Exit` は `defer` を実行しない。** 終了コードは `run()` に集約し、`defer shutdown()` を必ず通す（`main.go`）。
2. `BatchSpanProcessor` は溜めてから送るが、**`Shutdown()` が内部で `ForceFlush()` してくれる**ので終了前に呼べば取りこぼさない（`telemetry/trace.go`）。
3. root span `batch.Run` にジョブ全体の属性（件数など）を載せ、失敗時は `RecordError` + `SetStatus(codes.Error, ...)`（`job/job.go`）。

## ファイル構成

| ファイル | 役割 |
|---|---|
| `main.go` | `main()` は `os.Exit(run())` のみ。終了コードを `run()` に集約し flush を保証 |
| `config.go` | envconfig で設定を読み込む（`SERVICE_NAME`, `EXPORTER_ENDPOINT`） |
| `telemetry/trace.go` | TracerProvider セットアップ。exporter を stdout / OTLP で切替、shutdown ヘルパを返す |
| `job/job.go` | ビジネスロジック。root span `batch.Run` と子 span `fetch` / `process-item` |

## 設定（環境変数）

| 変数 | デフォルト | 説明 |
|---|---|---|
| `SERVICE_NAME` | `example-batch` | Trace 上のサービス名 |
| `EXPORTER_ENDPOINT` | （空） | **空なら stdout に span 出力**（`go run .` で即確認）。値があれば OTLP gRPC でその宛先（例 `jaeger:4317`）へ送信 |

## 実行

### そのまま（stdout に span を出力）

```console
$ go mod tidy
$ go run .
# batch.Run を root に、fetch / process-item がぶら下がった span が stdout に出力される
```

### Jaeger で可視化

```console
$ docker compose up --build
# batch が 1 回実行され、トレースが Jaeger に送られて終了する
```

ブラウザで Jaeger UI を開く: http://localhost:16686 （Service: `example-batch`）

## 落とし穴

- `os.Exit` / `log.Fatal` / panic で `defer` が飛ぶ → `Shutdown()` されず span 消失。終了コードは `run()` に集約する。
- 短命バッチで「span が出ない」→ 送信前にプロセスが死んでいる。`Shutdown()`（= 内部 `ForceFlush`）を終了前に必ず。
- stdout exporter はデモ専用。本番は OTLP / Cloud Trace 等へ。
