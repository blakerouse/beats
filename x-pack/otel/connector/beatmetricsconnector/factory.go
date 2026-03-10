// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package beatmetricsconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const Name = "beatmetrics"

// NewFactory creates a factory for the beatmetrics connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		component.MustNewType(Name),
		createDefaultConfig,
		connector.WithMetricsToLogs(createMetricsToLogs, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsToLogs(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	nextConsumer consumer.Logs,
) (connector.Metrics, error) {
	return &beatMetricsConnector{
		logger:       set.Logger,
		logsConsumer: nextConsumer,
	}, nil
}
