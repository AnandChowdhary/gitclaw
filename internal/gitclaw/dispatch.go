package gitclaw

import (
	"context"
	"fmt"
)

func ResolveDispatchIssue(ctx context.Context, ev Event, github interface {
	GetIssue(context.Context, string, int) (Issue, error)
}) (Event, error) {
	if ev.Kind != EventWorkflowDispatch {
		return ev, nil
	}
	if ev.Issue.Number <= 0 {
		return Event{}, fmt.Errorf("workflow_dispatch missing issue number")
	}
	issue, err := github.GetIssue(ctx, ev.Repo, ev.Issue.Number)
	if err != nil {
		return Event{}, fmt.Errorf("resolve workflow_dispatch issue: %w", err)
	}
	ev.Issue = issue
	return ev, nil
}
