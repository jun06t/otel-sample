# Simple

OpenTelemetry の基本的なトレーシングを示す最小構成のサンプルです。

## アーキテクチャ

```
┌──────────────┐
│  API (:8000) │──→ httpbin.org/delay/1
└──────┬───────┘
       │ Jaeger HTTP
       ▼
┌──────────────┐
│   Jaeger     │
│ (:16686 UI)  │
└──────────────┘
```

単一の HTTP サーバーと Jaeger のみのシンプルな構成です。

## 特徴

- **最小構成**: サービス1つ + Jaeger のみ
- **手動スパン作成**: `tracer.Start()` で op1, op2, op3 の子スパンを明示的に作成
- **HTTP 自動計装**: `otelhttp.NewHandler`（受信）+ `otelhttp.NewTransport`（送信）
- **外部 HTTP 呼び出し**: httpbin.org への HTTP クライアント呼び出しもトレースに含まれる
- **AlwaysSample**: 全リクエストをトレース

## `/hello` のトレース構造

```
server (otelhttp)
├── op1 (100ms)
├── op2 (100ms)
├── HTTP GET httpbin.org/delay/1
└── op3 (100ms)
```

## 使い方

```bash
docker compose up --build
curl http://localhost:8000/hello
open http://localhost:16686
```
