package opentelemetrygithubactionsannotationsreceiver

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
)

type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
	Path                    string
	WebhookSecret           configopaque.String `mapstructure:"webhook_secret"`
	Token                   configopaque.String `mapstructure:"token"`
	Retry                   RetryConfig         `mapstructure:"retry"`
	BatchSize               int                 `mapstructure:"batch_size"`
	CustomServiceName       string              `mapstructure:"custom_service_name"`
	ServiceNamePrefix       string              `mapstructure:"service_name_prefix"`
	ServiceNameSuffix       string              `mapstructure:"service_name_suffix"`
}

type RetryConfig struct {
	InitialInterval time.Duration `mapstructure:"initial_interval"`
	MaxInterval     time.Duration `mapstructure:"max_interval"`
	MaxElapsedTime  time.Duration `mapstructure:"max_elapsed_time"`
}
