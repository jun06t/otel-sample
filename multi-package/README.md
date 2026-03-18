# Multi-Package

複数の Go パッケージにまたがるトレーシングのサンプルです。
パッケージごとに独立した Tracer を持ち、スパンが正しく親子関係を維持することを示します。

## アーキテクチャ

```
┌──────────────────────────────────┐
│  API (:8000)                     │
│  ┌─────┐  ┌─────────┐  ┌──────┐ │
│  │ cmd │→│ service  │→│ util │ │──→ httpbin.org
│  └─────┘  └─────────┘  └──────┘ │
└──────────────┬───────────────────┘
               │ Jaeger HTTP
               ▼
       ┌─────────────────┐
       │     Jaeger       │
       │  (:16686 UI)     │
       └─────────────────┘
```

## 特徴

- **パッケージ別 Tracer**: 各パッケージが `otel.Tracer("package/path")` で独自の Tracer を取得
- **スパンの親子関係**: `context.Context` を渡すことで、パッケージを跨いでもトレースが繋がる
- **3層構成**: `cmd`（エントリポイント）→ `service`（ビジネスロジック）→ `util`（ユーティリティ）
- **AlwaysSample**: 全リクエストをトレース

## パッケージ構成

```
multi-package/
├── cmd/main.go          # エントリポイント、HTTP サーバー起動
├── service/service.go   # HTTP ハンドラー、Operation1/2、HTTP クライアント
├── util/util.go         # ユーティリティ関数 (Operation)
└── telemetry/trace.go   # TracerProvider 初期化
```

## `/hello` のトレース構造

```
server (otelhttp)                    ← cmd パッケージ
├── service: op1 (100ms)             ← service パッケージ
├── service: HTTP GET httpbin.org    ← service パッケージ
└── util: op (100ms)                 ← util パッケージ
```

Jaeger 上でスパンごとにどのパッケージ（Tracer）から発行されたか確認できます。

## 使い方

```bash
docker compose up --build
curl http://localhost:8000/hello
open http://localhost:16686
```
