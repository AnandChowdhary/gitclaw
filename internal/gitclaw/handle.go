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

	transcript := BuildTranscript(ev, comments)
	writeRequested := DetectWriteRequest(transcript)
	if writeRequested {
		status.SetWriteRequested()
	}
	repoContext, err := LoadRepoContext(cfg.Workdir, transcript)
	if err != nil {
		return failStartedTurn(ctx, cfg, github, ev, status, "context", fmt.Errorf("load repo context: %w", err))
	}
	if writeRequested {
		repoContext.ToolOutputs = append(repoContext.ToolOutputs, WriteRequestPolicyOutput())
	}
	if IsContextReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/context",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderContextReport(ev, cfg, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post context report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSoulReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/soul",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSoulReport(ev, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post soul report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsToolsReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/tools",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderToolsReport(ev, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post tools report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsPolicyReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/policy",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderPolicyReport(ev, cfg, decision, transcript, repoContext, writeRequested))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post policy report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSessionReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/session",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSessionReport(ev, comments, transcript))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post session report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsBackupReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/backup",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderBackupReport(ev, comments, transcript))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post backup report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsProactiveReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/proactive",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderProactiveReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post proactive report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsModelReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/models",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderModelReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post model report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSkillsReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/skills",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSkillsReport(ev, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post skills report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	response, err := llm.Complete(ctx, LLMRequest{
		Event:      ev,
		Transcript: transcript,
		Context:    repoContext,
		Config:     cfg,
	})
	if err != nil {
		return failStartedTurn(ctx, cfg, github, ev, status, "model", fmt.Errorf("complete LLM response: %w", err))
	}

	body := RenderAssistantComment(Marker{
		RunID:          envFirst("GITHUB_RUN_ID", "local"),
		EventID:        eventID(ev),
		Model:          cfg.Model,
		IdempotencyKey: key,
		RunURL:         actionRunURL(ev),
	}, response)
	if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
		return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post issue comment: %w", err))
	}
	status.SetDone()
	return nil
}

func failStartedTurn(ctx context.Context, cfg Config, github GitHubClient, ev Event, status issueStatusUpdater, phase string, cause error) error {
	status.SetError()
	diagnostic := safeFailureDiagnostic(phase, cause)
	_, _ = github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, RenderErrorComment(ErrorMarker{
		RunID:   envFirst("GITHUB_RUN_ID", "local"),
		EventID: eventID(ev),
		Phase:   phase,
		RunURL:  actionRunURL(ev),
	}, diagnostic))
	return cause
}

func safeFailureDiagnostic(phase string, cause error) string {
	switch phase {
	case "context":
		return "repository context could not be loaded"
	case "model":
		return "model provider request failed"
	case "comment":
		return "assistant comment could not be posted"
	default:
		_ = cause
		return "assistant turn failed"
	}
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

func (u issueStatusUpdater) SetWriteRequested() {
	u.add(u.cfg.WriteRequestedLabel)
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
	if ev.Kind == EventWorkflowDispatch {
		if ev.DispatchID != "" {
			return fmt.Sprintf("dispatch-%s", ev.DispatchID)
		}
		return fmt.Sprintf("dispatch-issue-%d", ev.Issue.Number)
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
