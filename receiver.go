package opentelemetrygithubactionsannotationsreceiver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-github/v66/github"
	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"go.uber.org/zap"
)

func newLogsReceiver(cfg *Config, params receiver.CreateSettings, consumer consumer.Logs) (*githubactionsannotationsreceiver, error) {
	_, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              "http",
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}
	ghClient := github.NewClient(nil).WithAuthToken(string(cfg.Token))
	rateLimit, _, err := ghClient.RateLimit.Get(context.Background())
	if err != nil {
		return nil, err
	}
	params.Logger.Info("GitHub API rate limit", zap.Int("limit", rateLimit.GetCore().Limit), zap.Int("remaining", rateLimit.GetCore().Remaining), zap.Time("reset", rateLimit.GetCore().Reset.Time))
	return &githubactionsannotationsreceiver{
		config:   cfg,
		settings: params,
		logger:   params.Logger,
		ghClient: ghClient,
	}, nil
}

type githubactionsannotationsreceiver struct {
	config   *Config
	server   *http.Server
	settings receiver.CreateSettings
	logger   *zap.Logger
	ghClient *github.Client
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
		rec.handleWorkflowJobEvent(event, w, r, nil)
	default:
		{
			// TODO: avoid verbosity while running this
			//rec.logger.Debug("Skipping the request because it is not a workflow_job event", zap.Any("event", event))
			w.WriteHeader(http.StatusOK)
		}
	}
}

func (rec *githubactionsannotationsreceiver) handleWorkflowJobEvent(event *github.WorkflowJobEvent, w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
	annotations, err := getAnnotations(context.Background(), withWorkflowInfoFields, event, rec.ghClient)
	if err != nil {
		rec.logger.Error("Failed to get job annotations", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, annotation := range annotations {
		// TODO: process in batches
		rec.logger.Info("------- " + *annotation.Message)
	}

	if len(annotations) == 0 {
		rec.logger.Debug("No annotations found")
		w.WriteHeader(http.StatusOK)
		return
	}
	return
}

func getAnnotations(ctx context.Context, withWorkflowInfoFields func(fields ...zap.Field) []zap.Field, ghEvent *github.WorkflowJobEvent, ghClient *github.Client) ([]*github.CheckRunAnnotation, error) {
	listOpts := &github.ListOptions{
		PerPage: 100,
	}
	var allAnnotations []*github.CheckRunAnnotation
	for {
		artifacts, response, err := ghClient.Checks.ListCheckRunAnnotations(ctx, ghEvent.GetRepo().GetOwner().GetLogin(), ghEvent.GetRepo().GetName(), ghEvent.WorkflowJob.GetID(), listOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to get job annotations: %w", err)
		}
		allAnnotations = append(allAnnotations, artifacts...)
		if response.NextPage == 0 {
			break
		}
		listOpts.Page = response.NextPage
	}
	return allAnnotations, nil
}
