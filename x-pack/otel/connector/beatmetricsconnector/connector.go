// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package beatmetricsconnector

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
)

type beatMetricsConnector struct {
	logger       *zap.Logger
	logsConsumer consumer.Logs
}

func (c *beatMetricsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *beatMetricsConnector) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *beatMetricsConnector) Shutdown(_ context.Context) error {
	return nil
}

func (c *beatMetricsConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	metricsMap := make(map[string]any)
	datasetMap := make(map[string]any)
	var latestTimestamp pcommon.Timestamp

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				c.processMetric(m, metricsMap, datasetMap, &latestTimestamp)
			}
		}
	}

	if len(metricsMap) == 0 && len(datasetMap) == 0 {
		return nil
	}

	body := make(map[string]any)
	if len(metricsMap) > 0 {
		body["metrics"] = metricsMap
	}
	if len(datasetMap) > 0 {
		body["dataset"] = datasetMap
	}

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().Attributes().PutStr("elastic.mapping.mode", "bodymap")
	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(latestTimestamp)

	if err := lr.Body().SetEmptyMap().FromRaw(body); err != nil {
		return err
	}

	return c.logsConsumer.ConsumeLogs(ctx, logs)
}

func (c *beatMetricsConnector) processMetric(
	m pmetric.Metric,
	metricsMap map[string]any,
	datasetMap map[string]any,
	latestTimestamp *pcommon.Timestamp,
) {
	name := m.Name()

	switch m.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
			dp := m.Gauge().DataPoints().At(i)
			c.processDataPoint(name, dp, metricsMap, datasetMap, latestTimestamp)
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < m.Sum().DataPoints().Len(); i++ {
			dp := m.Sum().DataPoints().At(i)
			c.processDataPoint(name, dp, metricsMap, datasetMap, latestTimestamp)
		}
	default:
		c.logger.Debug("unsupported metric type", zap.String("name", name), zap.String("type", m.Type().String()))
	}
}

func (c *beatMetricsConnector) processDataPoint(
	name string,
	dp pmetric.NumberDataPoint,
	metricsMap map[string]any,
	datasetMap map[string]any,
	latestTimestamp *pcommon.Timestamp,
) {
	ts := dp.Timestamp()
	if ts > *latestTimestamp {
		*latestTimestamp = ts
	}

	value := extractDataPointValue(dp)

	inputID, hasInputID := dp.Attributes().Get("input_id")
	if hasInputID {
		key := sanitizeID(inputID.Str())
		entry, ok := datasetMap[key].(map[string]any)
		if !ok {
			entry = make(map[string]any)
			entry["id"] = inputID.Str()
			if inputType, found := dp.Attributes().Get("input_type"); found {
				entry["input"] = inputType.Str()
			}
			datasetMap[key] = entry
		}
		setNestedValue(entry, name, value)
	} else {
		setNestedValue(metricsMap, name, value)
	}
}

func extractDataPointValue(dp pmetric.NumberDataPoint) any {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return dp.IntValue()
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	default:
		return nil
	}
}

// setNestedValue sets a value in a nested map using a dot-separated key.
// For example, setNestedValue(m, "beat.memstats.rss", 123) creates
// m["beat"]["memstats"]["rss"] = 123.
func setNestedValue(m map[string]any, dottedKey string, value any) {
	parts := strings.Split(dottedKey, ".")
	current := m
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

// sanitizeID replaces dots with underscores in an input ID, matching
// the behavior of libbeat/monitoring/inputmon/input.go:sanitizeID.
func sanitizeID(id string) string {
	return strings.ReplaceAll(id, ".", "_")
}
