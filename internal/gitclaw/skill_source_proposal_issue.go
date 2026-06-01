package gitclaw

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const skillSourceProposalIssueMarker = "gitclaw:skill-source-proposal-issue"

type SkillSourceProposalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type SkillSourceProposalIssueRequest struct {
	Repo                     string
	Command                  string
	Scope                    string
	Subcommand               string
	ProposalID               string
	SourceName               string
	NotifyRoutes             []string
	NotifyRoutesSHA          string
	SourceRefSHA             string
	SourceKind               string
	SkillPath                string
	SourcePinPath            string
	TrustLevel               string
	InstallMode              string
	ExpectedSHA              string
	RequiresApproval         bool
	RemoteFetchAllowed       bool
	ExistingSourcePinMatches int
	ExistingSourcePins       int
	ExistingSkillMatches     int
	AvailableSkills          int
	SourceRiskStatus         string
	SourceRiskFindings       int
	HighRiskFindings         int
	SourceIssueNumber        int
	SourceCommentID          int64
	SourceSHA                string
	SourceBytes              int
	SourceLines              int
	SourceKindOfRequest      string
}

type SkillSourceProposalIssueResult struct {
	IssueNumber         int
	IssueURL            string
	Created             bool
	Duplicate           bool
	ChannelNotification SkillSourceProposalChannelNotification
}

type SkillSourceProposalChannelNotification struct {
	Requested           bool
	Routes              int
	Queued              int
	Duplicates          int
	TargetIssuesCreated int
	MessageSHA          string
	BodySHA             string
	BodyBytes           int
	BodyLines           int
	Destinations        []ChannelBroadcastDestinationResult
}

