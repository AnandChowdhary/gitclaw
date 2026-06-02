package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSoulInfoOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	InfoID            string
	RequestedPath     string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSoulInfoReport struct {
	RequestedPathHash       string
	NormalizedPathHash      string
	InfoStatus              string
	MatchedSoulFiles        int
	ContextDocuments        int
	ValidationStatus        string
	ValidationErrors        int
	ValidationWarnings      int
	RequiredFiles           int
	PresentRequiredFiles    int
	MissingRequiredFiles    int
	MemoryNotes             int
	NoncanonicalMemoryNotes int
	RiskStatus              string
	RiskFindings            int
	HighRiskFindings        int
	WarningRiskFindings     int
	InfoRiskFindings        int
	RawBodiesIncluded       bool
	Match                   soulInfoMatchResult
}

type ChannelSoulInfoResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	InfoIDHash          string
	RequestedPathHash   string
	NormalizedPathHash  string
	Info                ChannelSoulInfoReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSoulInfoActionRequest struct {
	Options             ChannelSoulInfoOptions
	Info                ChannelSoulInfoReport
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoInfoID          bool
	TargetFromIssue     bool
	PathSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	InfoIDHash          string
	RequestedPathHash   string
	NormalizedPathHash  string
	RequestedPathBytes  int
	SoulInfoMatchHash   string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelSoulInfoActionRequest(ev Event, cfg Config) bool {
	return isChannelSoulInfoActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSoulInfoActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSoulInfoSubcommand(fields[1]) {
	case "soul-info", "souls-info", "context-info", "authority-info", "identity-info", "policy-info", "memory-info", "heartbeat-info", "soul-card", "authority-card", "context-card":
		return true
	default:
		return false
	}
}

func BuildChannelSoulInfoActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSoulInfoActionRequest, error) {
	fields, trailing, ok := channelSoulInfoActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSoulInfoActionRequest{}, fmt.Errorf("missing channel soul info command")
	}
	req := ChannelSoulInfoActionRequest{
		Options: ChannelSoulInfoOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSoulInfoSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var pathParts []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--info-id", "--soul-info-id", "--authority-info-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.InfoID = cleanChannelSoulInfoID(fields[i+1])
			i++
		case "--path", "--soul", "--target", "--file":
			if i+1 >= len(fields) {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			pathParts = append(pathParts, fields[i+1])
			req.PathSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSoulInfoActionRequest{}, fmt.Errorf("unknown channel soul info argument %q", field)
			}
			pathParts = append(pathParts, field)
			if req.PathSource == "" {
				req.PathSource = "positional"
			}
		}
	}
	if strings.TrimSpace(req.Options.RequestedPath) == "" {
		req.Options.RequestedPath = cleanChannelSoulInfoPath(strings.Join(pathParts, " "))
	}
	if strings.TrimSpace(req.Options.RequestedPath) == "" {
		req.Options.RequestedPath = parseChannelSoulInfoTrailingPath(trailing)
		if req.Options.RequestedPath != "" {
			req.PathSource = "trailing-path"
		}
	}
	if err := applyChannelSoulInfoIssueTarget(ev, &req); err != nil {
		return ChannelSoulInfoActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSoulInfoSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.InfoID) == "" {
		req.Options.InfoID = autoChannelSoulInfoID(ev, req.Options)
		req.AutoInfoID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSoulInfoNotifyMessageID(ev, req.Options.InfoID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSoulInfoOptions(req.Options)
	if err := validateChannelSoulInfoActionRequestOptions(req.Options); err != nil {
		return ChannelSoulInfoActionRequest{}, err
	}
	req.Info = BuildChannelSoulInfoReport(cfg, repoContext, req.Options.RequestedPath)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.InfoIDHash = shortDocumentHash(req.Options.InfoID)
	req.RequestedPathHash = req.Info.RequestedPathHash
	req.NormalizedPathHash = req.Info.NormalizedPathHash
	req.RequestedPathBytes = len(req.Options.RequestedPath)
	req.SoulInfoMatchHash = hashStringOrNone(channelSoulInfoMatchIndex(req.Info))
	notificationBody := RenderChannelSoulInfoNotificationBody(req.Options, req.Info)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelSoulInfoReport(cfg Config, repoContext RepoContext, requestedPath string) ChannelSoulInfoReport {
	requestedPath = cleanChannelSoulInfoPath(requestedPath)
	normalized := normalizeSoulInfoPath(requestedPath, cfg, repoContext)
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	report := ChannelSoulInfoReport{
		RequestedPathHash:       shortDocumentHash(requestedPath),
		NormalizedPathHash:      shortDocumentHash(normalized),
		InfoStatus:              "ok",
		ContextDocuments:        len(repoContext.Documents),
		ValidationStatus:        validation.Status,
		ValidationErrors:        validation.Errors,
		ValidationWarnings:      validation.Warnings,
		RequiredFiles:           validation.RequiredFiles,
		PresentRequiredFiles:    validation.PresentRequiredFiles,
		MissingRequiredFiles:    validation.MissingRequiredFiles,
		MemoryNotes:             validation.MemoryNotes,
		NoncanonicalMemoryNotes: validation.NoncanonicalMemoryNotes,
		RiskStatus:              risk.Status,
		RiskFindings:            len(risk.Findings),
		HighRiskFindings:        risk.HighRiskFindings,
		WarningRiskFindings:     risk.WarningRiskFindings,
		InfoRiskFindings:        risk.InfoRiskFindings,
		RawBodiesIncluded:       false,
	}
	if requestedPath == "" || normalized == "" {
		report.InfoStatus = "missing_path"
		return report
	}
	match, ok := soulInfoMatch(cfg.Workdir, repoContext, normalized)
	if !ok {
		report.InfoStatus = "not_found"
		return report
	}
	report.Match = match
	report.MatchedSoulFiles = 1
	if !match.Present {
		report.InfoStatus = "missing"
	}
	return report
}

func RunChannelSoulInfo(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelSoulInfoActionRequest, repoContext RepoContext) (ChannelSoulInfoResult, error) {
	opts := normalizeChannelSoulInfoOptions(req.Options)
	var err error
	opts, err = applyChannelSoulInfoRoute(cfg, opts)
	if err != nil {
		return ChannelSoulInfoResult{}, err
	}
	if err := validateChannelSoulInfoOptions(opts); err != nil {
		return ChannelSoulInfoResult{}, err
	}
	info := req.Info
	if info.RequestedPathHash == "" {
		info = BuildChannelSoulInfoReport(cfg, repoContext, opts.RequestedPath)
	}
	body := RenderChannelSoulInfoNotificationBody(opts, info)
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
		return ChannelSoulInfoResult{}, fmt.Errorf("queue channel soul info notification: %w", err)
	}
	return ChannelSoulInfoResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		InfoIDHash:          shortDocumentHash(opts.InfoID),
		RequestedPathHash:   info.RequestedPathHash,
		NormalizedPathHash:  info.NormalizedPathHash,
		Info:                info,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelSoulInfoActionReport(ev Event, req ChannelSoulInfoActionRequest, result ChannelSoulInfoResult) string {
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
	infoIDHash := result.InfoIDHash
	if infoIDHash == "" {
		infoIDHash = req.InfoIDHash
	}
	requestedPathHash := result.RequestedPathHash
	if requestedPathHash == "" {
		requestedPathHash = req.RequestedPathHash
	}
	normalizedPathHash := result.NormalizedPathHash
	if normalizedPathHash == "" {
		normalizedPathHash = req.NormalizedPathHash
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
	info := result.Info
	if info.RequestedPathHash == "" {
		info = req.Info
	}
	matchHash := hashStringOrNone(channelSoulInfoMatchIndex(info))
	if matchHash == "" {
		matchHash = req.SoulInfoMatchHash
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Soul Info Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_soul_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- soul_info_status: `%s`\n", info.InfoStatus)
	fmt.Fprintf(&b, "- info_mode: `%s`\n", "repo-local-high-authority-metadata-card")
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
	fmt.Fprintf(&b, "- soul_info_id_sha256_12: `%s`\n", noneIfEmpty(infoIDHash))
	fmt.Fprintf(&b, "- soul_info_id_auto: `%t`\n", req.AutoInfoID)
	fmt.Fprintf(&b, "- requested_soul_path_sha256_12: `%s`\n", noneIfEmpty(requestedPathHash))
	fmt.Fprintf(&b, "- normalized_soul_path_sha256_12: `%s`\n", noneIfEmpty(normalizedPathHash))
	fmt.Fprintf(&b, "- requested_soul_path_bytes: `%d`\n", req.RequestedPathBytes)
	fmt.Fprintf(&b, "- path_source: `%s`\n", noneIfEmpty(req.PathSource))
	fmt.Fprintf(&b, "- matched_soul_files: `%d`\n", info.MatchedSoulFiles)
	fmt.Fprintf(&b, "- context_documents: `%d`\n", info.ContextDocuments)
	fmt.Fprintf(&b, "- validation_status: `%s`\n", info.ValidationStatus)
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", info.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", info.ValidationWarnings)
	fmt.Fprintf(&b, "- required_files: `%d`\n", info.RequiredFiles)
	fmt.Fprintf(&b, "- present_required_files: `%d`\n", info.PresentRequiredFiles)
	fmt.Fprintf(&b, "- missing_required_files: `%d`\n", info.MissingRequiredFiles)
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", info.MemoryNotes)
	fmt.Fprintf(&b, "- noncanonical_memory_notes: `%d`\n", info.NoncanonicalMemoryNotes)
	fmt.Fprintf(&b, "- risk_status: `%s`\n", info.RiskStatus)
	fmt.Fprintf(&b, "- risk_findings: `%d`\n", info.RiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", info.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", info.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", info.InfoRiskFindings)
	fmt.Fprintf(&b, "- soul_info_match_sha256_12: `%s`\n", noneIfEmpty(matchHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_writes_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_writes_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_export_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_requested_soul_path_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_normalized_soul_path_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_info_id_included: `%t`\n", false)
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
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_soul_info_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing focused high-authority context card from repo-local soul metadata. The source receipt keeps raw context paths, ids, channel bodies, and file bodies out of band. The action does not call a model, execute tools, mutate repository files, write soul or memory, contact registries, export profiles, call provider APIs, or print raw soul/context bodies.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read soul-info cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent soul-info cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate soul-info notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSoulInfoNotificationBody(opts ChannelSoulInfoOptions, report ChannelSoulInfoReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel soul info\n\n")
	fmt.Fprintf(&b, "Soul info status: %s\n", report.InfoStatus)
	fmt.Fprintf(&b, "Requested path hash: %s\n", report.RequestedPathHash)
	fmt.Fprintf(&b, "Normalized path hash: %s\n", report.NormalizedPathHash)
	fmt.Fprintf(&b, "Matched soul files: %d\n", report.MatchedSoulFiles)
	fmt.Fprintf(&b, "Context documents: %d\n", report.ContextDocuments)
	fmt.Fprintf(&b, "Validation status: %s\n", report.ValidationStatus)
	fmt.Fprintf(&b, "Validation errors: %d\n", report.ValidationErrors)
	fmt.Fprintf(&b, "Validation warnings: %d\n", report.ValidationWarnings)
	fmt.Fprintf(&b, "Required files: %d\n", report.RequiredFiles)
	fmt.Fprintf(&b, "Present required files: %d\n", report.PresentRequiredFiles)
	fmt.Fprintf(&b, "Missing required files: %d\n", report.MissingRequiredFiles)
	fmt.Fprintf(&b, "Memory notes: %d\n", report.MemoryNotes)
	fmt.Fprintf(&b, "Risk status: %s\n", report.RiskStatus)
	fmt.Fprintf(&b, "Risk findings: %d\n", report.RiskFindings)
	fmt.Fprintf(&b, "Soul info id hash: %s\n", shortDocumentHash(opts.InfoID))
	b.WriteString("\nMatch:\n")
	if report.MatchedSoulFiles == 0 {
		b.WriteString("- none\n")
	} else {
		match := report.Match
		fmt.Fprintf(&b, "- path=%s category=%s source=%s present=%t required=%t canonical=%t latest=%t loaded_for_this_turn=%t bytes=%d lines=%d sha256_12=%s at_context_limit=%t\n",
			match.Path,
			match.Category,
			match.Source,
			match.Present,
			match.Required,
			match.Canonical,
			match.Latest,
			match.LoadedForThisTurn,
			match.Bytes,
			match.Lines,
			match.SHA,
			match.AtContextLimit,
		)
	}
	b.WriteString("\nRaw soul, identity, user, memory, tool guidance, heartbeat, channel, issue, comment, prompt, and tool output bodies are not included. Model call: not performed by this action. Soul write: not performed by this action. Memory write: not performed by this action. Registry contact: not performed by this action. Profile export: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSoulInfoActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSoulInfoActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSoulInfoIssueTarget(ev Event, req *ChannelSoulInfoActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel soul info requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSoulInfoOptions(opts ChannelSoulInfoOptions) ChannelSoulInfoOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.InfoID = cleanChannelSoulInfoID(opts.InfoID)
	opts.RequestedPath = cleanChannelSoulInfoPath(opts.RequestedPath)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSoulInfoRoute(cfg Config, opts ChannelSoulInfoOptions) (ChannelSoulInfoOptions, error) {
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
		Body:      "GitClaw channel soul info.",
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

func validateChannelSoulInfoOptions(opts ChannelSoulInfoOptions) error {
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
	if opts.InfoID == "" {
		return fmt.Errorf("missing soul info id")
	}
	if !skillNamePattern.MatchString(opts.InfoID) {
		return fmt.Errorf("invalid soul info id %q", opts.InfoID)
	}
	if cleanChannelSoulInfoPath(opts.RequestedPath) == "" {
		return fmt.Errorf("missing requested soul path")
	}
	return nil
}

func validateChannelSoulInfoActionRequestOptions(opts ChannelSoulInfoOptions) error {
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
	if opts.InfoID == "" {
		return fmt.Errorf("missing soul info id")
	}
	if !skillNamePattern.MatchString(opts.InfoID) {
		return fmt.Errorf("invalid soul info id %q", opts.InfoID)
	}
	if cleanChannelSoulInfoPath(opts.RequestedPath) == "" {
		return fmt.Errorf("missing requested soul path")
	}
	return nil
}

func cleanChannelSoulInfoSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSoulInfoID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSoulInfoPath(value string) string {
	value = cleanSoulInfoPath(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 160 {
		value = strings.TrimSpace(value[:160])
	}
	return value
}

func parseChannelSoulInfoTrailingPath(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "path:") || strings.HasPrefix(lower, "soul:") || strings.HasPrefix(lower, "file:") || strings.HasPrefix(lower, "target:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelSoulInfoPath(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelSoulInfoSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-soul-info-source-%s", eventID(ev))
}

func autoChannelSoulInfoID(ev Event, opts ChannelSoulInfoOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.RequestedPath}, "|")
	return cleanChannelSoulInfoID(fmt.Sprintf("soul-info-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSoulInfoNotifyMessageID(ev Event, infoID string) string {
	seed := strings.Join([]string{eventID(ev), infoID}, "|")
	return fmt.Sprintf("gitclaw-channel-soul-info-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelSoulInfoMatchIndex(report ChannelSoulInfoReport) string {
	if report.MatchedSoulFiles == 0 {
		return ""
	}
	match := report.Match
	return fmt.Sprintf("%s|%s|%s|%t|%t|%t|%t|%t|%d|%d|%s|%t|%s",
		match.Path,
		match.Category,
		match.Source,
		match.Present,
		match.Required,
		match.Canonical,
		match.Latest,
		match.LoadedForThisTurn,
		match.Bytes,
		match.Lines,
		match.SHA,
		match.AtContextLimit,
		report.InfoStatus,
	)
}
