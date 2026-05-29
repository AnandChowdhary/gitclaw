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
	return nil
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
