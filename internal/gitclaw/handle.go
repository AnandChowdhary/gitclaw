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
	if IsToolRunCancelRequest(ev, cfg) {
		cancelClient, ok := github.(ToolRunCancelGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("github client cannot cancel tool run request issues"))
		}
		req, err := BuildToolRunCancelRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("build tool run cancel action: %w", err))
		}
		result, err := RunToolRunCancel(ctx, cancelClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "tool", fmt.Errorf("run tool run cancel action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/tools",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderToolRunCancelActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post tool run cancel action comment: %w", err))
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
	if IsChannelSessionSearchActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel session search results"))
		}
		req, err := BuildChannelSessionSearchActionRequest(ev, cfg, transcript)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel session search action: %w", err))
		}
		result, err := RunChannelSessionSearch(ctx, cfg, channelClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel session search action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSessionSearchActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel session search action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelToolStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel tool status"))
		}
		req, err := BuildChannelToolStatusActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel tool status action: %w", err))
		}
		result, err := RunChannelToolStatus(ctx, cfg, channelClient, req.Options, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel tool status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolStatusActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel tool status action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelToolSearchActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel tool search results"))
		}
		req, err := BuildChannelToolSearchActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel tool search action: %w", err))
		}
		result, err := RunChannelToolSearch(ctx, cfg, channelClient, req, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel tool search action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolSearchActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel tool search action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelToolInfoActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel tool info"))
		}
		req, err := BuildChannelToolInfoActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel tool info action: %w", err))
		}
		result, err := RunChannelToolInfo(ctx, cfg, channelClient, req, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel tool info action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolInfoActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel tool info action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelProfileStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel profile status"))
		}
		req, err := BuildChannelProfileStatusActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel profile status action: %w", err))
		}
		result, err := RunChannelProfileStatus(ctx, cfg, channelClient, req.Options, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel profile status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelProfileStatusActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel profile status action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSoulStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel soul status"))
		}
		req, err := BuildChannelSoulStatusActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel soul status action: %w", err))
		}
		result, err := RunChannelSoulStatus(ctx, cfg, channelClient, req.Options, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel soul status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSoulStatusActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel soul status action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSoulInfoActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel soul info"))
		}
		req, err := BuildChannelSoulInfoActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel soul info action: %w", err))
		}
		result, err := RunChannelSoulInfo(ctx, cfg, channelClient, req, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel soul info action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSoulInfoActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel soul info action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSoulRiskActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel soul risk"))
		}
		req, err := BuildChannelSoulRiskActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel soul risk action: %w", err))
		}
		result, err := RunChannelSoulRisk(ctx, cfg, channelClient, req, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel soul risk action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSoulRiskActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel soul risk action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSoulSearchActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel soul search results"))
		}
		req, err := BuildChannelSoulSearchActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel soul search action: %w", err))
		}
		result, err := RunChannelSoulSearch(ctx, cfg, channelClient, req, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel soul search action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSoulSearchActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel soul search action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelMemoryStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel memory status"))
		}
		req, err := BuildChannelMemoryStatusActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel memory status action: %w", err))
		}
		result, err := RunChannelMemoryStatus(ctx, cfg, channelClient, req.Options, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel memory status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelMemoryStatusActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel memory status action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelMemorySearchActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel memory search results"))
		}
		req, err := BuildChannelMemorySearchActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel memory search action: %w", err))
		}
		result, err := RunChannelMemorySearch(ctx, cfg, channelClient, req, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel memory search action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelMemorySearchActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel memory search action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelMoodActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel mood updates"))
		}
		req, err := BuildChannelMoodActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel mood action: %w", err))
		}
		result, err := RunChannelMood(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel mood action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelMoodActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel mood action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelToolResultActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel tool results"))
		}
		req, err := BuildChannelToolResultActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel tool result action: %w", err))
		}
		result, err := RunChannelToolResult(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel tool result action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolResultActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel tool result action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelToolLessonActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel tool lessons"))
		}
		req, err := BuildChannelToolLessonActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel tool lesson action: %w", err))
		}
		result, err := RunChannelToolLesson(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel tool lesson action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolLessonActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel tool lesson action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelQuoteActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel quotes"))
		}
		req, err := BuildChannelQuoteActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel quote action: %w", err))
		}
		result, err := RunChannelQuote(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel quote action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelQuoteActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel quote action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelGlossaryActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel glossary entries"))
		}
		req, err := BuildChannelGlossaryActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel glossary action: %w", err))
		}
		result, err := RunChannelGlossary(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel glossary action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelGlossaryActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel glossary action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelFAQActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel FAQ entries"))
		}
		req, err := BuildChannelFAQActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel FAQ action: %w", err))
		}
		result, err := RunChannelFAQ(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel FAQ action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelFAQActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel FAQ action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSkillNoteActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel skill notes"))
		}
		req, err := BuildChannelSkillNoteActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel skill note action: %w", err))
		}
		result, err := RunChannelSkillNote(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel skill note action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSkillNoteActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel skill note action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSoulNoteActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel soul notes"))
		}
		req, err := BuildChannelSoulNoteActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel soul note action: %w", err))
		}
		result, err := RunChannelSoulNote(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel soul note action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSoulNoteActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel soul note action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelBackupNoteActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel backup notes"))
		}
		req, err := BuildChannelBackupNoteActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel backup note action: %w", err))
		}
		result, err := RunChannelBackupNote(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel backup note action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBackupNoteActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel backup note action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelMemoryNoteActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel memory notes"))
		}
		req, err := BuildChannelMemoryNoteActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel memory note action: %w", err))
		}
		result, err := RunChannelMemoryNote(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel memory note action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelMemoryNoteActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel memory note action comment: %w", err))
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
	if IsChannelToolApprovalPlanActionRequest(ev, cfg) {
		channelToolClient, ok := github.(interface {
			ToolApprovalPlanIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel tool approval plans"))
		}
		req, err := BuildChannelToolApprovalPlanActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel tool approval plan action: %w", err))
		}
		result, err := RunChannelToolApprovalPlan(ctx, cfg, repoContext, channelToolClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel tool approval plan action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolApprovalPlanActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel tool approval plan action comment: %w", err))
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
	if IsChannelToolsetProposalActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel toolset proposals"))
		}
		req, err := BuildChannelToolsetProposalActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel toolset proposal action: %w", err))
		}
		result, err := RunChannelToolsetProposal(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel toolset proposal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelToolsetProposalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel toolset proposal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelPromptProposalActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel prompt proposals"))
		}
		req, err := BuildChannelPromptProposalActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel prompt proposal action: %w", err))
		}
		result, err := RunChannelPromptProposal(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel prompt proposal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelPromptProposalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel prompt proposal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelBundleProposalActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel bundle proposals"))
		}
		req, err := BuildChannelBundleProposalActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel bundle proposal action: %w", err))
		}
		result, err := RunChannelBundleProposal(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel bundle proposal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBundleProposalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel bundle proposal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSkillStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel skill status"))
		}
		req, err := BuildChannelSkillStatusActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel skill status action: %w", err))
		}
		result, err := RunChannelSkillStatus(ctx, cfg, channelClient, req.Options, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel skill status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSkillStatusActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel skill status action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSkillSearchActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel skill search results"))
		}
		req, err := BuildChannelSkillSearchActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel skill search action: %w", err))
		}
		result, err := RunChannelSkillSearch(ctx, cfg, channelClient, req, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel skill search action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSkillSearchActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel skill search action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSkillInfoActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel skill info"))
		}
		req, err := BuildChannelSkillInfoActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel skill info action: %w", err))
		}
		result, err := RunChannelSkillInfo(ctx, cfg, channelClient, req, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel skill info action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSkillInfoActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel skill info action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSkillProposalActionRequest(ev, cfg) {
		channelSkillClient, ok := github.(interface {
			SkillProposalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel skill proposals"))
		}
		req, err := BuildChannelSkillProposalActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel skill proposal action: %w", err))
		}
		result, err := RunChannelSkillProposal(ctx, cfg, channelSkillClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel skill proposal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSkillProposalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel skill proposal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelSoulProposalActionRequest(ev, cfg) {
		channelSoulClient, ok := github.(interface {
			SoulProposalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel soul proposals"))
		}
		req, err := BuildChannelSoulProposalActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel soul proposal action: %w", err))
		}
		result, err := RunChannelSoulProposal(ctx, cfg, channelSoulClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel soul proposal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSoulProposalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel soul proposal action comment: %w", err))
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
	if IsChannelMemoryProposalActionRequest(ev, cfg) {
		channelMemoryClient, ok := github.(interface {
			MemoryProposalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel memory proposals"))
		}
		req, err := BuildChannelMemoryProposalActionRequest(ev, cfg, repoContext)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel memory proposal action: %w", err))
		}
		result, err := RunChannelMemoryProposal(ctx, cfg, channelMemoryClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel memory proposal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelMemoryProposalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel memory proposal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelWorkspaceProposalActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel workspace proposals"))
		}
		req, err := BuildChannelWorkspaceProposalActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel workspace proposal action: %w", err))
		}
		result, err := RunChannelWorkspaceProposal(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel workspace proposal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelWorkspaceProposalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel workspace proposal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelBoardCardActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel board cards"))
		}
		req, err := BuildChannelBoardCardActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel board card action: %w", err))
		}
		result, err := RunChannelBoardCard(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel board card action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBoardCardActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel board card action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelChecklistActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel checklists"))
		}
		req, err := BuildChannelChecklistActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel checklist action: %w", err))
		}
		result, err := RunChannelChecklist(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel checklist action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelChecklistActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel checklist action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelAgendaActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel agendas"))
		}
		req, err := BuildChannelAgendaActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel agenda action: %w", err))
		}
		result, err := RunChannelAgenda(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel agenda action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelAgendaActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel agenda action comment: %w", err))
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
	if IsChannelBackupSearchActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel backup search results"))
		}
		req, err := BuildChannelBackupSearchActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel backup search action: %w", err))
		}
		result, err := RunChannelBackupSearch(ctx, cfg, channelClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel backup search action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBackupSearchActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel backup search action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelBackupInfoActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel backup info"))
		}
		req, err := BuildChannelBackupInfoActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel backup info action: %w", err))
		}
		result, err := RunChannelBackupInfo(ctx, cfg, channelClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel backup info action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBackupInfoActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel backup info action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelBackupStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel backup status messages"))
		}
		req, err := BuildChannelBackupStatusActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel backup status action: %w", err))
		}
		result, err := RunChannelBackupStatus(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel backup status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBackupStatusActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel backup status action comment: %w", err))
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
	if IsChannelBackupRestoreRequestActionRequest(ev, cfg) {
		channelBackupClient, ok := github.(interface {
			BackupRestoreRequestIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel backup restore requests"))
		}
		req, err := BuildChannelBackupRestoreRequestActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel backup restore request action: %w", err))
		}
		result, err := RunChannelBackupRestoreRequest(ctx, cfg, channelBackupClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel backup restore request action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBackupRestoreRequestActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel backup restore request action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelCheckpointRehearsalActionRequest(ev, cfg) {
		channelCheckpointClient, ok := github.(interface {
			CheckpointRehearsalIssueGitHubClient
			ChannelSendGitHubClient
		})
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel checkpoint rehearsals"))
		}
		req, err := BuildChannelCheckpointRehearsalActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel checkpoint rehearsal action: %w", err))
		}
		result, err := RunChannelCheckpointRehearsal(ctx, cfg, channelCheckpointClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel checkpoint rehearsal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelCheckpointRehearsalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel checkpoint rehearsal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelCheckpointStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel checkpoint status"))
		}
		req, err := BuildChannelCheckpointStatusActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel checkpoint status action: %w", err))
		}
		result, err := RunChannelCheckpointStatus(ctx, cfg, channelClient, req)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel checkpoint status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelCheckpointStatusActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel checkpoint status action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelAvailabilityActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel availability"))
		}
		req, err := BuildChannelAvailabilityActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel availability action: %w", err))
		}
		result, err := RunChannelAvailability(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel availability action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelAvailabilityActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel availability action comment: %w", err))
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
	if IsChannelWatchActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel watches"))
		}
		req, err := BuildChannelWatchActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel watch action: %w", err))
		}
		result, err := RunChannelWatch(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel watch action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelWatchActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel watch action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelStandingOrderProposalActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel standing-order proposals"))
		}
		req, err := BuildChannelStandingOrderProposalActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel standing-order proposal action: %w", err))
		}
		result, err := RunChannelStandingOrderProposal(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel standing-order proposal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelStandingOrderProposalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel standing-order proposal action comment: %w", err))
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
	if IsChannelOpenLoopActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel open loops"))
		}
		req, err := BuildChannelOpenLoopActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel open-loop action: %w", err))
		}
		result, err := RunChannelOpenLoop(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel open-loop action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelOpenLoopActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel open-loop action comment: %w", err))
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
	if IsChannelSnippetActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel snippets"))
		}
		req, err := BuildChannelSnippetActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel snippet action: %w", err))
		}
		result, err := RunChannelSnippet(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel snippet action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelSnippetActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel snippet action comment: %w", err))
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
	if IsChannelJournalActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel journals"))
		}
		req, err := BuildChannelJournalActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel journal action: %w", err))
		}
		result, err := RunChannelJournal(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel journal action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelJournalActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel journal action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelTimeCapsuleActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel time capsules"))
		}
		req, err := BuildChannelTimeCapsuleActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel time capsule action: %w", err))
		}
		result, err := RunChannelTimeCapsule(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel time capsule action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelTimeCapsuleActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel time capsule action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelIdeaActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel ideas"))
		}
		req, err := BuildChannelIdeaActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel idea action: %w", err))
		}
		result, err := RunChannelIdea(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel idea action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelIdeaActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel idea action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelQuestActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel quests"))
		}
		req, err := BuildChannelQuestActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel quest action: %w", err))
		}
		result, err := RunChannelQuest(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel quest action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelQuestActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel quest action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelRitualActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel rituals"))
		}
		req, err := BuildChannelRitualActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel ritual action: %w", err))
		}
		result, err := RunChannelRitual(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel ritual action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelRitualActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel ritual action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelPactActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel pacts"))
		}
		req, err := BuildChannelPactActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel pact action: %w", err))
		}
		result, err := RunChannelPact(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel pact action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelPactActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel pact action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelForecastActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel forecasts"))
		}
		req, err := BuildChannelForecastActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel forecast action: %w", err))
		}
		result, err := RunChannelForecast(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel forecast action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelForecastActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel forecast action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelLoreActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel lore"))
		}
		req, err := BuildChannelLoreActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel lore action: %w", err))
		}
		result, err := RunChannelLore(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel lore action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelLoreActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel lore action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelJamActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel jams"))
		}
		req, err := BuildChannelJamActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel jam action: %w", err))
		}
		result, err := RunChannelJam(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel jam action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelJamActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel jam action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelKudosActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel kudos"))
		}
		req, err := BuildChannelKudosActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel kudos action: %w", err))
		}
		result, err := RunChannelKudos(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel kudos action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelKudosActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel kudos action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelRetroActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel retros"))
		}
		req, err := BuildChannelRetroActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel retro action: %w", err))
		}
		result, err := RunChannelRetro(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel retro action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelRetroActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel retro action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelPlaybookActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel playbooks"))
		}
		req, err := BuildChannelPlaybookActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel playbook action: %w", err))
		}
		result, err := RunChannelPlaybook(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel playbook action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelPlaybookActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel playbook action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelInsightActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel insights"))
		}
		req, err := BuildChannelInsightActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel insight action: %w", err))
		}
		result, err := RunChannelInsight(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel insight action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelInsightActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel insight action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelIncidentActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel incidents"))
		}
		req, err := BuildChannelIncidentActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel incident action: %w", err))
		}
		result, err := RunChannelIncident(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel incident action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelIncidentActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel incident action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelVoiceActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel voice notes"))
		}
		req, err := BuildChannelVoiceActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel voice action: %w", err))
		}
		result, err := RunChannelVoice(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel voice action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelVoiceActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel voice action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelImageActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel image notes"))
		}
		req, err := BuildChannelImageActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel image action: %w", err))
		}
		result, err := RunChannelImage(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel image action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelImageActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel image action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelLinkActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel link cards"))
		}
		req, err := BuildChannelLinkActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel link action: %w", err))
		}
		result, err := RunChannelLink(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel link action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelLinkActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel link action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelBookmarkActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot capture channel bookmarks"))
		}
		req, err := BuildChannelBookmarkActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel bookmark action: %w", err))
		}
		result, err := RunChannelBookmark(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel bookmark action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelBookmarkActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel bookmark action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelForkActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot fork channel threads"))
		}
		req, err := BuildChannelForkActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel fork action: %w", err))
		}
		result, err := RunChannelFork(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel fork action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelForkActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel fork action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelMergeActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot merge channel threads"))
		}
		req, err := BuildChannelMergeActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel merge action: %w", err))
		}
		result, err := RunChannelMerge(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel merge action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelMergeActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel merge action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelAccessRequestActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel access requests"))
		}
		req, err := BuildChannelAccessRequestActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel access request action: %w", err))
		}
		result, err := RunChannelAccessRequest(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel access request action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelAccessRequestActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel access request action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelContactActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel contact cards"))
		}
		req, err := BuildChannelContactActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel contact action: %w", err))
		}
		result, err := RunChannelContact(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel contact action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelContactActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel contact action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelWhoamiActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel identity status"))
		}
		req, err := BuildChannelWhoamiActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel whoami action: %w", err))
		}
		result, err := RunChannelWhoami(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel whoami action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelWhoamiActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel whoami action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelPlatformActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel platform status"))
		}
		req, err := BuildChannelPlatformActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel platform action: %w", err))
		}
		result, err := RunChannelPlatform(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel platform action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelPlatformActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel platform action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelModelStatusActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel model status"))
		}
		req, err := BuildChannelModelStatusActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel model status action: %w", err))
		}
		result, err := RunChannelModelStatus(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel model status action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelModelStatusActionReport(ev, cfg, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel model status action comment: %w", err))
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
	if IsChannelPollVoteActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel poll votes"))
		}
		req, err := BuildChannelPollVoteActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel poll vote action: %w", err))
		}
		result, err := RunChannelPollVote(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel poll vote action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelPollVoteActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel poll vote action comment: %w", err))
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
	if IsChannelRollActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel rolls"))
		}
		req, err := BuildChannelRollActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel roll action: %w", err))
		}
		result, err := RunChannelRoll(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel roll action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelRollActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel roll action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelChooseActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel choices"))
		}
		req, err := BuildChannelChooseActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel choose action: %w", err))
		}
		result, err := RunChannelChoose(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel choose action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelChooseActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel choose action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelRsvpActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot create channel RSVPs"))
		}
		req, err := BuildChannelRsvpActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel rsvp action: %w", err))
		}
		result, err := RunChannelRsvp(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel rsvp action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelRsvpActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel rsvp action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelRsvpResponseActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot record channel RSVP responses"))
		}
		req, err := BuildChannelRsvpResponseActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel rsvp response action: %w", err))
		}
		result, err := RunChannelRsvpResponse(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel rsvp response action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelRsvpResponseActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel rsvp response action comment: %w", err))
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
	if IsChannelTopicActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel topics"))
		}
		req, err := BuildChannelTopicActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel topic action: %w", err))
		}
		result, err := RunChannelTopic(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel topic action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelTopicActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel topic action comment: %w", err))
		}
		status.SetDone()
		return nil
	}
	if IsChannelActivityActionRequest(ev, cfg) {
		channelClient, ok := github.(ChannelSendGitHubClient)
		if !ok {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("github client cannot queue channel activity signals"))
		}
		req, err := BuildChannelActivityActionRequest(ev, cfg)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("build channel activity action: %w", err))
		}
		result, err := RunChannelActivity(ctx, cfg, channelClient, req.Options)
		if err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "channel", fmt.Errorf("run channel activity action: %w", err))
		}
		body := RenderAssistantComment(Marker{
			RunID:          envFirst("GITHUB_RUN_ID", "local"),
			EventID:        eventID(ev),
			Model:          "gitclaw/channels",
			IdempotencyKey: key,
			RunURL:         actionRunURL(ev),
		}, RenderChannelActivityActionReport(ev, req, result))
		if _, err := github.PostIssueComment(ctx, ev.Repo, ev.Issue.Number, body); err != nil {
			return failStartedTurn(ctx, cfg, github, ev, status, "comment", fmt.Errorf("post channel activity action comment: %w", err))
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
