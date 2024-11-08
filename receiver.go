package opentelemetrygithubactionsannotationsreceiver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-github/v66/github"
	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"go.uber.org/zap"
)

func newLogsReceiver(cfg *Config, params receiver.CreateSettings, consumer consumer.Logs) (*githubactionsannotationsreceiver, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              "http",
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}
	ghClient, err := createGitHubClient(cfg.GitHubAuth)
	if err != nil {
		return nil, err
	}
	rateLimit, _, err := ghClient.RateLimit.Get(context.Background())
	if err != nil {
		return nil, err
	}
	params.Logger.Info("GitHub API rate limit", zap.Int("limit", rateLimit.GetCore().Limit), zap.Int("remaining", rateLimit.GetCore().Remaining), zap.Time("reset", rateLimit.GetCore().Reset.Time))
	return &githubactionsannotationsreceiver{
		config:   cfg,
		consumer: consumer,
		settings: params,
		logger:   params.Logger,
		ghClient: ghClient,
		obsrecv:  obsrecv,
	}, nil
}

type githubactionsannotationsreceiver struct {
	config   *Config
	consumer consumer.Logs
	server   *http.Server
	settings receiver.CreateSettings
	logger   *zap.Logger
	ghClient *github.Client
	obsrecv  *receiverhelper.ObsReport
}

