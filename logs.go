package opentelemetrygithubactionsannotationsreceiver

import (
	"time"

	"github.com/google/go-github/v66/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func attachData(logRecord *plog.LogRecord, repository Repository, run Run, logLine LogLine) error {
	logRecord.SetSeverityNumber(plog.SeverityNumber(logLine.SeverityNumber))
	logRecord.SetSeverityText(logLine.SeverityText)
	if err := attachTraceId(logRecord, run); err != nil {
		return err
	}
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(logLine.Timestamp))
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.Body().SetStr(logLine.Body)
	attachRepositoryAttributes(logRecord, repository)
	attachRunAttributes(logRecord, run)
	return nil
}

// parseAnnotationToLogLine parses an annotation from the GitHub Actions log file
func parseAnnotationToLogLine(completedAt time.Time, line *github.CheckRunAnnotation) LogLine {
	var severityNumber = 0 // Unspecified
	return LogLine{
		Body:           line.GetMessage(),
		Timestamp:      completedAt,
		SeverityNumber: severityNumber,
		SeverityText:   line.GetAnnotationLevel(),
	}
}

func attachTraceId(logRecord *plog.LogRecord, run Run) error {
	traceId, err := generateTraceID(run.ID, int(run.RunAttempt))
	if err != nil {
		return err
	}
	logRecord.SetTraceID(traceId)
	return nil
}

func attachRepositoryAttributes(logRecord *plog.LogRecord, repository Repository) {
	logRecord.Attributes().PutStr("github.repository", repository.FullName)
}

func attachRunAttributes(logRecord *plog.LogRecord, run Run) {
	logRecord.Attributes().PutInt("github.workflow_run.id", run.ID)
	logRecord.Attributes().PutInt("github.workflow_run.run_attempt", run.RunAttempt)
	logRecord.Attributes().PutStr("github.workflow_run.conclusion", run.Conclusion)
	logRecord.Attributes().PutStr("github.workflow_run.status", run.Status)
	logRecord.Attributes().PutStr("github.workflow_run.run_started_at", pcommon.NewTimestampFromTime(run.RunStartedAt).String())
	logRecord.Attributes().PutStr("github.workflow_run.created_at", pcommon.NewTimestampFromTime(run.CreatedAt).String())
	logRecord.Attributes().PutStr("github.workflow_run.completed_at", pcommon.NewTimestampFromTime(run.CompletedAt).String())
	logRecord.Attributes().PutStr("github.workflow_run.head_branch", run.HeadBranch)
	logRecord.Attributes().PutStr("github.workflow_run.html_url", run.URL)
}
