package opentelemetrygithubactionsannotationsreceiver

import (
	"testing"
	"time"

	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/require"
)

func TestMapRun(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	runID := int64(123)
	runAttempt := int64(2)
	runURL := "https://github.com/org/repo/actions/runs/123"
	status := "completed"
	conclusion := "success"
	headBranch := "main"

	ghRun := &github.WorkflowJob{
		RunID:       &runID,
		RunAttempt:  &runAttempt,
		RunURL:      &runURL,
		Status:      &status,
		Conclusion:  &conclusion,
		HeadBranch:  &headBranch,
		StartedAt:   &github.Timestamp{Time: now.Add(-time.Minute)},
		CreatedAt:   &github.Timestamp{Time: now.Add(-2 * time.Minute)},
		CompletedAt: &github.Timestamp{Time: now},
	}

	run := mapRun(ghRun)

	require.Equal(t, runID, run.ID)
	require.Equal(t, runAttempt, run.RunAttempt)
	require.Equal(t, runURL, run.URL)
	require.Equal(t, status, run.Status)
	require.Equal(t, conclusion, run.Conclusion)
	require.Equal(t, headBranch, run.HeadBranch)
	require.Equal(t, now.Add(-time.Minute), run.RunStartedAt)
	require.Equal(t, now.Add(-2*time.Minute), run.CreatedAt)
	require.Equal(t, now, run.CompletedAt)
}

func TestMapRepository(t *testing.T) {
	repo := &github.Repository{
		Name:     github.String("repo"),
		FullName: github.String("org/repo"),
		Owner: &github.User{
			Login: github.String("org"),
		},
	}

	mapped := mapRepository(repo)

	require.Equal(t, "org/repo", mapped.FullName)
	require.Equal(t, "org", mapped.Org)
	require.Equal(t, "repo", mapped.Name)
}
