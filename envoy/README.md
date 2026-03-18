# Envoy Sidecar

Envoy プロキシをフロントプロキシとして配置し、OpenTelemetry の分散トレーシングに参加させるサンプルです。

## アーキテクチャ

```
                        ┌──────────┐     gRPC     ┌──────────────┐
Client ──→ Envoy ──────→│ Gateway  │─────────────→│ Backend-gRPC │
          (:10000)       │ (:8000)  │              │ (:8080)      │
                        └────┬─────┘              └──────┬───────┘
Envoy ─┐                     │ Jaeger HTTP                │ Jaeger HTTP
 OTLP  │                     └──────────┬────────────────┘
 gRPC  │                                ▼
       └──────────────→┌─────────────────┐
                       │     Jaeger       │
                       │  (:16686 UI)     │
                       └─────────────────┘
```

## 特徴

- **Envoy がトレースに参加**: Envoy 自身がスパンを生成し、Jaeger にエクスポート
- **W3C Trace Context**: Envoy の OpenTelemetry tracer で `traceparent` ヘッダーを生成・伝搬
- **OTLP gRPC エクスポート**: Envoy → Jaeger (:4317) へ OTLP gRPC で直接送信
- **AlwaysSample**: 全リクエストをトレース（Envoy 側も 100% サンプリング）

## トレース構造

`curl http://localhost:10000/hello` のトレース:

```
envoy-front-proxy: ingress_http              ← Envoy が生成
└── gateway: server (otelhttp)               ← Gateway が生成
    ├── gateway: grpc.Greeter/SayHello       ← gRPC クライアント
    │   └── backend: grpc.Greeter/SayHello   ← gRPC サーバー
    │       └── backend: operation (200ms)
    └── gateway: op1 (100ms)
```

Envoy・Gateway・Backend の3サービスのスパンが同一トレースに統合されます。

## Envoy 設定（sidecar/envoy.yaml）

```yaml
tracing:
  provider:
    name: envoy.tracers.opentelemetry
    typed_config:
      "@type": .../OpenTelemetryConfig
      grpc_service:
        envoy_grpc:
          cluster_name: otel_collector   # Jaeger の OTLP gRPC エンドポイント
      service_name: envoy-front-proxy
```

## microservice/ との違い

| 項目 | envoy/ (本サンプル) | microservice/ |
|------|--------------------|---------------|
| フロントプロキシ | Envoy (:10000) | なし（Gateway に直接アクセス） |
| Envoy のスパン | あり | なし |
| 伝搬フォーマット | W3C (Envoy OTel tracer) | W3C (OTel SDK のみ) |
| クライアントのアクセス先 | `:10000` (Envoy) | `:8000` (Gateway) |

## 使い方

```bash
docker compose up --build

# Envoy 経由でリクエスト送信
curl http://localhost:10000/hello

# Jaeger UI でトレースを確認
open http://localhost:16686

# Envoy admin UI
open http://localhost:9901
```
