package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type HeartbeatOptions struct {
	Repo  string
	Label string
	Slot  string
	Limit int
}

type HeartbeatResult struct {
	Scanned int
	Posted  int
	Skipped int
}

func RunHeartbeat(ctx context.Context, cfg Config, github HeartbeatGitHubClient, llm LLMClient, opts HeartbeatOptions) (HeartbeatResult, error) {
	if opts.Repo == "" {
		return HeartbeatResult{}, fmt.Errorf("missing heartbeat repo")
	}
	if err := validateRepoName(opts.Repo); err != nil {
		return HeartbeatResult{}, err
	}
	if opts.Label == "" {
		opts.Label = cfg.HeartbeatLabel
	}
	if opts.Slot == "" {
		opts.Slot = time.Now().UTC().Format("2006-01-02T15")
	}
	if opts.Limit <= 0 {
		opts.Limit = 3
	}
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{opts.Label}, opts.Limit)
	if err != nil {
		return HeartbeatResult{}, fmt.Errorf("list heartbeat issues: %w", err)
	}
	result := HeartbeatResult{Scanned: len(issues)}
	for _, issue := range issues {
		if issue.IsPullRequest || hasLabel(issue.Labels, cfg.DisabledLabel) {
			result.Skipped++
			continue
		}
		posted, err := runIssueHeartbeat(ctx, cfg, github, llm, opts.Repo, issue, opts.Slot)
		if err != nil {
			return result, err
		}
		if posted {
			result.Posted++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

func runIssueHeartbeat(ctx context.Context, cfg Config, github HeartbeatGitHubClient, llm LLMClient, repo string, issue Issue, slot string) (bool, error) {
	comments, err := github.ListIssueComments(ctx, repo, issue.Number)
	if err != nil {
		return false, fmt.Errorf("list issue comments for heartbeat: %w", err)
	}
	for _, comment := range comments {
		if HasHeartbeatMarker(comment.Body) && ContainsHeartbeatSlot(comment.Body, slot) {
			return false, nil
		}
	}

	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "heartbeat",
		Repo:      repo,
		Issue:     issue,
		Sender:    User{Login: "github-actions[bot]", Type: "Bot"},
	}
	transcript := BuildTranscript(ev, comments)
	transcript = append(transcript, TranscriptMessage{
		Role:    "user",
		Body:    heartbeatInstruction(slot),
		Actor:   "gitclaw-heartbeat",
		Trusted: true,
	})
	repoContext, err := LoadRepoContext(cfg.Workdir, transcript)
	if err != nil {
		return false, fmt.Errorf("load heartbeat context: %w", err)
	}
	response, err := llm.Complete(ctx, LLMRequest{
		Event:      ev,
		Transcript: transcript,
		Context:    repoContext,
		Config:     cfg,
	})
	if err != nil {
		return false, fmt.Errorf("complete heartbeat response: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(response), "HEARTBEAT_OK") {
		return false, nil
	}
	body := RenderHeartbeatComment(HeartbeatMarker{
		RunID:  envFirst("GITHUB_RUN_ID", "local"),
		Slot:   slot,
		RunURL: actionRunURL(ev),
	}, response)
	if _, err := github.PostIssueComment(ctx, repo, issue.Number, body); err != nil {
		return false, fmt.Errorf("post heartbeat comment: %w", err)
	}
	return true, nil
}

func heartbeatInstruction(slot string) string {
	return fmt.Sprintf(`GitClaw heartbeat slot %s.

Read .gitclaw/HEARTBEAT.md and the current issue thread as context.
If no action is needed, reply exactly HEARTBEAT_OK.
If the issue requests a heartbeat response, answer briefly and include any exact verification tokens requested by the issue or heartbeat context.
Do not claim to run commands or modify files.`, slot)
}
