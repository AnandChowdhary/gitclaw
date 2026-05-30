package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	ev = withWorkflowDispatchActiveText(ev, comments)

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
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, transcript, cfg)
	if err != nil {
		return failStartedTurn(ctx, cfg, github, ev, status, "context", fmt.Errorf("load repo context: %w", err))
	}
	if toolEnabled, _, _ := toolEnabledByConfig("gitclaw.policy", cfg); writeRequested && toolEnabled {
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
	if IsDiffReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/diffs",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderDiffReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post diffs report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsWorkspaceReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/workspace",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderWorkspaceReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post workspace report comment: %w", err))
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
		}, RenderSoulReport(ev, cfg, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post soul report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsMemoryReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/memory",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderMemoryReport(ev, cfg, repoContext, transcript))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post memory report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsPromptReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/prompt",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderPromptReport(ev, cfg, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post prompt report comment: %w", err))
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
		}, RenderToolsReport(ev, cfg, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post tools report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsProfileReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/profile",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderProfileReport(ev, cfg, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post profile report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsMigrationReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/migration",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderMigrationReport(ev, cfg, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post migration report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsRunReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/runs",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderRunReport(ev, cfg, decision, comments, transcript, repoContext, writeRequested))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post run ledger report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSandboxReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/sandbox",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSandboxReport(ev, cfg, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post sandbox report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSecretsReportRequest(ev, cfg) {
		report, err := BuildSecretAuditReport(cfg.Workdir)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "secrets", fmt.Errorf("build secrets audit: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/secrets",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSecretsReport(ev, report))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post secrets report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsCheckpointReportRequest(ev, cfg) {
		report := BuildCheckpointReport(cfg.Workdir)
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/checkpoints",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderCheckpointReport(ev, report))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post checkpoints report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsApprovalReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/approvals",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderApprovalReport(ev, cfg, decision, transcript, writeRequested))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post approvals report comment: %w", err))
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
	if IsCommandReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/commands",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderCommandReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post command report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsHeartbeatReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/heartbeat",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderHeartbeatReport(ev, cfg, comments))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post heartbeat report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsHookReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/hooks",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderHookReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post hooks report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsPluginReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/plugins",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderPluginReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post plugins report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsTaskReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/tasks",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderTaskReport(ev, cfg, comments, transcript))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post tasks report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsAgentReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/agents",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderAgentReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post agents report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsNodeReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/nodes",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderNodeReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post nodes report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsArtifactReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/artifacts",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderArtifactReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post artifacts report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsStandingOrdersReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/orders",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderStandingOrdersReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post standing orders report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsDoctorReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/doctor",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderDoctorReport(ev, cfg, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post doctor report comment: %w", err))
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
		}, RenderSessionReport(ev, cfg, comments, transcript))
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
		}, RenderBackupReport(ev, cfg, comments, transcript))
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
	if IsConfigReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/config",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderConfigReport(ev, cfg))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post config report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelReport(ev, cfg, comments))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel report comment: %w", err))
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
		}, RenderSkillsReport(ev, cfg, repoContext))
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

	body := RenderAssistantComment(withPromptProvenance(Marker{
		RunID:          envFirst("GITHUB_RUN_ID", "local"),
		EventID:        eventID(ev),
		Model:          selectedLLMModel(cfg, llm),
		IdempotencyKey: key,
		RunURL:         actionRunURL(ev),
	}, repoContext), response)
	if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
		return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post issue comment: %w", err))
	}
	status.SetDone()
	return nil
}

type selectedModelReporter interface {
	SelectedModel() string
}

func selectedLLMModel(cfg Config, llm LLMClient) string {
	if reporter, ok := llm.(selectedModelReporter); ok {
		if model := strings.TrimSpace(reporter.SelectedModel()); model != "" {
			return model
		}
	}
	return cfg.Model
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
