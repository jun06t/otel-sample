# Collector

OpenTelemetry Collector を介してトレースを複数バックエンド（Jaeger + Zipkin）にエクスポートするサンプルです。

## アーキテクチャ

```
┌──────────┐     gRPC     ┌──────────────┐
│ Gateway  │─────────────→│ Backend-gRPC │
│ (:8000)  │              │ (:8080)      │
└────┬─────┘              └──────┬───────┘
     │ OTLP gRPC                 │ OTLP gRPC
     └──────────┬────────────────┘
                ▼
       ┌─────────────────┐
       │  OTel Collector  │
       │  (:4317)         │
       └───┬─────────┬───┘
           ▼         ▼
    ┌──────────┐ ┌──────────┐
    │  Jaeger  │ │  Zipkin  │
    │ (:16686) │ │ (:9411)  │
    └──────────┘ └──────────┘
```

## 特徴

- **OTel Collector 経由**: アプリは OTLP gRPC で Collector に送信、Collector が各バックエンドへ分配
- **マルチバックエンド**: 1つのトレースデータを Jaeger と Zipkin の両方に同時エクスポート
- **OTLP プロトコル**: Jaeger HTTP エクスポーターではなく、標準の OTLP gRPC を使用
- **ParentBased サンプリング**: Gateway=100%, Backend=50%（Parent の判断を継承）

## microservice/ との違い

| 項目 | collector/ (本サンプル) | microservice/ |
|------|----------------------|---------------|
| エクスポート先 | OTel Collector | Jaeger へ直接 |
| プロトコル | OTLP gRPC | Jaeger HTTP |
| バックエンド | Jaeger + Zipkin | Jaeger のみ |
| Collector | あり | なし |

## Collector 設定

```yaml
receivers:
  otlp:
    protocols:
      grpc:          # :4317 で OTLP gRPC を受信

processors:
  batch:             # バッチ処理で効率化

exporters:
  jaeger:            # Jaeger gRPC (:14250)
  zipkin:            # Zipkin HTTP (:9411)
```

## 使い方

```bash
docker compose up --build
curl http://localhost:8000/hello

# Jaeger UI
open http://localhost:16686

# Zipkin UI
open http://localhost:9411
```
