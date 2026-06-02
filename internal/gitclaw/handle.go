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
	if IsResearchReportRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/research",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderResearchReport(ev, cfg, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post research report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSoulRehearsalIssueRequest(ev, cfg) {
		rehearsalClient, ok := github.(SoulRehearsalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "soul", fmt.Errorf("github client cannot create soul rehearsal issues"))
		}
		req, err := BuildSoulRehearsalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "soul", fmt.Errorf("build soul rehearsal issue: %w", err))
		}
		result, err := RunSoulRehearsalIssue(ctx, cfg, rehearsalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "soul", fmt.Errorf("run soul rehearsal issue: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/soul",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSoulRehearsalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post soul rehearsal issue comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSoulProposalIssueRequest(ev, cfg) {
		proposalClient, ok := github.(SoulProposalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "soul", fmt.Errorf("github client cannot create soul proposal issues"))
		}
		req, err := BuildSoulProposalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "soul", fmt.Errorf("build soul proposal issue: %w", err))
		}
		result, err := RunSoulProposalIssue(ctx, proposalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "soul", fmt.Errorf("run soul proposal issue: %w", err))
		}
		if len(req.NotifyRoutes) > 0 {
			channelClient, ok := github.(ChannelSendGitHubClient)
			if !ok {
				return failStartedTurn(ctx, cfg, github, ev, status, "soul", fmt.Errorf("github client cannot notify channel routes for soul proposals"))
			}
			notification, err := RunSoulProposalChannelNotification(ctx, cfg, channelClient, req, result)
			if err != nil {
				return failStartedTurn(ctx, cfg, github, ev, status, "soul", fmt.Errorf("notify channel routes for soul proposal: %w", err))
			}
			result.ChannelNotification = notification
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/soul",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSoulProposalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post soul proposal issue comment: %w", err))
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
	if IsMemoryRehearsalIssueRequest(ev, cfg) {
		rehearsalClient, ok := github.(MemoryRehearsalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "memory", fmt.Errorf("github client cannot create memory rehearsal issues"))
		}
		req, err := BuildMemoryRehearsalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "memory", fmt.Errorf("build memory rehearsal issue: %w", err))
		}
		result, err := RunMemoryRehearsalIssue(ctx, cfg, rehearsalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "memory", fmt.Errorf("run memory rehearsal issue: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/memory",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderMemoryRehearsalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post memory rehearsal issue comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsMemoryProposalIssueRequest(ev, cfg) {
		proposalClient, ok := github.(MemoryProposalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "memory", fmt.Errorf("github client cannot create memory proposal issues"))
		}
		req, err := BuildMemoryProposalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "memory", fmt.Errorf("build memory proposal issue: %w", err))
		}
		result, err := RunMemoryProposalIssue(ctx, proposalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "memory", fmt.Errorf("run memory proposal issue: %w", err))
		}
		if len(req.NotifyRoutes) > 0 {
			channelClient, ok := github.(ChannelSendGitHubClient)
			if !ok {
				return failStartedTurn(ctx, cfg, github, ev, status, "memory", fmt.Errorf("github client cannot queue memory proposal channel notifications"))
			}
			notification, err := RunMemoryProposalChannelNotification(ctx, cfg, channelClient, req, result)
			if err != nil {
				return failStartedTurn(ctx, cfg, github, ev, status, "memory", fmt.Errorf("run memory proposal channel notification: %w", err))
			}
			result.ChannelNotification = notification
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/memory",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderMemoryProposalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post memory proposal issue comment: %w", err))
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
	if IsPromptPackRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/prompt",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderPromptPackReport(ev, cfg, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post prompt pack report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsPromptCacheRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/prompt",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderPromptCacheReport(ev, cfg, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post prompt cache report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsPromptCompressionRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/prompt",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderPromptCompressionReport(ev, cfg, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post prompt compression report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsPromptContextRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/prompt",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderPromptContextReport(ev, cfg, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post prompt context report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsPromptRiskRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/prompt",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderPromptRiskReport(ev, cfg, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post prompt risk report comment: %w", err))
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
	if IsToolRunRequestIssueRequest(ev, cfg) {
		requestClient, ok := github.(ToolRunRequestIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("github client cannot create tool run request issues"))
		}
		req, err := BuildToolRunRequestIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("build tool run request issue: %w", err))
		}
		result, err := RunToolRunRequestIssue(ctx, requestClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("run tool run request issue: %w", err))
		}
		if len(req.NotifyRoutes) > 0 {
			channelClient, ok := github.(ChannelSendGitHubClient)
			if !ok {
				return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("github client cannot notify channel routes for tool run requests"))
			}
			notification, err := RunToolRunRequestChannelNotification(ctx, cfg, channelClient, req, result)
			if err != nil {
				return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("notify channel routes for tool run request: %w", err))
			}
			result.ChannelNotification = notification
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/tools",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderToolRunRequestIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post tool run request issue comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsToolRehearsalIssueRequest(ev, cfg) {
		rehearsalClient, ok := github.(ToolRehearsalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("github client cannot create tool rehearsal issues"))
		}
		req, err := BuildToolRehearsalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("build tool rehearsal issue: %w", err))
		}
		result, err := RunToolRehearsalIssue(ctx, cfg, rehearsalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("run tool rehearsal issue: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/tools",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderToolRehearsalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post tool rehearsal issue comment: %w", err))
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
		var report string
		if IsRunHistoryRequest(ev, cfg) {
			report = RenderRunHistoryReport(ev, cfg, comments)
		} else {
			report = RenderRunReport(ev, cfg, decision, comments, transcript, repoContext, writeRequested)
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/runs",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, report)
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
	if IsSecurityReportRequest(ev, cfg) {
		reportBody, err := RenderSecurityReport(ev, cfg, repoContext, comments)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "security", fmt.Errorf("build security audit: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/security",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, reportBody)
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post security audit comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSecretsReportRequest(ev, cfg) {
		report, err := BuildSecretAuditReport(cfg.Workdir)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "secrets", fmt.Errorf("build secrets audit: %w", err))
		}
		reportBody := RenderSecretsReport(ev, report)
		if IsSecretsRiskRequest(ev, cfg) {
			reportBody = RenderSecretsRiskReport(ev, report)
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/secrets",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, reportBody)
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post secrets report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsCheckpointRehearsalIssueRequest(ev, cfg) {
		rehearsalClient, ok := github.(CheckpointRehearsalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "checkpoints", fmt.Errorf("github client cannot create checkpoint rehearsal issues"))
		}
		req, err := BuildCheckpointRehearsalIssueRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "checkpoints", fmt.Errorf("build checkpoint rehearsal issue: %w", err))
		}
		result, err := RunCheckpointRehearsalIssue(ctx, cfg, rehearsalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "checkpoints", fmt.Errorf("run checkpoint rehearsal issue: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/checkpoints",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderCheckpointRehearsalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post checkpoint rehearsal issue comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsCheckpointReportRequest(ev, cfg) {
		report := BuildCheckpointReport(cfg.Workdir)
		reportBody := RenderCheckpointReportWithConfig(ev, cfg, report)
		if isCheckpointRiskRequest(ev, cfg) {
			reportBody = RenderCheckpointRiskReport(ev, report)
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/checkpoints",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, reportBody)
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
		}, RenderApprovalReportWithComments(ev, cfg, decision, comments, transcript, writeRequested))
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
	if IsHeartbeatRiskRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/heartbeat",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderHeartbeatRiskReport(ev, cfg, comments))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post heartbeat risk report comment: %w", err))
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
	if IsSessionHandoffIssueRequest(ev, cfg) {
		handoffClient, ok := github.(SessionHandoffIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "session", fmt.Errorf("github client cannot create session handoff issues"))
		}
		req, err := BuildSessionHandoffIssueRequest(ev, cfg, comments, transcript)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "session", fmt.Errorf("build session handoff issue: %w", err))
		}
		result, err := RunSessionHandoffIssue(ctx, cfg, handoffClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "session", fmt.Errorf("run session handoff issue: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/session",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSessionHandoffIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post session handoff issue comment: %w", err))
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
	if IsBackupRehearsalIssueRequest(ev, cfg) {
		rehearsalClient, ok := github.(BackupRehearsalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "backup", fmt.Errorf("github client cannot create backup rehearsal issues"))
		}
		req, err := BuildBackupRehearsalIssueRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "backup", fmt.Errorf("build backup rehearsal issue: %w", err))
		}
		result, err := RunBackupRehearsalIssue(ctx, cfg, rehearsalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "backup", fmt.Errorf("run backup rehearsal issue: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/backup",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderBackupRehearsalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post backup rehearsal issue comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsBackupRestoreRequestIssueRequest(ev, cfg) {
		restoreClient, ok := github.(BackupRestoreRequestIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "backup", fmt.Errorf("github client cannot create backup restore request issues"))
		}
		req, err := BuildBackupRestoreRequestIssueRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "backup", fmt.Errorf("build backup restore request issue: %w", err))
		}
		result, err := RunBackupRestoreRequestIssue(ctx, cfg, restoreClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "backup", fmt.Errorf("run backup restore request issue: %w", err))
		}
		if len(req.NotifyRoutes) > 0 {
			channelClient, ok := github.(ChannelSendGitHubClient)
			if !ok {
				return failStartedTurn(ctx, cfg, github, ev, status, "backup", fmt.Errorf("github client cannot notify channel routes for backup restore requests"))
			}
			notification, err := RunBackupRestoreRequestChannelNotification(ctx, cfg, channelClient, req, result)
			if err != nil {
				return failStartedTurn(ctx, cfg, github, ev, status, "backup", fmt.Errorf("notify channel routes for backup restore request: %w", err))
			}
			result.ChannelNotification = notification
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/backup",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderBackupRestoreRequestIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post backup restore request issue comment: %w", err))
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
	if IsModelCostRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/models",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderModelCostReport(ev, cfg, comments, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post model cost report comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsModelUsageRequest(ev, cfg) {
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/models",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderModelUsageReport(ev, cfg, comments, transcript, repoContext))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post model usage report comment: %w", err))
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
	if IsChannelDoneActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelDoneGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot complete channel artifacts"))
		}
		req, err := BuildChannelDoneActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel done action: %w", err))
		}
		result, err := RunChannelDone(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel done action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelDoneActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel done action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSessionHandoffActionRequest(ev, cfg) {
		channelSessionClient, ok := github.(interface {
			SessionHandoffIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel session handoffs"))
		}
		req, err := BuildChannelSessionHandoffActionRequest(ev, cfg, comments, transcript)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel session handoff action: %w", err))
		}
		result, err := RunChannelSessionHandoff(ctx, cfg, channelSessionClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel session handoff action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSessionHandoffActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel session handoff action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelToolRunRequestActionRequest(ev, cfg) {
		channelToolClient, ok := github.(interface {
			ToolRunRequestIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel tool run requests"))
		}
		req, err := BuildChannelToolRunRequestActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel tool run request action: %w", err))
		}
		result, err := RunChannelToolRunRequest(ctx, cfg, channelToolClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel tool run request action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolRunRequestActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel tool run request action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelToolRehearsalActionRequest(ev, cfg) {
		channelToolClient, ok := github.(interface {
			ToolRehearsalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel tool rehearsals"))
		}
		req, err := BuildChannelToolRehearsalActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel tool rehearsal action: %w", err))
		}
		result, err := RunChannelToolRehearsal(ctx, cfg, channelToolClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel tool rehearsal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolRehearsalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel tool rehearsal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSkillRehearsalActionRequest(ev, cfg) {
		channelSkillClient, ok := github.(interface {
			SkillRehearsalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel skill rehearsals"))
		}
		req, err := BuildChannelSkillRehearsalActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel skill rehearsal action: %w", err))
		}
		result, err := RunChannelSkillRehearsal(ctx, cfg, channelSkillClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel skill rehearsal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSkillRehearsalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel skill rehearsal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSoulRehearsalActionRequest(ev, cfg) {
		channelSoulClient, ok := github.(interface {
			SoulRehearsalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel soul rehearsals"))
		}
		req, err := BuildChannelSoulRehearsalActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel soul rehearsal action: %w", err))
		}
		result, err := RunChannelSoulRehearsal(ctx, cfg, channelSoulClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel soul rehearsal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSoulRehearsalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel soul rehearsal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelMemoryRehearsalActionRequest(ev, cfg) {
		channelMemoryClient, ok := github.(interface {
			MemoryRehearsalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel memory rehearsals"))
		}
		req, err := BuildChannelMemoryRehearsalActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel memory rehearsal action: %w", err))
		}
		result, err := RunChannelMemoryRehearsal(ctx, cfg, channelMemoryClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel memory rehearsal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelMemoryRehearsalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel memory rehearsal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelBackupRehearsalActionRequest(ev, cfg) {
		channelBackupClient, ok := github.(interface {
			BackupRehearsalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel backup rehearsals"))
		}
		req, err := BuildChannelBackupRehearsalActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel backup rehearsal action: %w", err))
		}
		result, err := RunChannelBackupRehearsal(ctx, cfg, channelBackupClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel backup rehearsal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBackupRehearsalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel backup rehearsal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelTaskActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel tasks"))
		}
		req, err := BuildChannelTaskActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel task action: %w", err))
		}
		result, err := RunChannelTask(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel task action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelTaskActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel task action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelClipActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot save channel clips"))
		}
		req, err := BuildChannelClipActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel clip action: %w", err))
		}
		result, err := RunChannelClip(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel clip action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelClipActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel clip action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelAttachmentActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel attachments"))
		}
		req, err := BuildChannelAttachmentActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel attachment action: %w", err))
		}
		result, err := RunChannelAttachment(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel attachment action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelAttachmentActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel attachment action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelDeliverableActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel deliverables"))
		}
		req, err := BuildChannelDeliverableActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel deliverable action: %w", err))
		}
		result, err := RunChannelDeliverable(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel deliverable action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelDeliverableActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel deliverable action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelDecisionActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel decisions"))
		}
		req, err := BuildChannelDecisionActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel decision action: %w", err))
		}
		result, err := RunChannelDecision(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel decision action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelDecisionActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel decision action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelDigestActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel digests"))
		}
		req, err := BuildChannelDigestActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel digest action: %w", err))
		}
		result, err := RunChannelDigest(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel digest action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelDigestActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel digest action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelReminderActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel reminders"))
		}
		req, err := BuildChannelReminderActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel reminder action: %w", err))
		}
		result, err := RunChannelReminder(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel reminder action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelReminderActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel reminder action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelPollActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel polls"))
		}
		req, err := BuildChannelPollActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel poll action: %w", err))
		}
		result, err := RunChannelPoll(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel poll action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelPollActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel poll action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelRollcallActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel rollcalls"))
		}
		req, err := BuildChannelRollcallActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel rollcall action: %w", err))
		}
		result, err := RunChannelRollcall(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel rollcall action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelRollcallActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel rollcall action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel status updates"))
		}
		req, err := BuildChannelStatusActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel status action: %w", err))
		}
		result, err := RunChannelStatus(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelStatusActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel status action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelEditActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel edits"))
		}
		req, err := BuildChannelEditActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel edit action: %w", err))
		}
		result, err := RunChannelEdit(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel edit action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelEditActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel edit action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelReactionActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel reactions"))
		}
		req, err := BuildChannelReactionActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel reaction action: %w", err))
		}
		result, err := RunChannelReaction(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel reaction action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelReactionActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel reaction action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelRoomActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel rooms"))
		}
		req, err := BuildChannelRoomActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel room action: %w", err))
		}
		result, err := RunChannelRoom(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel room action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelRoomActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel room action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelHuddleActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel huddles"))
		}
		req, err := BuildChannelHuddleActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel huddle action: %w", err))
		}
		result, err := RunChannelHuddle(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel huddle action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelHuddleActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel huddle action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelInviteActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot invite channel routes"))
		}
		req, err := BuildChannelInviteActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel invite action: %w", err))
		}
		result, err := RunChannelBroadcast(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel invite action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelInviteActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel invite action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelBroadcastActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot broadcast channel messages"))
		}
		req, err := BuildChannelBroadcastActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel broadcast action: %w", err))
		}
		result, err := RunChannelBroadcast(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel broadcast action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBroadcastActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel broadcast action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSendActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot send channel messages"))
		}
		req, err := BuildChannelSendActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel send action: %w", err))
		}
		result, err := RunChannelSend(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel send action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSendActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel send action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelProbeActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot probe channel routes"))
		}
		req, err := BuildChannelProbeActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel probe action: %w", err))
		}
		result, err := RunChannelProbeAction(ctx, cfg, channelClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel probe action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelProbeActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel probe action comment: %w", err))
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
	if IsSkillBundleRehearsalIssueRequest(ev, cfg) {
		rehearsalClient, ok := github.(SkillBundleRehearsalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("github client cannot create skill bundle rehearsal issues"))
		}
		req, err := BuildSkillBundleRehearsalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("build skill bundle rehearsal issue: %w", err))
		}
		result, err := RunSkillBundleRehearsalIssue(ctx, cfg, rehearsalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("run skill bundle rehearsal issue: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/skills",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSkillBundleRehearsalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post skill bundle rehearsal issue comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSkillRehearsalIssueRequest(ev, cfg) {
		rehearsalClient, ok := github.(SkillRehearsalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("github client cannot create skill rehearsal issues"))
		}
		req, err := BuildSkillRehearsalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("build skill rehearsal issue: %w", err))
		}
		result, err := RunSkillRehearsalIssue(ctx, cfg, rehearsalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("run skill rehearsal issue: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/skills",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSkillRehearsalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post skill rehearsal issue comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSkillSourceProposalIssueRequest(ev, cfg) {
		proposalClient, ok := github.(SkillSourceProposalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("github client cannot create skill source proposal issues"))
		}
		req, err := BuildSkillSourceProposalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("build skill source proposal issue: %w", err))
		}
		result, err := RunSkillSourceProposalIssue(ctx, cfg, proposalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("run skill source proposal issue: %w", err))
		}
		if len(req.NotifyRoutes) > 0 {
			channelClient, ok := github.(ChannelSendGitHubClient)
			if !ok {
				return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("github client cannot queue skill source proposal channel notifications"))
			}
			notification, err := RunSkillSourceProposalChannelNotification(ctx, cfg, channelClient, req, result)
			if err != nil {
				return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("run skill source proposal channel notification: %w", err))
			}
			result.ChannelNotification = notification
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/skills",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSkillSourceProposalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post skill source proposal issue comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsSkillProposalIssueRequest(ev, cfg) {
		proposalClient, ok := github.(SkillProposalIssueGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("github client cannot create skill proposal issues"))
		}
		req, err := BuildSkillProposalIssueRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("build skill proposal issue: %w", err))
		}
		result, err := RunSkillProposalIssue(ctx, proposalClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("run skill proposal issue: %w", err))
		}
		if len(req.NotifyRoutes) > 0 {
			channelClient, ok := github.(ChannelSendGitHubClient)
			if !ok {
				return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("github client cannot queue skill proposal channel notifications"))
			}
			notification, err := RunSkillProposalChannelNotification(ctx, cfg, channelClient, req, result)
			if err != nil {
				return failStartedTurn(ctx, cfg, github, ev, status, "skill", fmt.Errorf("run skill proposal channel notification: %w", err))
			}
			result.ChannelNotification = notification
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/skills",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderSkillProposalIssueActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post skill proposal issue comment: %w", err))
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
		Usage:          selectedLLMUsage(llm),
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

type selectedUsageReporter interface {
	LastUsage() LLMUsage
}

func selectedLLMModel(cfg Config, llm LLMClient) string {
	if reporter, ok := llm.(selectedModelReporter); ok {
		if model := strings.TrimSpace(reporter.SelectedModel()); model != "" {
			return model
		}
	}
	return cfg.Model
}

func selectedLLMUsage(llm LLMClient) LLMUsage {
	if reporter, ok := llm.(selectedUsageReporter); ok {
		return reporter.LastUsage()
	}
	return LLMUsage{}
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
	case "channel":
		return "channel action could not be completed"
	case "session":
		return "session action could not be completed"
	case "memory":
		return "memory action could not be completed"
	case "skill":
		return "skill action could not be completed"
	case "soul":
		return "soul action could not be completed"
	case "tool":
		return "tool action could not be completed"
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