func IsSkillSourceProposalIssueRequest(ev Event, cfg Config) bool {
	return isSkillSourceProposalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isSkillSourceProposalIssueFields(fields []string) bool {
	if len(fields) < 3 || fields[0] != "/skills" {
		return false
	}
	scope := strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?"))
	subcommand := strings.ToLower(strings.Trim(fields[2], " \t\r\n.,:;!?"))
	if scope != "sources" && scope != "source" {
		return false
	}
	switch subcommand {
	case "propose", "proposal", "proposal-issue", "intake", "request":
		return true
	default:
		return false
	}
}

func BuildSkillSourceProposalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (SkillSourceProposalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isSkillSourceProposalIssueFields(fields) {
		return SkillSourceProposalIssueRequest{}, fmt.Errorf("missing skill source proposal issue command")
	}
	sourceText := activeRequestText(ev)
	args, err := parseSkillSourceProposalIssueArgs(fields[3:], sourceText)
	if err != nil {
		return SkillSourceProposalIssueRequest{}, err
	}
	if args.SourceName == "" || !skillNamePattern.MatchString(args.SourceName) {
		return SkillSourceProposalIssueRequest{}, fmt.Errorf("invalid skill source proposal name %q", args.SourceName)
	}
	if args.SourceRef == "" {
		return SkillSourceProposalIssueRequest{}, fmt.Errorf("missing skill source ref; pass --source <ref>")
	}
	if args.SkillPath == "" {
		args.SkillPath = skillInstallDestinationPath(args.SourceName)
	}
	if !safeSkillSourceProposalPath(args.SkillPath) {
		return SkillSourceProposalIssueRequest{}, fmt.Errorf("unsafe skill source proposal skill path %q", args.SkillPath)
	}
	if args.SourceKind == "" {
		args.SourceKind = inferSkillSourceProposalKind(args.SourceRef)
	}
	args.SourceKind = normalizeSkillSourceKind(args.SourceKind)
	if args.TrustLevel == "" {
		args.TrustLevel = "review-pending"
	}
	args.TrustLevel = normalizeSkillSourceValue(args.TrustLevel)
	if args.InstallMode == "" {
		args.InstallMode = "proposal-only"
	}
	args.InstallMode = normalizeSkillSourceValue(args.InstallMode)
	if args.ProposalID == "" {
		args.ProposalID = cleanSkillSourceProposalID(fmt.Sprintf("skill-source-%s-%s", args.SourceName, shortDocumentHash(args.SourceRef)))
	}
	if !skillNamePattern.MatchString(args.ProposalID) {
		return SkillSourceProposalIssueRequest{}, fmt.Errorf("invalid skill source proposal id %q", args.ProposalID)
	}

	sourceReport := BuildSkillSourceReport(cfg, repoContext)
	sourceMatches := matchingSkillSourceCards(sourceReport.Cards, args.SourceName)
	skillMatches := matchingSkillSummaries(repoContext.SkillSummaries, args.SourceName)
	sourceKind := "issue"
	var sourceCommentID int64
	if ev.Comment != nil {
		sourceKind = "comment"
		sourceCommentID = ev.Comment.ID
	}
	return SkillSourceProposalIssueRequest{
		Repo:                     ev.Repo,
		Command:                  fields[0],
		Scope:                    strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		Subcommand:               strings.ToLower(strings.Trim(fields[2], " \t\r\n.,:;!?")),
		ProposalID:               args.ProposalID,
		SourceName:               args.SourceName,
		NotifyRoutes:             normalizeChannelBroadcastRoutes(args.NotifyRoutes),
		NotifyRoutesSHA:          channelBroadcastRoutesHash(args.NotifyRoutes),
		SourceRefSHA:             shortDocumentHash(args.SourceRef),
		SourceKind:               args.SourceKind,
		SkillPath:                filepath.ToSlash(args.SkillPath),
		SourcePinPath:            skillSourceProposalPinPath(args.SourceName),
		TrustLevel:               args.TrustLevel,
		InstallMode:              args.InstallMode,
		ExpectedSHA:              strings.ToLower(strings.TrimSpace(args.ExpectedSHA)),
		RequiresApproval:         args.RequiresApproval,
		RemoteFetchAllowed:       args.RemoteFetchAllowed,
		ExistingSourcePinMatches: len(sourceMatches),
		ExistingSourcePins:       sourceReport.Specs,
		ExistingSkillMatches:     len(skillMatches),
		AvailableSkills:          availableSkillCount(repoContext),
		SourceRiskStatus:         sourceReport.Status,
		SourceRiskFindings:       len(sourceReport.Findings),
		HighRiskFindings:         sourceReport.HighRiskFindings,
		SourceIssueNumber:        ev.Issue.Number,
		SourceCommentID:          sourceCommentID,
		SourceSHA:                shortDocumentHash(sourceText),
		SourceBytes:              len(sourceText),
		SourceLines:              lineCount(sourceText),
		SourceKindOfRequest:      sourceKind,
	}, nil
}

func RunSkillSourceProposalChannelNotification(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req SkillSourceProposalIssueRequest, result SkillSourceProposalIssueResult) (SkillSourceProposalChannelNotification, error) {
	notification := SkillSourceProposalChannelNotification{
		Requested: len(req.NotifyRoutes) > 0,
		Routes:    len(req.NotifyRoutes),
	}
	if len(req.NotifyRoutes) == 0 {
		return notification, nil
	}
	if result.IssueNumber <= 0 {
		return notification, fmt.Errorf("missing skill source proposal issue for channel notification")
	}
	body := RenderSkillSourceProposalChannelNotificationBody(req, result)
	messageID := skillSourceProposalChannelNotificationMessageID(req)
	broadcast, err := RunChannelBroadcast(ctx, cfg, github, ChannelBroadcastOptions{
		Repo:      req.Repo,
		Routes:    req.NotifyRoutes,
		MessageID: messageID,
		Body:      body,
	})
	if err != nil {
		return notification, err
	}
	notification.Queued = broadcast.Queued
	notification.Duplicates = broadcast.Duplicates
	notification.TargetIssuesCreated = broadcast.Created
	notification.MessageSHA = shortDocumentHash(messageID)
	notification.BodySHA = shortDocumentHash(body)
	notification.BodyBytes = len(body)
	notification.BodyLines = lineCount(body)
	notification.Destinations = broadcast.Destinations
	return notification, nil
}

func RunSkillSourceProposalIssue(ctx context.Context, cfg Config, github SkillSourceProposalIssueGitHubClient, req SkillSourceProposalIssueRequest) (SkillSourceProposalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return SkillSourceProposalIssueResult{}, err
	}
	if req.ProposalID == "" {
		return SkillSourceProposalIssueResult{}, fmt.Errorf("missing skill source proposal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return SkillSourceProposalIssueResult{}, fmt.Errorf("list skill source proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if skillSourceProposalIssueMatches(issue.Body, req.ProposalID) {
			return SkillSourceProposalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	title := fmt.Sprintf("GitClaw skill source proposal: %s", req.SourceName)
	issue, err := github.CreateIssue(ctx, req.Repo, title, RenderSkillSourceProposalIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return SkillSourceProposalIssueResult{}, fmt.Errorf("create skill source proposal issue: %w", err)
	}
	return SkillSourceProposalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderSkillSourceProposalIssueBody(req SkillSourceProposalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" name=\"%s\" source_ref_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", skillSourceProposalIssueMarker, escapeMarkerValue(req.ProposalID), escapeMarkerValue(req.SourceName), escapeMarkerValue(req.SourceRefSHA), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw skill source proposal issue.\n\n")
	fmt.Fprintf(&b, "- proposal_id: %s\n", req.ProposalID)
	fmt.Fprintf(&b, "- source_name: %s\n", req.SourceName)
	fmt.Fprintf(&b, "- source_pin_path: %s\n", req.SourcePinPath)
	fmt.Fprintf(&b, "- proposed_skill_path: %s\n", req.SkillPath)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_ref_sha256_12: %s\n", req.SourceRefSHA)
	fmt.Fprintf(&b, "- trust_level: %s\n", req.TrustLevel)
	fmt.Fprintf(&b, "- install_mode: %s\n", req.InstallMode)
	fmt.Fprintf(&b, "- expected_sha256_12: %s\n", valueOrNone(req.ExpectedSHA))
	fmt.Fprintf(&b, "- requires_approval: %t\n", req.RequiresApproval)
	fmt.Fprintf(&b, "- remote_fetch_allowed: %t\n", req.RemoteFetchAllowed)
	fmt.Fprintf(&b, "- existing_source_pin_matches: %d\n", req.ExistingSourcePinMatches)
	fmt.Fprintf(&b, "- existing_source_pins: %d\n", req.ExistingSourcePins)
	fmt.Fprintf(&b, "- existing_skill_matches: %d\n", req.ExistingSkillMatches)
	fmt.Fprintf(&b, "- available_skills: %d\n", req.AvailableSkills)
	fmt.Fprintf(&b, "- current_source_risk_status: %s\n", req.SourceRiskStatus)
	fmt.Fprintf(&b, "- current_source_risk_findings: %d\n", req.SourceRiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: %d\n", req.HighRiskFindings)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- request_kind: %s\n", req.SourceKindOfRequest)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: %d\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: %d\n", req.SourceLines)
	b.WriteString("- review_pr_required: true\n")
	b.WriteString("- registry_contact_allowed: false\n")
	b.WriteString("- installer_scripts_run: false\n")
	b.WriteString("- dependency_install_allowed: false\n")
	b.WriteString("- source_pin_file_written: false\n")
	b.WriteString("- active_skill_write_performed: false\n")
	b.WriteString("- raw_source_ref_included: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_skill_body_included: false\n\n")
	fmt.Fprintf(&b, "Review the source request, then draft `%s` on a normal code-review branch if the proposed source pin is worth keeping. GitClaw does not fetch, install, or trust external skill sources from this issue.\n", req.SourcePinPath)
	return strings.TrimSpace(b.String())
}

func RenderSkillSourceProposalIssueActionReport(ev Event, req SkillSourceProposalIssueRequest, result SkillSourceProposalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Skill Source Proposal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_skill_command: `%s %s %s`\n", req.Command, req.Scope, req.Subcommand)
	fmt.Fprintf(&b, "- skill_source_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- skill_source_proposal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- skill_source_proposal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- skill_source_proposal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- proposal_id_sha256_12: `%s`\n", shortDocumentHash(req.ProposalID))
	fmt.Fprintf(&b, "- source_name: `%s`\n", inlineCode(req.SourceName))
	fmt.Fprintf(&b, "- channel_notification_requested: `%t`\n", result.ChannelNotification.Requested)
	fmt.Fprintf(&b, "- channel_notification_routes: `%d`\n", result.ChannelNotification.Routes)
	fmt.Fprintf(&b, "- channel_notification_queued: `%d`\n", result.ChannelNotification.Queued)
	fmt.Fprintf(&b, "- channel_notification_duplicates: `%d`\n", result.ChannelNotification.Duplicates)
	fmt.Fprintf(&b, "- channel_notification_target_issues_created: `%d`\n", result.ChannelNotification.TargetIssuesCreated)
	fmt.Fprintf(&b, "- channel_notification_routes_sha256_12: `%s`\n", noneIfEmpty(req.NotifyRoutesSHA))
	fmt.Fprintf(&b, "- channel_notification_message_id_sha256_12: `%s`\n", noneIfEmpty(result.ChannelNotification.MessageSHA))
	fmt.Fprintf(&b, "- channel_notification_body_sha256_12: `%s`\n", noneIfEmpty(result.ChannelNotification.BodySHA))
	fmt.Fprintf(&b, "- channel_notification_body_bytes: `%d`\n", result.ChannelNotification.BodyBytes)
	fmt.Fprintf(&b, "- channel_notification_body_lines: `%d`\n", result.ChannelNotification.BodyLines)
	fmt.Fprintf(&b, "- source_pin_path: `%s`\n", req.SourcePinPath)
	fmt.Fprintf(&b, "- proposed_skill_path: `%s`\n", req.SkillPath)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_ref_sha256_12: `%s`\n", req.SourceRefSHA)
	fmt.Fprintf(&b, "- trust_level: `%s`\n", req.TrustLevel)
	fmt.Fprintf(&b, "- install_mode: `%s`\n", req.InstallMode)
	fmt.Fprintf(&b, "- expected_sha256_12: `%s`\n", valueOrNone(req.ExpectedSHA))
	fmt.Fprintf(&b, "- requires_approval: `%t`\n", req.RequiresApproval)
	fmt.Fprintf(&b, "- remote_fetch_allowed: `%t`\n", req.RemoteFetchAllowed)
	fmt.Fprintf(&b, "- existing_source_pin_matches: `%d`\n", req.ExistingSourcePinMatches)
	fmt.Fprintf(&b, "- existing_source_pins: `%d`\n", req.ExistingSourcePins)
	fmt.Fprintf(&b, "- existing_skill_matches: `%d`\n", req.ExistingSkillMatches)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", req.AvailableSkills)
	fmt.Fprintf(&b, "- current_source_risk_status: `%s`\n", req.SourceRiskStatus)
	fmt.Fprintf(&b, "- current_source_risk_findings: `%d`\n", req.SourceRiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", req.HighRiskFindings)
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-skill-source-pin")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- proposal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- remote_fetch_runtime_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- dependency_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_ref_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_routes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_notification_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_pin_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- active_skill_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_source_proposal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for reviewing a skill source pin. The action does not fetch registries, install skills, write `.gitclaw/skill-sources`, or call a model; continue on the proposal issue to use GitHub Models for review discussion.\n\n")
	b.WriteString("### Review Path\n")
	fmt.Fprintf(&b, "- continue on proposal issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- if accepted, draft `%s` on a normal branch and review it as code\n", req.SourcePinPath)
	b.WriteString("- run `gitclaw skills sources verify`, `gitclaw skills sources risk`, and a live GitHub Models conversation E2E before promotion\n")
	if result.ChannelNotification.Requested {
		b.WriteString("\n### Channel Notifications\n")
		if len(result.ChannelNotification.Destinations) == 0 {
			b.WriteString("- none\n")
		} else {
			for _, destination := range result.ChannelNotification.Destinations {
				fmt.Fprintf(
					&b,
					"- destination=`%02d` target_issue=`#%d` outbound_comment_id=`%d` target_issue_created=`%t` duplicate_suppressed=`%t` route_sha256_12=`%s` channel=`%s` thread_id_sha256_12=`%s` message_id_sha256_12=`%s` body_sha256_12=`%s`\n",
					destination.Index,
					destination.IssueNumber,
					destination.CommentID,
					destination.Created,
					destination.Duplicate,
					noneIfEmpty(destination.RouteHash),
					destination.Channel,
					noneIfEmpty(destination.ThreadHash),
					noneIfEmpty(destination.MessageHash),
					noneIfEmpty(destination.BodyHash),
				)
			}
		}
		b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	}
	return strings.TrimSpace(b.String())
}

type skillSourceProposalIssueArgs struct {
	ProposalID         string
	SourceName         string
	NotifyRoutes       []string
	SourceRef          string
	SourceKind         string
	SkillPath          string
	TrustLevel         string
	InstallMode        string
	ExpectedSHA        string
	RequiresApproval   bool
	RemoteFetchAllowed bool
}

func parseSkillSourceProposalIssueArgs(args []string, sourceText string) (skillSourceProposalIssueArgs, error) {
	parsed := skillSourceProposalIssueArgs{RequiresApproval: true}
	var positionals []string
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--id", "--proposal-id":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("%s requires a value", field)
			}
			parsed.ProposalID = cleanSkillSourceProposalID(args[i])
		case "--name":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("--name requires a value")
			}
			parsed.SourceName = normalizeSkillInstallCandidate(args[i])
		case "--source", "--source-ref", "--ref":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("%s requires a value", field)
			}
			parsed.SourceRef = strings.Trim(strings.TrimSpace(args[i]), "`\"'")
		case "--notify-route", "--notify-routes", "--channel-route", "--channel-routes":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("%s requires a value", field)
			}
			parsed.NotifyRoutes = append(parsed.NotifyRoutes, splitChannelBroadcastRoutes(args[i])...)
		case "--source-kind", "--kind":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("%s requires a value", field)
			}
			parsed.SourceKind = args[i]
		case "--skill-path", "--path":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("%s requires a value", field)
			}
			parsed.SkillPath = filepath.ToSlash(strings.Trim(strings.TrimSpace(args[i]), "`\"'"))
		case "--trust", "--trust-level":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("%s requires a value", field)
			}
			parsed.TrustLevel = args[i]
		case "--install-mode":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("--install-mode requires a value")
			}
			parsed.InstallMode = args[i]
		case "--expected-sha", "--expected-sha256-12", "--sha":
			i++
			if i >= len(args) {
				return parsed, fmt.Errorf("%s requires a value", field)
			}
			parsed.ExpectedSHA = strings.Trim(strings.TrimSpace(args[i]), "`\"'")
		case "--remote-fetch-allowed":
			parsed.RemoteFetchAllowed = true
		case "--no-remote-fetch":
			parsed.RemoteFetchAllowed = false
		case "--approval-required":
			parsed.RequiresApproval = true
		case "--no-approval-required":
			parsed.RequiresApproval = false
		default:
			if strings.HasPrefix(field, "--") {
				return parsed, fmt.Errorf("unknown skill source proposal argument %q", field)
			}
			positionals = append(positionals, field)
		}
	}
	if parsed.SourceName == "" && len(positionals) > 0 {
		target := classifySkillInstallTarget(positionals[0])
		if target.Remote || strings.Contains(positionals[0], "/") || strings.Contains(positionals[0], ":") {
			parsed.SourceName = skillSourceProposalNameFromRef(positionals[0])
			if parsed.SourceRef == "" {
				parsed.SourceRef = strings.Trim(strings.TrimSpace(positionals[0]), "`\"'")
			}
		} else {
			parsed.SourceName = normalizeSkillInstallCandidate(positionals[0])
		}
	}
	if parsed.SourceRef == "" && len(positionals) > 1 {
		parsed.SourceRef = strings.Trim(strings.TrimSpace(positionals[1]), "`\"'")
	}
	if parsed.SourceName == "" && parsed.SourceRef != "" {
		parsed.SourceName = skillSourceProposalNameFromRef(parsed.SourceRef)
	}
	if parsed.SourceName == "" {
		parsed.SourceName = normalizeSkillInstallCandidate("skill-source-" + shortDocumentHash(sourceText))
	}
	parsed.NotifyRoutes = normalizeChannelBroadcastRoutes(parsed.NotifyRoutes)
	return parsed, nil
}

