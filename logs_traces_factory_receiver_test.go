package opentelemetrygithubactionsannotationsreceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestAttachDataAndAttributes(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	repository := Repository{FullName: "org/repo"}
	run := Run{
		ID:           10,
		RunAttempt:   1,
		Conclusion:   "success",
		Status:       "completed",
		RunStartedAt: now.Add(-time.Minute),
		CreatedAt:    now.Add(-2 * time.Minute),
		CompletedAt:  now,
		HeadBranch:   "main",
		URL:          "https://github.com/org/repo/actions/runs/10",
	}
	line := LogLine{
		Body:           "lint error",
		Timestamp:      now,
		SeverityNumber: int(plog.SeverityNumberWarn),
		SeverityText:   "warning",
	}

	record := plog.NewLogRecord()
	err := attachData(&record, repository, run, line)
	require.NoError(t, err)

	require.Equal(t, plog.SeverityNumberWarn, record.SeverityNumber())
	require.Equal(t, "warning", record.SeverityText())
	require.Equal(t, "lint error", record.Body().Str())
	require.NotEqual(t, [16]byte{}, [16]byte(record.TraceID()))

	attrs := record.Attributes()
	fullName, ok := attrs.Get("github.repository")
	require.True(t, ok)
	require.Equal(t, "org/repo", fullName.Str())
}

func TestParseAnnotationToLogLine(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	msg := "problem found"
	level := "failure"
	ann := &github.CheckRunAnnotation{Message: &msg, AnnotationLevel: &level}

	line := parseAnnotationToLogLine(now, ann)

	require.Equal(t, "problem found", line.Body)
	require.Equal(t, "failure", line.SeverityText)
	require.Equal(t, 0, line.SeverityNumber)
	require.Equal(t, now, line.Timestamp)
}

func TestGenerateTraceIDDeterministic(t *testing.T) {
	traceID1, err := generateTraceID(99, 2)
	require.NoError(t, err)

	traceID2, err := generateTraceID(99, 2)
	require.NoError(t, err)

	traceID3, err := generateTraceID(99, 3)
	require.NoError(t, err)

	require.Equal(t, traceID1, traceID2)
	require.NotEqual(t, traceID1, traceID3)
}

func TestGenerateServiceName(t *testing.T) {
	cfg := &Config{ServiceNamePrefix: "pre-", ServiceNameSuffix: "-suf"}
	require.Equal(t, "pre-org-repo-name-suf", generateServiceName(cfg, "Org/Repo_Name"))

	cfg.CustomServiceName = "custom-name"
	require.Equal(t, "custom-name", generateServiceName(cfg, "ignored/value"))
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	require.Equal(t, defaultPath, cfg.Path)
	require.Equal(t, "0.0.0.0:33333", cfg.ServerConfig.Endpoint)
	require.Equal(t, 10000, cfg.BatchSize)
	require.Equal(t, defaultRetryInitialInterval, cfg.Retry.InitialInterval)
	require.Equal(t, defaultRetryMaxInterval, cfg.Retry.MaxInterval)
	require.Equal(t, defaultRetryMaxElapsedTime, cfg.Retry.MaxElapsedTime)
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, component.MustNewType("githubactionsannotations"), factory.Type())
}

type fakeLogsConsumer struct {
	consumeFn func(context.Context, plog.Logs) error
	calls     int
}

func (f *fakeLogsConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	f.calls++
	return f.consumeFn(ctx, ld)
}

func (f *fakeLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestConsumeLogsWithRetrySuccess(t *testing.T) {
	rec := &githubactionsannotationsreceiver{
		config: &Config{Retry: RetryConfig{InitialInterval: time.Millisecond, MaxInterval: time.Millisecond, MaxElapsedTime: 10 * time.Millisecond}},
		logger: zap.NewNop(),
	}
	consumer := &fakeLogsConsumer{consumeFn: func(context.Context, plog.Logs) error { return nil }}
	rec.consumer = consumer

	err := rec.consumeLogsWithRetry(context.Background(), func(fields ...zap.Field) []zap.Field { return fields }, plog.NewLogs())

	require.NoError(t, err)
	require.Equal(t, 1, consumer.calls)
}

func TestConsumeLogsWithRetryPermanentError(t *testing.T) {
	rec := &githubactionsannotationsreceiver{
		config: &Config{Retry: RetryConfig{InitialInterval: time.Millisecond, MaxInterval: time.Millisecond, MaxElapsedTime: 10 * time.Millisecond}},
		logger: zap.NewNop(),
	}
	consumeErr := consumererror.NewPermanent(errors.New("boom"))
	consumer := &fakeLogsConsumer{consumeFn: func(context.Context, plog.Logs) error { return consumeErr }}
	rec.consumer = consumer

	err := rec.consumeLogsWithRetry(context.Background(), func(fields ...zap.Field) []zap.Field { return fields }, plog.NewLogs())

	require.ErrorIs(t, err, consumeErr)
	require.Equal(t, 1, consumer.calls)
}

func TestConsumeLogsWithRetryContextCanceled(t *testing.T) {
	rec := &githubactionsannotationsreceiver{
		config: &Config{Retry: RetryConfig{InitialInterval: time.Millisecond, MaxInterval: time.Millisecond, MaxElapsedTime: 50 * time.Millisecond}},
		logger: zap.NewNop(),
	}
	consumer := &fakeLogsConsumer{consumeFn: func(context.Context, plog.Logs) error { return errors.New("retryable") }}
	rec.consumer = consumer

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rec.consumeLogsWithRetry(ctx, func(fields ...zap.Field) []zap.Field { return fields }, plog.NewLogs())

	require.ErrorContains(t, err, "context is cancelled or timed out")
	require.GreaterOrEqual(t, consumer.calls, 1)
}
