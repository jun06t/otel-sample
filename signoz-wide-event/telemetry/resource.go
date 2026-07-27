package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// NewResource は traces と logs で共有する resource を生成する。
// 同じ resource を TracerProvider / LoggerProvider の両方に渡すことで、
// service.name などの次元でもトレースとログを突き合わせられる。
func NewResource(serviceName, version, environment string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(version),
		attribute.String("environment", environment),
	)
}
