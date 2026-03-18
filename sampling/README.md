# Head-Based Sampling

OpenTelemetry SDK の `TraceIDRatioBased` サンプラーを使った head-based sampling のサンプルです。

## アーキテクチャ

```
┌──────────┐     gRPC     ┌──────────────┐
│ Gateway  │─────────────→│ Backend-gRPC │
│ (:8000)  │              │ (:8080)      │
└────┬─────┘              └──────┬───────┘
     │ Jaeger HTTP                │ Jaeger HTTP
     └──────────┬────────────────┘
                ▼
       ┌─────────────────┐
       │     Jaeger       │
       │  (:16686 UI)     │
       └─────────────────┘
```

アプリから Jaeger へ直接エクスポートします。Collector やプロキシは不要です。

## 特徴

- **シンプル構成**: アプリ + Jaeger のみ。Collector 不要
- **SDK 内でサンプリング判断**: トレース開始時に確率的にサンプル/ドロップを決定
- **ParentBased**: 親スパンのサンプリング判断を子に伝搬（分散トレーシングの一貫性を保証）

## サンプリング設定

`ParentBased(TraceIDRatioBased(fraction))` を使用:

| サービス | SAMPLING_RATIO | 意味 |
|---------|---------------|------|
| Gateway | 0.1 | 10% のトレースをサンプリング |
| Backend-gRPC | 0.5 | 親がない場合 50%（通常は Gateway の判断を継承） |

`ParentBased` により、Gateway で「サンプルする」と決まったトレースは Backend-gRPC でも必ずサンプルされます。

## 使い方

```bash
# 起動
docker compose up --build

# リクエスト送信
curl http://localhost:8000/hello

# Jaeger UI でトレースを確認
open http://localhost:16686
```

## Head-Based vs Tail-Based Sampling

| 項目 | Head-Based (本サンプル) | Tail-Based (tail-sampling/) |
|------|----------------------|---------------------------|
| 判断タイミング | トレース開始時 | トレース完了後 |
| 判断場所 | アプリ側 SDK | 外部（Collector / Refinery） |
| エラートレースの保持 | 保証なし（確率的に落ちる） | 全て保持可能 |
| 遅延トレースの保持 | 保証なし | 全て保持可能 |
| メモリ使用量 | 少ない | バッファが必要 |
| 構成の複雑さ | シンプル | Collector / Refinery が必要 |
| ネットワーク帯域 | サンプルされたスパンのみ送信 | 全スパンを Collector に送信 |
| 適したユースケース | 高トラフィック環境でコスト削減 | エラーや異常を確実に捕捉したい場合 |
