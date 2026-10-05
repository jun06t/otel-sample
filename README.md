# otel-sample

A collection of OpenTelemetry instrumentation samples written in Go.
Each directory is an independent Go module with its own `README.md` (in Japanese) describing the architecture and how to run it.

## Samples

### Basics

| Directory | Description | Backend |
|---|---|---|
| [simple](./simple) | Minimal tracing setup for a single HTTP API | Jaeger |
| [multi-package](./multi-package) | Tracing across multiple Go packages, each with its own tracer, while keeping parent-child span relationships | Jaeger |
| [microservice](./microservice) | Distributed tracing across HTTP and gRPC services (Gateway → Backend) | Jaeger |
| [bridge](./bridge) | Migrating from OpenCensus to OpenTelemetry with the OpenCensus bridge (Bigtable emulator) | Jaeger |

### Infrastructure / Pipeline

| Directory | Description | Backend |
|---|---|---|
| [collector](./collector) | Exporting traces through the OpenTelemetry Collector to multiple backends | Jaeger, Zipkin |
| [envoy](./envoy) | Distributed tracing including Envoy sidecar proxies between services | Jaeger |

### Sampling

| Directory | Description | Backend |
|---|---|---|
| [sampling](./sampling) | Head-based sampling with the SDK `TraceIDRatioBased` sampler | Jaeger |
| [tail-sampling](./tail-sampling) | Tail-based sampling with the Collector `tailsampling` processor | Jaeger |
| [refinery](./refinery) | Tail-based dynamic sampling with Honeycomb Refinery | Honeycomb |

### Workload Patterns

| Directory | Description | Backend |
|---|---|---|
| [batch](./batch) | Run-to-completion batch jobs (K8s Job / CronJob): one run = one trace, flush before exit | Jaeger |
| [worker](./worker) | Long-running Pub/Sub workers: one message = one trace, spans created by the Pub/Sub SDK | Jaeger |
| [span-link](./span-link) | Linking messaging publish and process spans with span links instead of parent-child | Jaeger |

### Wide Events (Observability 2.0)

| Directory | Description | Backend |
|---|---|---|
| [signoz-wide-event](./signoz-wide-event) | Co-locating zap logs and traces in SigNoz and putting request context into span attributes | SigNoz |
| [event-comparison](./event-comparison) | Treating spans as wide events and choosing where to record everything else (span attributes, log-based events, plain log records), with span events for comparison | stdout / SigNoz |

## Usage

Most samples run with Docker Compose:

```bash
cd <sample>
docker compose up --build
```

Then open the Jaeger UI at http://localhost:16686 (or the backend described in each sample's README).
Some samples need extra setup: `refinery` requires a Honeycomb API key, `signoz-wide-event` requires a separately running SigNoz, and `event-comparison` runs with `go run .` and prints to stdout. See each sample's README for details.
