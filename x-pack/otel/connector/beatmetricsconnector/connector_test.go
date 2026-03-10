// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package beatmetricsconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

func newTestConnector(t *testing.T) (*beatMetricsConnector, *consumertest.LogsSink) {
	t.Helper()
	sink := &consumertest.LogsSink{}
	c := &beatMetricsConnector{
		logger:       zaptest.NewLogger(t),
		logsConsumer: sink,
	}
	return c, sink
}

func TestConsumeMetrics(t *testing.T) {
	c, sink := newTestConnector(t)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Add a gauge metric: beat.memstats.rss = 123
	m := sm.Metrics().AppendEmpty()
	m.SetName("beat.memstats.rss")
	m.SetEmptyGauge()
	dp := m.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(123)

	// Add another metric: beat.cpu.total.value = 50
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("beat.cpu.total.value")
	m2.SetEmptyGauge()
	dp2 := m2.Gauge().DataPoints().AppendEmpty()
	dp2.SetIntValue(50)

	err := c.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	require.Equal(t, 1, sink.LogRecordCount())

	logs := sink.AllLogs()[0]
	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	body := lr.Body().Map().AsRaw()

	metrics, ok := body["metrics"].(map[string]any)
	require.True(t, ok, "body should contain 'metrics' key")

	beat, ok := metrics["beat"].(map[string]any)
	require.True(t, ok)

	memstats, ok := beat["memstats"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, int64(123), memstats["rss"])

	cpu, ok := beat["cpu"].(map[string]any)
	require.True(t, ok)
	total, ok := cpu["total"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, int64(50), total["value"])
}

func TestPerInputMetrics(t *testing.T) {
	c, sink := newTestConnector(t)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	m := sm.Metrics().AppendEmpty()
	m.SetName("events_processed_total")
	m.SetEmptyGauge()
	dp := m.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(100)
	dp.Attributes().PutStr("input_id", "filestream-1")
	dp.Attributes().PutStr("input_type", "filestream")

	err := c.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	require.Equal(t, 1, sink.LogRecordCount())

	logs := sink.AllLogs()[0]
	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	body := lr.Body().Map().AsRaw()

	dataset, ok := body["dataset"].(map[string]any)
	require.True(t, ok, "body should contain 'dataset' key")

	entry, ok := dataset["filestream-1"].(map[string]any)
	require.True(t, ok, "dataset should contain 'filestream-1' key")

	assert.Equal(t, "filestream-1", entry["id"])
	assert.Equal(t, "filestream", entry["input"])
	assert.Equal(t, int64(100), entry["events_processed_total"])
}

func TestPerInputMetricsDotSanitization(t *testing.T) {
	c, sink := newTestConnector(t)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	m := sm.Metrics().AppendEmpty()
	m.SetName("events_total")
	m.SetEmptyGauge()
	dp := m.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(42)
	dp.Attributes().PutStr("input_id", "filestream.v2")
	dp.Attributes().PutStr("input_type", "filestream")

	err := c.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)

	body := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()
	dataset := body["dataset"].(map[string]any)

	// Dots should be replaced with underscores in the key
	_, hasDotKey := dataset["filestream.v2"]
	assert.False(t, hasDotKey, "should not have dot-separated key")

	entry, ok := dataset["filestream_v2"].(map[string]any)
	require.True(t, ok, "dataset should contain sanitized key 'filestream_v2'")
	assert.Equal(t, "filestream.v2", entry["id"]) // original id preserved
}

func TestBodymapAttribute(t *testing.T) {
	c, sink := newTestConnector(t)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test.metric")
	m.SetEmptyGauge()
	m.Gauge().DataPoints().AppendEmpty().SetIntValue(1)

	err := c.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	require.Equal(t, 1, sink.LogRecordCount())

	sl := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0)
	val, ok := sl.Scope().Attributes().Get("elastic.mapping.mode")
	require.True(t, ok, "scope attributes should contain 'elastic.mapping.mode'")
	assert.Equal(t, "bodymap", val.Str())
}

func TestNestedMapStructure(t *testing.T) {
	c, sink := newTestConnector(t)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	m := sm.Metrics().AppendEmpty()
	m.SetName("beat.memstats.rss")
	m.SetEmptyGauge()
	m.Gauge().DataPoints().AppendEmpty().SetIntValue(456)

	err := c.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)

	body := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	// Verify nesting: body["metrics"]["beat"]["memstats"]["rss"] = 456
	metrics := body["metrics"].(map[string]any)
	beat := metrics["beat"].(map[string]any)
	memstats := beat["memstats"].(map[string]any)
	assert.Equal(t, int64(456), memstats["rss"])
}

func TestEmptyMetrics(t *testing.T) {
	c, sink := newTestConnector(t)

	md := pmetric.NewMetrics()
	err := c.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	assert.Equal(t, 0, sink.LogRecordCount(), "empty metrics should produce no logs")
}

func TestMixedMetricTypes(t *testing.T) {
	c, sink := newTestConnector(t)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Int gauge
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("int_gauge")
	m1.SetEmptyGauge()
	m1.Gauge().DataPoints().AppendEmpty().SetIntValue(42)

	// Float gauge
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("float_gauge")
	m2.SetEmptyGauge()
	m2.Gauge().DataPoints().AppendEmpty().SetDoubleValue(3.14)

	// Int sum
	m3 := sm.Metrics().AppendEmpty()
	m3.SetName("int_sum")
	m3.SetEmptySum()
	m3.Sum().DataPoints().AppendEmpty().SetIntValue(1000)

	err := c.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)

	body := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()
	metrics := body["metrics"].(map[string]any)

	assert.Equal(t, int64(42), metrics["int_gauge"])
	assert.Equal(t, 3.14, metrics["float_gauge"])
	assert.Equal(t, int64(1000), metrics["int_sum"])
}

func TestSetNestedValue(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    any
		expected map[string]any
	}{
		{
			name:  "single key",
			key:   "foo",
			value: 1,
			expected: map[string]any{
				"foo": 1,
			},
		},
		{
			name:  "nested key",
			key:   "a.b.c",
			value: "hello",
			expected: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": "hello",
					},
				},
			},
		},
		{
			name:  "two level key",
			key:   "beat.rss",
			value: int64(99),
			expected: map[string]any{
				"beat": map[string]any{
					"rss": int64(99),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := make(map[string]any)
			setNestedValue(m, tt.key, tt.value)
			assert.Equal(t, tt.expected, m)
		})
	}
}

func TestTimestamp(t *testing.T) {
	c, sink := newTestConnector(t)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("metric1")
	m1.SetEmptyGauge()
	dp1 := m1.Gauge().DataPoints().AppendEmpty()
	dp1.SetIntValue(1)
	dp1.SetTimestamp(100)

	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("metric2")
	m2.SetEmptyGauge()
	dp2 := m2.Gauge().DataPoints().AppendEmpty()
	dp2.SetIntValue(2)
	dp2.SetTimestamp(200)

	err := c.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)

	lr := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, uint64(200), uint64(lr.Timestamp()), "should use the most recent timestamp")
}
