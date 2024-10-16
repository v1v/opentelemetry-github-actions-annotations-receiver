package opentelemetrygithubactionsannotationsreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: fmt.Sprintf("%s:%d", "0.0.0.0", defaultPort),
		},
		Path: defaultPath,
		Retry: RetryConfig{
			InitialInterval: defaultRetryInitialInterval,
			MaxInterval:     defaultRetryMaxInterval,
			MaxElapsedTime:  defaultRetryMaxElapsedTime,
		},
		BatchSize: 10000,
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
