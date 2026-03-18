# Tail-Based Sampling with Honeycomb Refinery

[Honeycomb Refinery](https://github.com/honeycombio/refinery) を使った tail-based sampling のサンプルです。
Refinery は Honeycomb が開発した tail-based sampling プロキシで、**動的サンプリング（EMADynamicSampler）** が最大の特徴です。

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
       │    Refinery      │
       │  (:4317 gRPC)    │
       │  (:8080 HTTP)    │
       └────────┬────────┘
                ▼
       ┌─────────────────┐
       │    Honeycomb     │
       │   (Cloud API)    │
       └─────────────────┘

       ┌─────────────────┐
       │       k6         │──→ Gateway へ負荷
       └─────────────────┘
```

## 特徴

- **動的サンプリング**: EMADynamicSampler がトラフィックパターンに応じてサンプリングレートを自動調整
- **レアイベントの保持**: 頻出パターンを多く間引き、レアなパターン（エラー等）を優先的に保持
- **ルールベース + 動的の組み合わせ**: エラーや遅延は確実に保持し、残りは動的にサンプリング
- **アプリ側は AlwaysSample**: サンプリング判断は全て Refinery 側で実施

## サンプリングルール

`refinery-rules.yaml` で定義:

| ルール | 条件 | 動作 |
|--------|------|------|
| `keep errors` | `otel.status_code = ERROR` | SampleRate: 1（全て保持） |
| `keep slow requests` | `duration_ms >= 1000` | SampleRate: 1（全て保持） |
| `dynamic sample rest` | 上記に該当しない全て | EMADynamicSampler (GoalSampleRate: 5) |

### EMADynamicSampler とは

Exponential Moving Average（指数移動平均）を使った動的サンプラーです。

- `FieldList` で指定したフィールドの組み合わせごとにサンプリングレートを計算
- **頻出パターン**（例: `GET /hello 200`）は多く間引く
- **レアパターン**（例: `POST /api 500`）はほぼ全て保持
- `GoalSampleRate: 5` は「平均で5分の1にする」目標

OTel Collector の `probabilistic` ポリシー（固定10%等）と異なり、トラフィックの多様性を維持しながらデータ量を削減できます。

## エラー/遅延シミュレーション

`backend-grpc` はリクエストの一部でランダムにエラーや遅延を発生させます:

- **10%** の確率で gRPC Internal エラーを返す
- **10%** の確率で 1〜3 秒の遅延を発生させる
- **80%** は通常レスポンス（約200ms）

## 前提条件

- Honeycomb のアカウントとAPI キーが必要（[フリープラン](https://www.honeycomb.io/pricing)で取得可能）

## 使い方

```bash
# 環境変数の設定
cp .env.example .env
# .env を編集して HONEYCOMB_API_KEY を設定

# 起動（k6 による負荷テストも自動開始）
docker compose up --build

# Honeycomb UI でトレースを確認
# https://ui.honeycomb.io
# - meta.refinery.reason: サンプリング理由
# - meta.refinery.send_reason: 送信理由の詳細

# 手動でリクエスト送信
curl http://localhost:8000/hello

# k6 の負荷パラメータを変更して実行
docker compose run k6 run /scripts/loadtest.js --vus 20 --duration 2m
```

## Refinery vs OTel Collector tail sampling

| 項目 | Refinery (本サンプル) | OTel Collector (tail-sampling/) |
|------|----------------------|-------------------------------|
| 動的サンプリング | EMADynamic, EMAThroughput 等 | なし（静的ルールのみ） |
| クラスタリング | 組み込み（peer-to-peer） | 外部の2層構成が必要 |
| バックエンド | Honeycomb のみ | 任意（Jaeger, Zipkin 等） |
| API キー | 必要 | 不要（ローカル完結可） |
| サンプリング理由の記録 | `meta.refinery.reason` で自動記録 | なし |
