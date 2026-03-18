# Envoy Sidecar

各マイクロサービスに Envoy サイドカーを配置し、サービス間通信を含めた分散トレーシングを実現するサンプルです。

## アーキテクチャ

```
              Envoy                              Envoy
            (GW sidecar)                      (Backend sidecar)
Client ──→ [:10000 HTTP] ──→ Gateway ──→ [:10001 gRPC] ──→ Backend-gRPC
                              (:8000)                        (:8080)

  Envoy-GW ─┐        Gateway ─┐       Envoy-Backend ─┐   Backend ─┐
   OTLP gRPC│    Jaeger HTTP  │         OTLP gRPC    │  Jaeger HTTP│
             └──→ Jaeger (:16686) ←────────────────────┘←──────────┘
```

**ポイント**: Gateway は `envoy-backend:10001` に gRPC 接続する（`backend-grpc:8080` に直接ではない）。
Backend への通信も Envoy サイドカーを経由するため、サービス間のネットワークレイヤもトレースに含まれます。

## 特徴

- **サイドカーパターン**: 各サービスに専用の Envoy を配置（envoy-gateway, envoy-backend）
- **W3C Trace Context**: Envoy の OpenTelemetry tracer で `traceparent` ヘッダーを生成・伝搬
- **4サービスのスパンが統合**: Envoy-GW → Gateway → Envoy-Backend → Backend-gRPC
- **gRPC プロキシ**: Envoy-Backend が gRPC (HTTP/2) をプロキシし、トレースに参加

## トレース構造

`curl http://localhost:10000/hello` のトレース:

```
envoy-gateway: inbound_http                     ← Envoy-GW が生成
└── gateway: server (otelhttp)                   ← Gateway が生成
    ├── gateway: grpc.Greeter/SayHello           ← gRPC クライアント
    │   └── envoy-backend: inbound_grpc          ← Envoy-Backend が生成
    │       └── backend: grpc.Greeter/SayHello   ← Backend が生成
    │           └── backend: operation (200ms)
    └── gateway: op1 (100ms)
```

## Envoy 設定

| ファイル | 役割 | Listen | 転送先 |
|---------|------|--------|--------|
| `envoy-gateway.yaml` | GW サイドカー（HTTP 受信） | `:10000` | `gateway:8000` |
| `envoy-backend.yaml` | Backend サイドカー（gRPC 受信） | `:10001` | `backend-grpc:8080` |

両方とも OpenTelemetry tracer で Jaeger (:4317) に OTLP gRPC でトレースをエクスポートします。

## microservice/ との違い

| 項目 | envoy/ (本サンプル) | microservice/ |
|------|--------------------|---------------|
| Envoy サイドカー | 各サービスに配置 | なし |
| サービス間通信 | Envoy 経由 | 直接 gRPC |
| トレース対象 | アプリ + ネットワーク層 | アプリのみ |
| スパン数 | 多い（Envoy 分が追加） | 少ない |
| クライアントのアクセス先 | `:10000` (Envoy-GW) | `:8000` (Gateway) |

## 使い方

```bash
docker compose up --build

# Envoy-GW 経由でリクエスト送信
curl http://localhost:10000/hello

# Jaeger UI でトレースを確認（4サービスのスパンが統合）
open http://localhost:16686

# Envoy-GW admin UI
open http://localhost:9901
```
