---
id: tracing
title: Tracing
---

OAuth2 Proxy has support for tracing using [OpenTelemetry](https://opentelemetry.io/).

### Configuration

To follow upstream, tracing is configured through [environment variables](https://opentelemetry.io/docs/reference/specification/sdk-environment-variables/).
Not all are supported, see [opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go) for more details.

Trace propagation will be handled automatically for the W3C Trace Context and W3C Baggage specifications, or [OTEL_PROPAGATORS](https://pkg.go.dev/go.opentelemetry.io/contrib/propagators/autoprop) can be used to configure others.

### Quick start

Here are some example configurations:

```
otlp_endpoint=http://host-name:4318
otlp-protocol=grpc
otlp-insecure=true
otlp-metrics=false
```
