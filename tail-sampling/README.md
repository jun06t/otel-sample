# Tail-Based Sampling with OTel Collector

OpenTelemetry Collector の `tailsampling` processor を使った tail-based sampling のサンプルです。

## アーキテクチャ

```
┌──────────┐     gRPC     ┌──────────────┐
│ Gateway  │─────────────→│ Backend-gRPC │
│ (:8000)  │              │ (:8080)      │
└────┬─────┘              └──────┬───────┘
     │ OTLP                      │ OTLP
     └──────────┬────────────────┘
                ▼
       ┌─────────────────┐
       │  OTel Collector  │
       │  tail_sampling   │
       │  processor       │
       └────────┬────────┘
                ▼
       ┌─────────────────┐
       │     Jaeger       │
       │  (:16686 UI)     │
       └─────────────────┘

       ┌─────────────────┐
       │       k6         │──→ Gateway へ負荷
       └─────────────────┘
```

## 特徴

- **ベンダー非依存**: OTel Collector + Jaeger でローカル完結。API キー不要
- **静的ルールベース**: 設定ファイルで定義した固定ルールでサンプリング判断
- **アプリ側は AlwaysSample**: サンプリング判断は全て Collector 側で実施

## サンプリングポリシー

| ポリシー | 種別 | 内容 |
|---------|------|------|
| `errors-policy` | `status_code` | ERROR ステータスのトレースを全て保持 |
| `latency-policy` | `latency` | 500ms 以上のトレースを全て保持 |
| `probabilistic-policy` | `probabilistic` | 残りのトレースを 10% サンプリング |

## エラー/遅延シミュレーション

`backend-grpc` はリクエストの一部でランダムにエラーや遅延を発生させます:

- **10%** の確率で gRPC Internal エラーを返す
- **10%** の確率で 1〜3 秒の遅延を発生させる
- **80%** は通常レスポンス（約200ms）

これにより、tail-based sampling の各ポリシーの効果を確認できます。

## 使い方

```bash
# 起動（k6 による負荷テストも自動開始）
docker compose up --build

# Jaeger UI でトレースを確認
open http://localhost:16686

# 手動でリクエスト送信
curl http://localhost:8000/hello

# k6 の負荷パラメータを変更して実行
docker compose run k6 run /scripts/loadtest.js --vus 20 --duration 2m
```

## OTel Collector との違い（Head-Based Sampling）

既存の `sampling/` ディレクトリでは head-based sampling（`TraceIDRatioBased`）を使用しています。

| 項目 | Head-Based (sampling/) | Tail-Based (本サンプル) |
|------|----------------------|----------------------|
| 判断タイミング | トレース開始時 | トレース完了後 |
| 判断場所 | アプリ側 SDK | OTel Collector |
| エラートレースの保持 | 保証なし（確率的に落ちる） | 全て保持可能 |
| 遅延トレースの保持 | 保証なし | 全て保持可能 |
| メモリ使用量 | 少ない | Collector でトレースをバッファするため多い |
| 設定の柔軟性 | サンプリング率のみ | status_code, latency, attribute 等で条件指定可能 |