func RenderSkillSourceProposalChannelNotificationBody(req SkillSourceProposalIssueRequest, result SkillSourceProposalIssueResult) string {
	var b strings.Builder
	b.WriteString("GitClaw skill source proposal\n\n")
	fmt.Fprintf(&b, "Review issue: #%d %s\n", result.IssueNumber, result.IssueURL)
	fmt.Fprintf(&b, "Source issue: #%d %s\n", req.SourceIssueNumber, issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "Source name: %s\n", req.SourceName)
	fmt.Fprintf(&b, "Source kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "Source ref sha256_12: %s\n", req.SourceRefSHA)
	fmt.Fprintf(&b, "Source pin path: %s\n", req.SourcePinPath)
	fmt.Fprintf(&b, "Proposed skill path: %s\n", req.SkillPath)
	fmt.Fprintf(&b, "Trust level: %s\n", req.TrustLevel)
	fmt.Fprintf(&b, "Install mode: %s\n", req.InstallMode)
	fmt.Fprintf(&b, "Review PR required: %t\n", true)
	b.WriteString("\nReview the GitHub source-pin proposal issue before drafting source YAML on a normal code-review branch. This notification did not call a model, fetch external sources, run installers, write source pins, write active skills, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func skillSourceProposalChannelNotificationMessageID(req SkillSourceProposalIssueRequest) string {
	return fmt.Sprintf("gitclaw-skill-source-proposal-%s", req.ProposalID)
}

func skillSourceProposalNameFromRef(sourceRef string) string {
	sourceRef = strings.Trim(strings.TrimSpace(sourceRef), "`\"'")
	if u, err := url.Parse(sourceRef); err == nil && u.Scheme != "" && strings.EqualFold(u.Hostname(), "github.com") {
		parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
		if len(parts) >= 2 {
			return normalizeSkillInstallCandidate(parts[1])
		}
	}
	return classifySkillInstallTarget(sourceRef).Candidate
}

func cleanSkillSourceProposalID(value string) string {
	return cleanSkillRehearsalID(value)
}

func inferSkillSourceProposalKind(sourceRef string) string {
	lower := strings.ToLower(strings.TrimSpace(sourceRef))
	switch {
	case strings.HasPrefix(lower, "github:"), strings.Contains(lower, "github.com"), strings.HasPrefix(lower, "git@github.com:"):
		return "github"
	case strings.HasPrefix(lower, "https://"):
		return "https-url"
	case strings.HasPrefix(lower, ".gitclaw/"), strings.HasPrefix(lower, "./.gitclaw/"):
		return "repo-local"
	case strings.Contains(lower, "clawhub"):
		return "clawhub"
	case strings.Contains(lower, "hermes"):
		return "hermes-hub"
	default:
		return "well-known"
	}
}

func skillSourceProposalPinPath(name string) string {
	if name == "" {
		return ""
	}
	return skillSourcesDir + "/" + name + ".yaml"
}

func safeSkillSourceProposalPath(path string) bool {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "\\") || strings.Contains(path, "\x00") {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if cleaned != path || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") || strings.Contains(cleaned, "..") {
		return false
	}
	return strings.HasPrefix(cleaned, ".gitclaw/SKILLS/") && strings.HasSuffix(strings.ToLower(cleaned), "/skill.md")
}

func skillSourceProposalIssueMatches(body, proposalID string) bool {
	return strings.Contains(body, "<!-- "+skillSourceProposalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(proposalID)))
}