func (rec *githubactionsannotationsreceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", rec.config.ServerConfig.Endpoint, rec.config.Path)
	rec.logger.Info("Starting receiver", zap.String("endpoint", endpoint))
	listener, err := rec.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}
	router := httprouter.New()
	router.POST(rec.config.Path, rec.handleEvent)
	rec.server, err = rec.config.ServerConfig.ToServer(ctx, host, rec.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}
	go func() {
		if err := rec.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			rec.settings.TelemetrySettings.ReportStatus(component.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

func (rec *githubactionsannotationsreceiver) Shutdown(context.Context) error {
	return nil
}

func (rec *githubactionsannotationsreceiver) handleEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var payload []byte
	var err error
	payload, err = github.ValidatePayload(r, []byte(string(rec.config.WebhookSecret)))
	if err != nil {
		rec.logger.Error("Invalid payload", zap.Error(err))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		rec.logger.Error("Error parsing webhook", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch event := event.(type) {
	case *github.WorkflowJobEvent:
		ctx := context.WithoutCancel(r.Context())
		rec.handleWorkflowJobEvent(ctx, event, w, r, nil)
	default:
		{
			// TODO: avoid verbosity while running this
			//rec.logger.Debug("Skipping the request because it is not a workflow_job event", zap.Any("event", event))
			w.WriteHeader(http.StatusOK)
		}
	}
}

func (rec *githubactionsannotationsreceiver) handleWorkflowJobEvent(ctx context.Context, event *github.WorkflowJobEvent, w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	rec.logger.Debug("Handling workflow job event", zap.Int64("workflow_job.id", event.WorkflowJob.GetID()))
	if event.GetAction() != "completed" {
		// TODO: avoid verbosity while running this
		//rec.logger.Debug("Skipping the request because it is not a completed workflow_job event", zap.Any("event", workflowJobEvent))
		w.WriteHeader(http.StatusOK)
		return
	}
	var withWorkflowInfoFields = func(fields ...zap.Field) []zap.Field {
		workflowInfoFields := []zap.Field{
			zap.String("github.repository", event.GetRepo().GetFullName()),
			zap.String("github.workflow_run.name", *event.GetWorkflowJob().WorkflowName),
			zap.Int64("github.workflow_run.id", event.GetWorkflowJob().GetRunID()),
			zap.Int("github.workflow_run.run_attempt", int(event.GetWorkflowJob().GetRunAttempt())),
		}
		return append(workflowInfoFields, fields...)
	}

	rec.logger.Info("Starting to process webhook event", withWorkflowInfoFields()...)
	err := rec.processWorkflowJobEvent(ctx, withWorkflowInfoFields, event)
	if err != nil {
		rec.logger.Error("Failed to process webhook event", withWorkflowInfoFields(zap.Error(err))...)
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}

func (rec *githubactionsannotationsreceiver) processWorkflowJobEvent(
	ctx context.Context,
	withWorkflowInfoFields func(fields ...zap.Field) []zap.Field,
	event *github.WorkflowJobEvent,
) error {
	annotations, err := getAnnotations(context.Background(), event, rec.ghClient)
	if err != nil {
		rec.logger.Error("Failed to get job annotations", zap.Error(err))
	}

	run := mapRun(event.WorkflowJob)
	repository := mapRepository(event.GetRepo())
	_, err = rec.processAnnotations(ctx, annotations, repository, run, withWorkflowInfoFields)
	if err != nil {
		return err
	}
	return nil
}

func getAnnotations(ctx context.Context, ghEvent *github.WorkflowJobEvent, ghClient *github.Client) ([]*github.CheckRunAnnotation, error) {
	listOpts := &github.ListOptions{
		PerPage: 100,
	}
	var allAnnotations []*github.CheckRunAnnotation
	for {
		annotations, response, err := ghClient.Checks.ListCheckRunAnnotations(ctx, ghEvent.GetRepo().GetOwner().GetLogin(), ghEvent.GetRepo().GetName(), ghEvent.WorkflowJob.GetID(), listOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to get job annotations: %w", err)
		}
		allAnnotations = append(allAnnotations, annotations...)
		if response.NextPage == 0 {
			break
		}
		listOpts.Page = response.NextPage
	}
	return allAnnotations, nil
}

func (rec *githubactionsannotationsreceiver) processAnnotations(ctx context.Context, batch []*github.CheckRunAnnotation, repository Repository, run Run, withWorkflowInfoFields func(fields ...zap.Field) []zap.Field) (int, error) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceAttributes := resourceLogs.Resource().Attributes()
	serviceName := generateServiceName(rec.config, repository.FullName)
	resourceAttributes.PutStr("service.name", serviceName)
	resourceAttributes.PutStr("event.dataset", "github.annotations")
	scopeLogsSlice := resourceLogs.ScopeLogs()
	scopeLogs := scopeLogsSlice.AppendEmpty()
	logRecords := scopeLogs.LogRecords()
	for _, line := range batch {
		logLine := parseAnnotationToLogLine(run.CompletedAt, line)
		logRecord := logRecords.AppendEmpty()
		if err := attachData(&logRecord, repository, run, logLine); err != nil {
			return 0, fmt.Errorf("failed to attach data to log record: %w", err)
		}
	}
	if logs.LogRecordCount() == 0 {
		return 0, nil
	}
	rec.obsrecv.StartLogsOp(ctx)
	err := rec.consumeLogsWithRetry(ctx, withWorkflowInfoFields, logs)
	if err != nil {
		rec.logger.Error("Failed to consume annotations", withWorkflowInfoFields(zap.Error(err), zap.Int("dropped_items", logs.LogRecordCount()))...)
	} else {
		rec.logger.Info("Successfully consumed annotations", withWorkflowInfoFields(zap.Int("log_record_count", logs.LogRecordCount()))...)
	}
	rec.obsrecv.EndLogsOp(ctx, "github-actions", logs.LogRecordCount(), err)
	return logs.LogRecordCount(), err
}

func (rec *githubactionsannotationsreceiver) consumeLogsWithRetry(ctx context.Context, withWorkflowInfoFields func(fields ...zap.Field) []zap.Field, logs plog.Logs) error {
	expBackoff := backoff.ExponentialBackOff{
		MaxElapsedTime:      rec.config.Retry.MaxElapsedTime,
		InitialInterval:     rec.config.Retry.InitialInterval,
		MaxInterval:         rec.config.Retry.MaxInterval,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	expBackoff.Reset()
	retryableErr := consumererror.Logs{}
	for {
		err := rec.consumer.ConsumeLogs(ctx, logs)
		if err == nil {
			return nil
		}
		if consumererror.IsPermanent(err) {
			rec.logger.Error(
				"Consuming logs failed. The error is not retryable. Dropping data.",
				withWorkflowInfoFields(
					zap.Error(err),
					zap.Int("dropped_items", logs.LogRecordCount()),
				)...,
			)
			return err
		}
		if errors.As(err, &retryableErr) {
			logs = retryableErr.Data()
		}
		backoffDelay := expBackoff.NextBackOff()
		if backoffDelay == backoff.Stop {
			rec.logger.Error(
				"Max elapsed time expired. Dropping data.",
				withWorkflowInfoFields(
					zap.Error(err),
					zap.Int("dropped_items", logs.LogRecordCount()),
				)...,
			)
			return err
		}
		rec.logger.Debug(
			"Consuming logs failed. Will retry the request after interval.",
			withWorkflowInfoFields(
				zap.Error(err),
				zap.String("interval", backoffDelay.String()),
				zap.Int("logs_count", logs.LogRecordCount()),
			)...,
		)
		select {
		case <-ctx.Done():
			return fmt.Errorf("context is cancelled or timed out %w", err)
		case <-time.After(backoffDelay):
		}
	}
}
