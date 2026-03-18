# Microservice

HTTP + gRPC のマイクロサービス間で分散トレーシングを行うサンプルです。

## アーキテクチャ

```
┌──────────┐     gRPC     ┌──────────────┐
│ Gateway  │─────────────→│ Backend-gRPC │
│ (:8000)  │              │ (:8080)      │
│          │──→ httpbin.org               │
└────┬─────┘              └──────┬───────┘
     │ Jaeger HTTP                │ Jaeger HTTP
     └──────────┬────────────────┘
                ▼
       ┌─────────────────┐
       │     Jaeger       │
       │  (:16686 UI)     │
       └─────────────────┘
```

アプリから Jaeger へ直接エクスポートします。Collector は使いません。

## 特徴

- **gRPC 分散トレーシング**: `otelgrpc` のクライアント/サーバーインターセプターで自動計装
- **HTTP 自動計装**: `otelhttp` でHTTP受信・送信を自動トレース
- **コンテキスト伝搬**: gRPC メタデータを通じて Trace Context が自動伝搬
- **外部 HTTP 呼び出し**: httpbin.org への呼び出しもトレースに含まれる
- **AlwaysSample**: 全リクエストをトレース

## `/hello` のトレース構造

```
gateway: server (otelhttp)
├── gateway: grpc.helloworld.Greeter/SayHello (client)
│   └── backend: grpc.helloworld.Greeter/SayHello (server)
│       └── backend: operation (200ms)
├── gateway: op1 (100ms)
└── gateway: HTTP GET httpbin.org/delay/1
```

## collector/ との違い

| 項目 | microservice/ (本サンプル) | collector/ |
|------|--------------------------|-----------|
| エクスポート先 | Jaeger へ直接 | OTel Collector 経由 |
| プロトコル | Jaeger HTTP | OTLP gRPC |
| バックエンド | Jaeger のみ | Jaeger + Zipkin |

## 使い方

```bash
docker compose up --build
curl http://localhost:8000/hello
open http://localhost:16686
```
