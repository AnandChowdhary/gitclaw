package gitclaw

import (
	"context"
	"fmt"
	"os"
)

func Handle(ctx context.Context, ev Event, cfg Config, github GitHubClient, llm LLMClient) error {
	decision := Preflight(ev, cfg)
	if !decision.Allowed {
		return fmt.Errorf("%s: %s", decision.Code, decision.Reason)
	}

	comments, err := github.ListIssueComments(ctx, ev.Repo, ev.Issue.Number)
	if err != nil {
		return fmt.Errorf("list issue comments: %w", err)
	}

	key := IdempotencyKey(ev)
	for _, comment := range comments {
		if ContainsIdempotencyKey(comment.Body, key) {
			return nil
		}
	}

	status := newIssueStatusUpdater(ctx, cfg, github, ev.Repo, ev.Issue.Number)
	status.SetRunning()
	completed := false
	defer func() {
		if !completed {
			status.SetError()
		}
	}()

	transcript := BuildTranscript(ev, comments)
	repoContext, err := LoadRepoContext(cfg.Workdir, transcript)
	if err != nil {
		return fmt.Errorf("load repo context: %w", err)
	}
	response, err := llm.Complete(ctx, LLMRequest{
		Event:      ev,
		Transcript: transcript,
		Context:    repoContext,
		Config:     cfg,
	})
	if err != nil {
		return fmt.Errorf("complete LLM response: %w", err)
	}

	body := RenderAssistantComment(Marker{
		RunID:          envFirst("GITHUB_RUN_ID", "local"),
		EventID:        eventID(ev),
		Model:          cfg.Model,
		IdempotencyKey: key,
		RunURL:         actionRunURL(ev),
	}, response)
	if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
		return fmt.Errorf("post issue comment: %w", err)
	}
	completed = true
	status.SetDone()
	return nil
}

type issueStatusUpdater struct {
	ctx         context.Context
	cfg         Config
	github      GitHubClient
	repo        string
	issueNumber int
}

func newIssueStatusUpdater(ctx context.Context, cfg Config, github GitHubClient, repo string, issueNumber int) issueStatusUpdater {
	return issueStatusUpdater{
		ctx:         ctx,
		cfg:         cfg,
		github:      github,
		repo:        repo,
		issueNumber: issueNumber,
	}
}

func (u issueStatusUpdater) SetRunning() {
	u.remove(u.cfg.DoneLabel, u.cfg.ErrorLabel)
	u.add(u.cfg.RunningLabel)
}

func (u issueStatusUpdater) SetDone() {
	u.remove(u.cfg.RunningLabel, u.cfg.ErrorLabel)
	u.add(u.cfg.DoneLabel)
}

func (u issueStatusUpdater) SetError() {
	u.remove(u.cfg.RunningLabel, u.cfg.DoneLabel)
	u.add(u.cfg.ErrorLabel)
}

func (u issueStatusUpdater) add(labels ...string) {
	labels = nonEmptyLabels(labels)
	if len(labels) == 0 {
		return
	}
	_ = u.github.AddIssueLabels(u.ctx, u.repo, u.issueNumber, labels)
}

func (u issueStatusUpdater) remove(labels ...string) {
	for _, label := range nonEmptyLabels(labels) {
		_ = u.github.RemoveIssueLabel(u.ctx, u.repo, u.issueNumber, label)
	}
}

func nonEmptyLabels(labels []string) []string {
	filtered := make([]string, 0, len(labels))
	for _, label := range labels {
		if label != "" {
			filtered = append(filtered, label)
		}
	}
	return filtered
}

func eventID(ev Event) string {
	if ev.Comment != nil {
		return fmt.Sprintf("comment-%d", ev.Comment.ID)
	}
	return fmt.Sprintf("issue-%d", ev.Issue.Number)
}

func actionRunURL(ev Event) string {
	runID := os.Getenv("GITHUB_RUN_ID")
	if runID == "" || ev.Repo == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/actions/runs/%s", ev.Repo, runID)
}

func envFirst(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
