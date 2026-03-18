# Bridge (OpenCensus → OpenTelemetry)

OpenCensus から OpenTelemetry への移行ブリッジのサンプルです。
`go.opentelemetry.io/otel/bridge/opencensus` を使い、既存の OpenCensus 計装コードを OpenTelemetry に統合します。

## アーキテクチャ

```
┌──────────────┐         ┌───────────────────┐
│  API (:8000) │────────→│ BigTable Emulator  │
│  OTel + OC   │         │ (:8086)            │
└──────┬───────┘         └───────────────────┘
       │ Jaeger HTTP
       ▼
┌──────────────┐
│   Jaeger     │
│ (:16686 UI)  │
└──────────────┘
```

## 特徴

- **OpenCensus ブリッジ**: `bridge/opencensus` パッケージで OpenCensus の DefaultTracer を OpenTelemetry にルーティング
- **BigTable 統合**: Google Cloud BigTable エミュレータを使ったデータ読み書き（OpenCensus で計装済み）
- **統合トレース**: OpenCensus 由来のスパンと OpenTelemetry のスパンが同一トレースに統合される
- **段階的移行**: 既存の OpenCensus コードを書き換えずに OpenTelemetry へ移行する方法を示す

## ユースケース

BigTable クライアントライブラリは内部で OpenCensus を使用しています。
ブリッジを使うことで、アプリ側は OpenTelemetry に統一しつつ、ライブラリの OpenCensus トレースも Jaeger に統合できます。

## 使い方

```bash
docker compose up --build
curl http://localhost:8000/hello
open http://localhost:16686
```
