# ifritah-go

## Logging and metrics

The API emits structured `log/slog` records. `LOG_FORMAT` accepts `json`
(default) or `text`, and `LOG_LEVEL` accepts `debug`, `info`, `warn`, or
`error`. Request completion records include a validated `X-Request-ID`,
method, route template, status, duration, trusted authenticated user ID, and
server-derived tenant/company context when configured. Request bodies, tokens,
passwords, SQL, and arbitrary client text are not logged. Legacy logging
wrappers retain only a bounded operation name and error type; their arguments
are never rendered.

JetStream publish calls are bounded by `NATS_PUBLISH_TIMEOUT` (default `5s`,
maximum `1m`). Request cancellation always takes precedence, including for
legacy publish wrappers.

`GET /metrics` is operator-only. Set `METRICS_TOKEN` and send
`Authorization: Bearer <METRICS_TOKEN>`; when unset, the endpoint returns 404,
and missing or incorrect credentials return 403. Metrics use bounded method,
route-template, and status-class labels. Request IDs, tenant IDs, user IDs, and
resource IDs are intentionally excluded from metric labels.

## OpenTelemetry tracing

Tracing is fail-open and disabled unless an OTLP endpoint is configured. The
backend uses the OTLP HTTP/protobuf exporter, which is suitable for
OpenObserve. Configure `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` (or the general
`OTEL_EXPORTER_OTLP_ENDPOINT`), optional comma-separated
`OTEL_EXPORTER_OTLP_TRACES_HEADERS`, and
`OTEL_EXPORTER_OTLP_TRACES_INSECURE=true` for a local HTTP collector. An
OpenObserve base URL such as
`http://openobserve:5080/api/default` is accepted; the backend sends traces to
its `/v1/traces` path. A bare collector URL is normalized the same way, while
an endpoint that already ends in `/v1/traces` is used once without
double-appending. Signal-specific OTEL variables take precedence over their
general counterparts.

Headers are deployment-provided OTLP collector headers, so they can carry
per-tenant authentication or stream/organization identity without hard-coding
an OpenObserve contract in the backend. Header names and values are bounded
and application configuration/exporter warnings do not log their values.
Incoming request headers are never used for tenant attribution; the
`tenant.id` span attribute comes only from trusted server-derived context
(`TENANT_ID`/`DBNAME`).

Gin middleware extracts and creates W3C trace context. Existing structured
events add `trace_id` and `span_id` only while a valid span is active, without
changing request IDs, trusted tenant/company context, redaction, or protected
Prometheus metrics. External VIN HTTP calls, NATS publishing, and the startup
database ping create dependency spans where context is available. Exception
events contain only bounded error types and a redacted message marker.

Unset the endpoint or set `OTEL_TRACES_EXPORTER=none` to preserve the
telemetry-disabled behavior. Export uses a bounded non-blocking queue,
batch/export timeouts, rate-limited exporter warnings, and a bounded graceful
shutdown; an unavailable OpenObserve instance does not fail requests or stop
the API.

Validation note: `go vet ./...` still reports the pre-existing generated
`pkg/db/gen/models.go:994` warning about an unexported field with a JSON tag;
full-tree `gofmt -l` still reports the pre-existing generated
`pkg/db/gen/config.go` formatting difference; generated database files are not
modified by this instrumentation.