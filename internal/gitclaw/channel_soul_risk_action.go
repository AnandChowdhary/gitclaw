package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSoulRiskOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RiskID            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSoulRiskReport struct {
	Risk                    SoulRiskReport
	ValidationStatus        string
	ValidationErrors        int
	ValidationWarnings      int
	RequiredFiles           int
	PresentRequiredFiles    int
	MissingRequiredFiles    int
	MemoryNotes             int
	NoncanonicalMemoryNotes int
}

type ChannelSoulRiskResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	RiskIDHash          string
	Risk                ChannelSoulRiskReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSoulRiskActionRequest struct {
	Options              ChannelSoulRiskOptions
	Risk                 ChannelSoulRiskReport
	Command              string
	Subcommand           string
	AutoSourceMessageID  bool
	AutoNotifyMessageID  bool
	AutoRiskID           bool
	TargetFromIssue      bool
	RequestedRouteHash   string
	RequestedThreadHash  string
	RequestedMsgHash     string
	NotifyMessageHash    string
	RiskIDHash           string
	RiskCardIndexHash    string
	RiskFindingIndexHash string
	NotificationBodySHA  string
	NotificationBytes    int
	NotificationLines    int
}

func IsChannelSoulRiskActionRequest(ev Event, cfg Config) bool {
	return isChannelSoulRiskActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSoulRiskActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSoulRiskSubcommand(fields[1]) {
	case "soul-risk", "souls-risk", "authority-risk", "identity-risk", "policy-risk", "context-risk", "high-authority-risk", "risk-soul", "risk-authority", "risk-context":
		return true
	default:
		return false
	}
}

func BuildChannelSoulRiskActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSoulRiskActionRequest, error) {
	fields, _, ok := channelSoulRiskActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSoulRiskActionRequest{}, fmt.Errorf("missing channel soul risk command")
	}
	req := ChannelSoulRiskActionRequest{
		Options: ChannelSoulRiskOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSoulRiskSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSoulRiskActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSoulRiskActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSoulRiskActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulRiskActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulRiskActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--risk-id", "--soul-risk-id", "--authority-risk-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSoulRiskActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RiskID = cleanChannelSoulRiskID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSoulRiskActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSoulRiskActionRequest{}, fmt.Errorf("unknown channel soul risk argument %q", field)
			}
			return ChannelSoulRiskActionRequest{}, fmt.Errorf("unexpected channel soul risk argument %q", field)
		}
	}
	if err := applyChannelSoulRiskIssueTarget(ev, &req); err != nil {
		return ChannelSoulRiskActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSoulRiskSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.RiskID) == "" {
		req.Options.RiskID = autoChannelSoulRiskID(ev, req.Options)
		req.AutoRiskID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSoulRiskNotifyMessageID(ev, req.Options.RiskID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSoulRiskOptions(req.Options)
	if err := validateChannelSoulRiskActionRequestOptions(req.Options); err != nil {
		return ChannelSoulRiskActionRequest{}, err
	}
	req.Risk = BuildChannelSoulRiskReport(repoContext)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.RiskIDHash = shortDocumentHash(req.Options.RiskID)
	req.RiskCardIndexHash = hashStringOrNone(channelSoulRiskCardIndex(repoContext))
	req.RiskFindingIndexHash = hashStringOrNone(channelSoulRiskFindingIndex(req.Risk.Risk.Findings))
	notificationBody := RenderChannelSoulRiskNotificationBody(req.Options, req.Risk, repoContext)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelSoulRiskReport(repoContext RepoContext) ChannelSoulRiskReport {
	validation := ValidateSoulContext(repoContext)
	return ChannelSoulRiskReport{
		Risk:                    BuildSoulRiskReport(repoContext),
		ValidationStatus:        validation.Status,
		ValidationErrors:        validation.Errors,
		ValidationWarnings:      validation.Warnings,
		RequiredFiles:           validation.RequiredFiles,
		PresentRequiredFiles:    validation.PresentRequiredFiles,
		MissingRequiredFiles:    validation.MissingRequiredFiles,
		MemoryNotes:             validation.MemoryNotes,
		NoncanonicalMemoryNotes: validation.NoncanonicalMemoryNotes,
	}
}

func RunChannelSoulRisk(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelSoulRiskActionRequest, repoContext RepoContext) (ChannelSoulRiskResult, error) {
	opts := normalizeChannelSoulRiskOptions(req.Options)
	var err error
	opts, err = applyChannelSoulRiskRoute(cfg, opts)
	if err != nil {
		return ChannelSoulRiskResult{}, err
	}
	if err := validateChannelSoulRiskOptions(opts); err != nil {
		return ChannelSoulRiskResult{}, err
	}
	risk := req.Risk
	if risk.Risk.RegistryVerification == "" {
		risk = BuildChannelSoulRiskReport(repoContext)
	}
	body := RenderChannelSoulRiskNotificationBody(opts, risk, repoContext)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelSoulRiskResult{}, fmt.Errorf("queue channel soul risk notification: %w", err)
	}
	return ChannelSoulRiskResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		RiskIDHash:          shortDocumentHash(opts.RiskID),
		Risk:                risk,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelSoulRiskActionReport(ev Event, req ChannelSoulRiskActionRequest, result ChannelSoulRiskResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	threadHash := result.ThreadHash
	if threadHash == "" {
		threadHash = req.RequestedThreadHash
	}
	messageHash := result.MessageHash
	if messageHash == "" {
		messageHash = req.RequestedMsgHash
	}
	notifyHash := result.NotifyHash
	if notifyHash == "" {
		notifyHash = req.NotifyMessageHash
	}
	riskIDHash := result.RiskIDHash
	if riskIDHash == "" {
		riskIDHash = req.RiskIDHash
	}
	notificationBodySHA := result.NotificationBodySHA
	if notificationBodySHA == "" {
		notificationBodySHA = req.NotificationBodySHA
	}
	notificationBytes := result.NotificationBytes
	if notificationBytes == 0 {
		notificationBytes = req.NotificationBytes
	}
	notificationLines := result.NotificationLines
	if notificationLines == 0 {
		notificationLines = req.NotificationLines
	}
	risk := result.Risk
	if risk.Risk.RegistryVerification == "" {
		risk = req.Risk
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Soul Risk Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_soul_risk_status: `%s`\n", status)
	fmt.Fprintf(&b, "- soul_risk_status: `%s`\n", risk.Risk.Status)
	fmt.Fprintf(&b, "- risk_mode: `%s`\n", "repo-local-high-authority-risk-scan")
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- source_message_id_auto: `%t`\n", req.AutoSourceMessageID)
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- soul_risk_id_sha256_12: `%s`\n", noneIfEmpty(riskIDHash))
	fmt.Fprintf(&b, "- soul_risk_id_auto: `%t`\n", req.AutoRiskID)
	fmt.Fprintf(&b, "- context_documents: `%d`\n", risk.Risk.Documents)
	fmt.Fprintf(&b, "- scanned_documents: `%d`\n", risk.Risk.ScannedDocuments)
	fmt.Fprintf(&b, "- documents_with_risk_findings: `%d`\n", risk.Risk.DocumentsWithRiskFindings)
	fmt.Fprintf(&b, "- soul_risk_findings: `%d`\n", len(risk.Risk.Findings))
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", risk.Risk.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", risk.Risk.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", risk.Risk.InfoRiskFindings)
	fmt.Fprintf(&b, "- validation_status: `%s`\n", risk.ValidationStatus)
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", risk.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", risk.ValidationWarnings)
	fmt.Fprintf(&b, "- required_files: `%d`\n", risk.RequiredFiles)
	fmt.Fprintf(&b, "- present_required_files: `%d`\n", risk.PresentRequiredFiles)
	fmt.Fprintf(&b, "- missing_required_files: `%d`\n", risk.MissingRequiredFiles)
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", risk.MemoryNotes)
	fmt.Fprintf(&b, "- noncanonical_memory_notes: `%d`\n", risk.NoncanonicalMemoryNotes)
	fmt.Fprintf(&b, "- registry_verification: `%s`\n", risk.Risk.RegistryVerification)
	fmt.Fprintf(&b, "- profile_export_verification: `%s`\n", risk.Risk.ProfileExportVerification)
	fmt.Fprintf(&b, "- risk_card_index_sha256_12: `%s`\n", noneIfEmpty(req.RiskCardIndexHash))
	fmt.Fprintf(&b, "- risk_finding_index_sha256_12: `%s`\n", noneIfEmpty(req.RiskFindingIndexHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_writes_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_writes_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_export_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_risk_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_identity_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_user_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_guidance_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_heartbeat_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_file_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_risk_finding_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_soul_risk_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing high-authority persistent-state risk card from repo-local soul metadata. The source receipt keeps raw context paths, ids, channel bodies, and file bodies out of band. The action does not call a model, execute tools, mutate repository files, write soul or memory, contact registries, export profiles, call provider APIs, or print raw soul/context bodies.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read soul-risk cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent soul-risk cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate soul-risk notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSoulRiskNotificationBody(opts ChannelSoulRiskOptions, report ChannelSoulRiskReport, repoContext RepoContext) string {
	var b strings.Builder
	b.WriteString("GitClaw channel soul risk\n\n")
	fmt.Fprintf(&b, "Soul risk status: %s\n", report.Risk.Status)
	fmt.Fprintf(&b, "Context documents: %d\n", report.Risk.Documents)
	fmt.Fprintf(&b, "Scanned documents: %d\n", report.Risk.ScannedDocuments)
	fmt.Fprintf(&b, "Documents with risk findings: %d\n", report.Risk.DocumentsWithRiskFindings)
	fmt.Fprintf(&b, "Risk findings: %d\n", len(report.Risk.Findings))
	fmt.Fprintf(&b, "High risk findings: %d\n", report.Risk.HighRiskFindings)
	fmt.Fprintf(&b, "Warning risk findings: %d\n", report.Risk.WarningRiskFindings)
	fmt.Fprintf(&b, "Info risk findings: %d\n", report.Risk.InfoRiskFindings)
	fmt.Fprintf(&b, "Validation status: %s\n", report.ValidationStatus)
	fmt.Fprintf(&b, "Validation errors: %d\n", report.ValidationErrors)
	fmt.Fprintf(&b, "Validation warnings: %d\n", report.ValidationWarnings)
	fmt.Fprintf(&b, "Registry verification: %s\n", report.Risk.RegistryVerification)
	fmt.Fprintf(&b, "Profile export verification: %s\n", report.Risk.ProfileExportVerification)
	fmt.Fprintf(&b, "Soul risk id hash: %s\n", shortDocumentHash(opts.RiskID))
	b.WriteString("\nRisk cards:\n")
	if len(repoContext.Documents) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, doc := range repoContext.Documents {
			findings := scanSoulRiskFindings(doc.Path, doc.Body)
			fmt.Fprintf(&b, "- path=%s category=%s source=%s required=%t bytes=%d lines=%d sha256_12=%s risk_findings=%d risk_max_severity=%s risk_codes=%s line_hashes=%s\n",
				doc.Path,
				soulDocumentCategory(doc.Path),
				soulTrustSource(doc.Path),
				isRequiredSoulDocument(doc.Path),
				len(doc.Body),
				lineCount(doc.Body),
				shortDocumentHash(doc.Body),
				len(findings),
				soulRiskMaxSeverity(findings),
				inlineListOrNone(soulRiskCodes(findings)),
				inlineListOrNone(soulRiskLineHashes(findings)),
			)
		}
	}
	b.WriteString("\nRisk findings:\n")
	if len(report.Risk.Findings) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, finding := range report.Risk.Findings {
			fmt.Fprintf(&b, "- severity=%s code=%s category=%s path=%s line=%d line_sha256_12=%s\n",
				finding.Severity,
				finding.Code,
				finding.Category,
				finding.Path,
				finding.Line,
				finding.LineSHA,
			)
		}
	}
	b.WriteString("\nRaw soul, identity, user, memory, tool guidance, heartbeat, channel, issue, comment, prompt, and tool output bodies are not included. Model call: not performed by this action. Soul write: not performed by this action. Memory write: not performed by this action. Registry contact: not performed by this action. Profile export: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSoulRiskActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSoulRiskActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSoulRiskIssueTarget(ev Event, req *ChannelSoulRiskActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel soul risk requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSoulRiskOptions(opts ChannelSoulRiskOptions) ChannelSoulRiskOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RiskID = cleanChannelSoulRiskID(opts.RiskID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSoulRiskRoute(cfg Config, opts ChannelSoulRiskOptions) (ChannelSoulRiskOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      "GitClaw channel soul risk.",
	})
	if err != nil {
		return opts, err
	}
	opts.Route = routeOpts.Route
	opts.Channel = routeOpts.Channel
	opts.ThreadID = routeOpts.ThreadID
	opts.Author = routeOpts.Author
	return opts, nil
}

func validateChannelSoulRiskOptions(opts ChannelSoulRiskOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.RiskID == "" {
		return fmt.Errorf("missing soul risk id")
	}
	if !skillNamePattern.MatchString(opts.RiskID) {
		return fmt.Errorf("invalid soul risk id %q", opts.RiskID)
	}
	return nil
}

func validateChannelSoulRiskActionRequestOptions(opts ChannelSoulRiskOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Route == "" && (opts.Channel == "" || opts.ThreadID == "") {
		return fmt.Errorf("missing channel route or channel thread target")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.RiskID == "" {
		return fmt.Errorf("missing soul risk id")
	}
	if !skillNamePattern.MatchString(opts.RiskID) {
		return fmt.Errorf("invalid soul risk id %q", opts.RiskID)
	}
	return nil
}

func cleanChannelSoulRiskSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSoulRiskID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelSoulRiskSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-soul-risk-source-%s", eventID(ev))
}

func autoChannelSoulRiskID(ev Event, opts ChannelSoulRiskOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return cleanChannelSoulRiskID(fmt.Sprintf("soul-risk-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSoulRiskNotifyMessageID(ev Event, riskID string) string {
	seed := strings.Join([]string{eventID(ev), riskID}, "|")
	return fmt.Sprintf("gitclaw-channel-soul-risk-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelSoulRiskCardIndex(repoContext RepoContext) string {
	var lines []string
	for _, doc := range repoContext.Documents {
		findings := scanSoulRiskFindings(doc.Path, doc.Body)
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%t|%d|%d|%s|%d|%s|%s|%s",
			doc.Path,
			soulDocumentCategory(doc.Path),
			soulTrustSource(doc.Path),
			isRequiredSoulDocument(doc.Path),
			len(doc.Body),
			lineCount(doc.Body),
			shortDocumentHash(doc.Body),
			len(findings),
			soulRiskMaxSeverity(findings),
			strings.Join(soulRiskCodes(findings), ","),
			strings.Join(soulRiskLineHashes(findings), ","),
		))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(uniqueSortedStrings(lines), "\n")
}

func channelSoulRiskFindingIndex(findings []SoulRiskFinding) string {
	var lines []string
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%s|%d|%s",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Path,
			finding.Line,
			finding.LineSHA,
		))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(uniqueSortedStrings(lines), "\n")
}
