package opentelemetrygithubactionsannotationsreceiver

import (
	"time"

	"github.com/google/go-github/v66/github"
)

type LogLine struct {
	Body           string
	Timestamp      time.Time
	SeverityNumber int
	SeverityText   string
}

type Repository struct {
	FullName string
	Org      string
	Name     string
}

type Run struct {
	ID           int64
	RunAttempt   int64     `json:"run_attempt"`
	RunStartedAt time.Time `json:"run_started_at"`
	URL          string    `json:"html_url"`
	Status       string
	Conclusion   string
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at"`
	HeadBranch   string
}

func mapRun(run *github.WorkflowJob) Run {
	return Run{
		ID:           *run.RunID,
		RunAttempt:   int64(*run.RunAttempt),
		URL:          *run.RunURL,
		Status:       *run.Status,
		Conclusion:   *run.Conclusion,
		RunStartedAt: run.StartedAt.Time,
		CreatedAt:    run.CreatedAt.Time,
		CompletedAt:  run.CompletedAt.Time,
		HeadBranch:   run.GetHeadBranch(),
	}
}

func mapRepository(repo *github.Repository) Repository {
	return Repository{
		FullName: repo.GetFullName(),
		Org:      repo.GetOwner().GetLogin(),
		Name:     repo.GetName(),
	}
}
