package opentelemetrygithubactionsannotationsreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:33333",
		},
		Path: "/githubactionsannotations",
	}
}

func createTracerReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	return newLogsReceiver(cfg, params, nextConsumer)
}

// NewFactory creates a factory for githubactionsannotationsreceiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType("githubactionsannotations"),
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelAlpha),
	)
}

func createLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	return newLogsReceiver(cfg, params, consumer)
}
