# Beat Metrics Connector

| Status    |                                  |
| --------- | -------------------------------- |
| Stability | [development]: metrics_to_logs   |

[development]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#development

The Beat Metrics connector (`beatmetrics`) is an OpenTelemetry Collector connector that converts Beat monitoring metrics (`pmetric.Metrics`) into logs (`plog.Logs`) with the `elastic.mapping.mode: bodymap` encoding, so the Elasticsearch exporter can index them as monitoring documents.

> [!NOTE]
> This component is only expected to work correctly with metrics from the Beat internal telemetry (via OTLP receiver from the collector's internal telemetry pipeline).
> This is because it relies on the specific structure and naming conventions of metrics emitted by Beat's [RegistryBridge] and [SystemRegistryBridge].
> Using it with metrics coming from other components is not recommended and may result in unexpected behavior.

## How it works

The connector receives OTel metrics and produces a single log document per batch with the following structure:

```json
{
  "metrics": {
    "beat": {"memstats": {"rss": 123}, "cpu": {"total": {"value": 50}}},
    "system": {"load": {"1": 1.5, "5": 2.0}},
    "libbeat": {"pipeline": {"events": {"total": 1000}}}
  },
  "dataset": {
    "filestream_1": {
      "id": "filestream-1",
      "input": "filestream",
      "events_processed_total": 100
    }
  }
}
```

- **`metrics`**: Stats and system metrics. Dot-separated OTel metric names are converted to a nested map structure.
- **`dataset`**: Per-input metrics grouped by sanitized `input_id` (dots replaced with underscores). Includes `id` and `input` string fields from data point attributes.

The connector handles `Gauge` and `Sum` metric types, extracting `int64` or `float64` values from their data points.

## Example

```yaml
connectors:
  beatmetrics: {}

service:
  pipelines:
    metrics/telemetry:
      receivers: [otlp]
      exporters: [beatmetrics]
    logs/monitoring:
      receivers: [beatmetrics]
      exporters: [elasticsearch]
```

In the above configuration, the `otlp` receiver collects Beat internal telemetry metrics. The `beatmetrics` connector converts those metrics into a log document and forwards it to the `elasticsearch` exporter for indexing as a monitoring document.

[RegistryBridge]: https://github.com/elastic/beats/blob/main/libbeat/monitoring/report/otel/otel.go
[SystemRegistryBridge]: https://github.com/elastic/beats/blob/main/libbeat/monitoring/report/otel/otel.go
