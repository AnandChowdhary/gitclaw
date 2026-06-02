package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelAccessRequestOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	AccessID          string
	Requester         string
	ProviderUserID    string
	ProviderHandle    string
	Scope             string
	RequestedRole     string
	Reason            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelAccessRequestResult struct {
	AccessIssueNumber int
	AccessIssueURL    string
	AccessCreated     bool
	AccessDuplicate   bool
	Notification      ChannelSendResult
	RouteName         string
	RouteHash         string
	Channel           string
	ThreadHash        string
	MessageHash       string
	NotifyHash        string
}

type ChannelAccessRequestActionRequest struct {
	Options             ChannelAccessRequestOptions
	Command             string
	Subcommand          string
	AutoAccessID        bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RequesterSHA        string
	RequesterBytes      int
	RequesterLines      int
	ProviderUserIDSHA   string
	ProviderHandleSHA   string
	ScopeSHA            string
	ScopeBytes          int
	ScopeLines          int
	RequestedRoleSHA    string
	ReasonSHA           string
	ReasonBytes         int
	ReasonLines         int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

type channelAccessRequestDetails struct {
	Requester string
	Scope     string
	Role      string
	Reason    string
}

func IsChannelAccessRequestActionRequest(ev Event, cfg Config) bool {
	return isChannelAccessRequestActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelAccessRequestActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "access", "access-request", "request-access", "pair-request", "pairing-request", "pairing", "allow-request", "permission-request", "permissions":
		return true
	default:
		return false
	}
}

func BuildChannelAccessRequestActionRequest(ev Event, cfg Config) (ChannelAccessRequestActionRequest, error) {
	fields, trailing, ok := channelAccessRequestActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelAccessRequestActionRequest{}, fmt.Errorf("missing channel access request command")
	}
	req := ChannelAccessRequestActionRequest{
		Options: ChannelAccessRequestOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--access-id", "--request-id", "--pairing-id", "--permission-id", "--id":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.AccessID = cleanChannelAccessRequestID(fields[i+1])
			i++
		case "--requester", "--name", "--display-name":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Requester = fields[i+1]
			i++
		case "--provider-user-id", "--user-id", "--member-id":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProviderUserID = fields[i+1]
			i++
		case "--handle", "--username", "--provider-handle":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProviderHandle = fields[i+1]
			i++
		case "--scope":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("--scope requires a value")
			}
			req.Options.Scope = fields[i+1]
			i++
		case "--role", "--requested-role", "--access-role":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RequestedRole = fields[i+1]
			i++
		case "--reason":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("--reason requires a value")
			}
			req.Options.Reason = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelAccessRequestActionRequest{}, fmt.Errorf("unknown channel access request argument %q", field)
			}
			if req.Options.AccessID == "" {
				req.Options.AccessID = cleanChannelAccessRequestID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelAccessRequestActionRequest{}, fmt.Errorf("unexpected channel access request argument %q", field)
		}
	}
	if err := applyChannelAccessRequestIssueTarget(ev, &req); err != nil {
		return ChannelAccessRequestActionRequest{}, err
	}
	details := parseChannelAccessRequestDetails(trailing, ev)
	if strings.TrimSpace(req.Options.Requester) == "" {
		req.Options.Requester = details.Requester
	}
	if strings.TrimSpace(req.Options.Scope) == "" {
		req.Options.Scope = details.Scope
	}
	if strings.TrimSpace(req.Options.RequestedRole) == "" {
		req.Options.RequestedRole = details.Role
	}
	if strings.TrimSpace(req.Options.Reason) == "" {
		req.Options.Reason = details.Reason
	}
	if strings.TrimSpace(req.Options.AccessID) == "" {
		req.Options.AccessID = autoChannelAccessRequestID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Requester, req.Options.ProviderUserID, req.Options.ProviderHandle, req.Options.Scope, req.Options.RequestedRole)
		req.AutoAccessID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelAccessRequestNotifyMessageID(ev, req.Options.AccessID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelAccessRequestOptions(req.Options)
	if err := validateChannelAccessRequestActionRequestOptions(req.Options); err != nil {
		return ChannelAccessRequestActionRequest{}, err
	}
	req.RequesterSHA = shortDocumentHash(req.Options.Requester)
	req.RequesterBytes = len(req.Options.Requester)
	req.RequesterLines = lineCount(req.Options.Requester)
	req.ProviderUserIDSHA = optionalChannelAccessRequestHash(req.Options.ProviderUserID)
	req.ProviderHandleSHA = optionalChannelAccessRequestHash(req.Options.ProviderHandle)
	req.ScopeSHA = shortDocumentHash(req.Options.Scope)
	req.ScopeBytes = len(req.Options.Scope)
	req.ScopeLines = lineCount(req.Options.Scope)
	req.RequestedRoleSHA = shortDocumentHash(req.Options.RequestedRole)
	req.ReasonSHA = shortDocumentHash(req.Options.Reason)
	req.ReasonBytes = len(req.Options.Reason)
	req.ReasonLines = lineCount(req.Options.Reason)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelAccessRequestNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelAccessRequest(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelAccessRequestOptions) (ChannelAccessRequestResult, error) {
	opts = normalizeChannelAccessRequestOptions(opts)
	var err error
	opts, err = applyChannelAccessRequestRoute(cfg, opts)
	if err != nil {
		return ChannelAccessRequestResult{}, err
	}
	if err := validateChannelAccessRequestOptions(opts); err != nil {
		return ChannelAccessRequestResult{}, err
	}
	accessIssue, created, duplicate, err := findOrCreateChannelAccessRequestIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelAccessRequestResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelAccessRequestNotificationBody(opts, accessIssue.Number, issueURL(opts.Repo, accessIssue.Number)),
	})
	if err != nil {
		return ChannelAccessRequestResult{}, fmt.Errorf("queue channel access request notification: %w", err)
	}
	return ChannelAccessRequestResult{
		AccessIssueNumber: accessIssue.Number,
		AccessIssueURL:    issueURL(opts.Repo, accessIssue.Number),
		AccessCreated:     created,
		AccessDuplicate:   duplicate,
		Notification:      notification,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		MessageHash:       shortDocumentHash(opts.SourceMessageID),
		NotifyHash:        shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelAccessRequestActionReport(ev Event, req ChannelAccessRequestActionRequest, result ChannelAccessRequestResult) string {
	status := "opened"
	switch {
	case result.AccessDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.AccessDuplicate:
		status = "existing"
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
	var b strings.Builder
	b.WriteString("## GitClaw Channel Access Request Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_access_request_status: `%s`\n", status)
	fmt.Fprintf(&b, "- access_request_issue: `#%d`\n", result.AccessIssueNumber)
	fmt.Fprintf(&b, "- access_request_issue_url: `%s`\n", result.AccessIssueURL)
	fmt.Fprintf(&b, "- access_request_issue_created: `%t`\n", result.AccessCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.AccessDuplicate)
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
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- access_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.AccessID))
	fmt.Fprintf(&b, "- access_id_auto: `%t`\n", req.AutoAccessID)
	fmt.Fprintf(&b, "- requester_sha256_12: `%s`\n", req.RequesterSHA)
	fmt.Fprintf(&b, "- requester_bytes: `%d`\n", req.RequesterBytes)
	fmt.Fprintf(&b, "- requester_lines: `%d`\n", req.RequesterLines)
	fmt.Fprintf(&b, "- provider_user_id_sha256_12: `%s`\n", noneIfEmpty(req.ProviderUserIDSHA))
	fmt.Fprintf(&b, "- provider_handle_sha256_12: `%s`\n", noneIfEmpty(req.ProviderHandleSHA))
	fmt.Fprintf(&b, "- scope_sha256_12: `%s`\n", req.ScopeSHA)
	fmt.Fprintf(&b, "- scope_bytes: `%d`\n", req.ScopeBytes)
	fmt.Fprintf(&b, "- scope_lines: `%d`\n", req.ScopeLines)
	fmt.Fprintf(&b, "- requested_role_sha256_12: `%s`\n", req.RequestedRoleSHA)
	fmt.Fprintf(&b, "- reason_sha256_12: `%s`\n", req.ReasonSHA)
	fmt.Fprintf(&b, "- reason_bytes: `%d`\n", req.ReasonBytes)
	fmt.Fprintf(&b, "- reason_lines: `%d`\n", req.ReasonLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- permission_grant_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- allowlist_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- pairing_code_issued: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_access_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requester_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_provider_user_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_provider_handle_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_scope_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_role_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_reason_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_access_request_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened a GitHub-native review issue for a channel-origin access or pairing request, then queued a provider-facing review link back to the mirrored thread. This action does not grant access, mutate allowlists, issue pairing codes, call provider APIs, or call a model. The source receipt keeps provider user identifiers, requester names, scopes, roles, reasons, IDs, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the access-review notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent access-review links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate access review issues are suppressed by `access_id`; duplicate access-review notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the access request issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelAccessRequestIssueBody(opts ChannelAccessRequestOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-access-request access_id=\"%s\" channel=\"%s\" requester_sha256_12=\"%s\" provider_user_id_sha256_12=\"%s\" provider_handle_sha256_12=\"%s\" scope_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.AccessID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Requester), optionalChannelAccessRequestHash(opts.ProviderUserID), optionalChannelAccessRequestHash(opts.ProviderHandle), shortDocumentHash(opts.Scope), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel access request.\n\n")
	fmt.Fprintf(&b, "- access_id: %s\n", opts.AccessID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- requester: %s\n", opts.Requester)
	fmt.Fprintf(&b, "- provider_user_id_sha256_12: %s\n", noneIfEmpty(optionalChannelAccessRequestHash(opts.ProviderUserID)))
	fmt.Fprintf(&b, "- provider_handle_sha256_12: %s\n", noneIfEmpty(optionalChannelAccessRequestHash(opts.ProviderHandle)))
	fmt.Fprintf(&b, "- scope: %s\n", opts.Scope)
	fmt.Fprintf(&b, "- requested_role: %s\n", opts.RequestedRole)
	fmt.Fprintf(&b, "- access_mode: github-issue-access-review\n")
	fmt.Fprintf(&b, "- permission_grant_performed: false\n")
	fmt.Fprintf(&b, "- allowlist_mutation_performed: false\n")
	fmt.Fprintf(&b, "- pairing_code_issued: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_provider_user_id_included: false\n")
	fmt.Fprintf(&b, "- raw_provider_handle_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	if strings.TrimSpace(opts.Reason) != "" {
		b.WriteString("## Reason\n\n")
		b.WriteString(strings.TrimSpace(opts.Reason))
		b.WriteString("\n\n")
	}
	b.WriteString("Use this issue as the durable GitHub home for reviewing the channel-origin access or pairing request. This action did not grant access, mutate allowlists, or issue pairing codes.")
	return strings.TrimSpace(b.String())
}

func channelAccessRequestActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelAccessRequestActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelAccessRequestIssueTarget(ev Event, req *ChannelAccessRequestActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel access request requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelAccessRequestDetails(trailing string, ev Event) channelAccessRequestDetails {
	details := channelAccessRequestDetails{
		Requester: fmt.Sprintf("Channel access requester from issue #%d", ev.Issue.Number),
		Scope:     "current-channel-thread",
		Role:      "user",
	}
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	var reasonLines []string
	section := ""
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if section == "reason" && len(reasonLines) > 0 {
				reasonLines = append(reasonLines, "")
			}
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "requester:"):
			details.Requester = strings.TrimSpace(trimmed[len("requester:"):])
			section = ""
		case strings.HasPrefix(lower, "name:"):
			details.Requester = strings.TrimSpace(trimmed[len("name:"):])
			section = ""
		case strings.HasPrefix(lower, "scope:"):
			details.Scope = strings.TrimSpace(trimmed[len("scope:"):])
			section = ""
		case strings.HasPrefix(lower, "role:"):
			details.Role = strings.TrimSpace(trimmed[len("role:"):])
			section = ""
		case strings.HasPrefix(lower, "requested role:"):
			details.Role = strings.TrimSpace(trimmed[len("requested role:"):])
			section = ""
		case strings.HasPrefix(lower, "reason:"):
			remainder := strings.TrimSpace(trimmed[len("reason:"):])
			if remainder != "" {
				reasonLines = append(reasonLines, remainder)
			}
			section = "reason"
		case strings.HasPrefix(lower, "notes:"):
			remainder := strings.TrimSpace(trimmed[len("notes:"):])
			if remainder != "" {
				reasonLines = append(reasonLines, remainder)
			}
			section = "reason"
		default:
			if section == "reason" {
				reasonLines = append(reasonLines, line)
			} else {
				reasonLines = append(reasonLines, line)
				section = "reason"
			}
		}
	}
	if strings.TrimSpace(details.Requester) == "" {
		details.Requester = fmt.Sprintf("Channel access requester from issue #%d", ev.Issue.Number)
	}
	if strings.TrimSpace(details.Scope) == "" {
		details.Scope = "current-channel-thread"
	}
	if strings.TrimSpace(details.Role) == "" {
		details.Role = "user"
	}
	details.Reason = strings.TrimSpace(strings.Join(reasonLines, "\n"))
	return details
}

func normalizeChannelAccessRequestOptions(opts ChannelAccessRequestOptions) ChannelAccessRequestOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.AccessID = cleanChannelAccessRequestID(opts.AccessID)
	opts.Requester = strings.TrimSpace(opts.Requester)
	opts.ProviderUserID = strings.TrimSpace(opts.ProviderUserID)
	opts.ProviderHandle = strings.TrimSpace(opts.ProviderHandle)
	opts.Scope = strings.TrimSpace(opts.Scope)
	opts.RequestedRole = strings.ToLower(strings.TrimSpace(opts.RequestedRole))
	if opts.RequestedRole == "" {
		opts.RequestedRole = "user"
	}
	opts.Reason = strings.TrimSpace(opts.Reason)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelAccessRequestRoute(cfg Config, opts ChannelAccessRequestOptions) (ChannelAccessRequestOptions, error) {
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
		Body:      opts.Requester,
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

func validateChannelAccessRequestOptions(opts ChannelAccessRequestOptions) error {
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
	if opts.AccessID == "" {
		return fmt.Errorf("missing access request id")
	}
	if opts.Requester == "" {
		return fmt.Errorf("missing access requester")
	}
	if opts.Scope == "" {
		return fmt.Errorf("missing access scope")
	}
	if opts.RequestedRole == "" {
		return fmt.Errorf("missing requested role")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing access request source issue")
	}
	return nil
}

func validateChannelAccessRequestActionRequestOptions(opts ChannelAccessRequestOptions) error {
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
	if opts.AccessID == "" {
		return fmt.Errorf("missing access request id")
	}
	if opts.Requester == "" {
		return fmt.Errorf("missing access requester")
	}
	if opts.Scope == "" {
		return fmt.Errorf("missing access scope")
	}
	if opts.RequestedRole == "" {
		return fmt.Errorf("missing requested role")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing access request source issue")
	}
	return nil
}

func findOrCreateChannelAccessRequestIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelAccessRequestOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel access request issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelAccessRequestMatches(issue.Body, opts.AccessID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelAccessRequestIssueTitle(opts), RenderChannelAccessRequestIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel access request issue: %w", err)
	}
	return issue, true, false, nil
}

func channelAccessRequestIssueTitle(opts ChannelAccessRequestOptions) string {
	requester := strings.ReplaceAll(strings.TrimSpace(opts.Requester), "\n", " ")
	if requester == "" {
		requester = opts.AccessID
	}
	if len(requester) > 80 {
		requester = strings.TrimSpace(requester[:80])
	}
	return "GitClaw channel access request: " + requester
}

func channelAccessRequestMatches(body, accessID string) bool {
	return HasChannelAccessRequestMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`access_id="%s"`, escapeMarkerValue(cleanChannelAccessRequestID(accessID))))
}

func cleanChannelAccessRequestID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelAccessRequestID(ev Event, channel, threadID, sourceMessageID, requester, providerUserID, providerHandle, scope, role string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, requester, providerUserID, providerHandle, scope, role}, "|")
	return fmt.Sprintf("access-request-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelAccessRequestNotifyMessageID(ev Event, accessID string) string {
	seed := strings.Join([]string{eventID(ev), accessID}, "|")
	return fmt.Sprintf("gitclaw-channel-access-request-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func optionalChannelAccessRequestHash(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return shortDocumentHash(value)
}

func renderChannelAccessRequestNotificationBody(opts ChannelAccessRequestOptions, accessIssueNumber int, accessIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel access request opened.\n\n")
	if accessIssueNumber > 0 {
		fmt.Fprintf(&b, "Access review: #%d\n", accessIssueNumber)
	}
	if accessIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", accessIssueURL)
	}
	fmt.Fprintf(&b, "Requester: %s\n", strings.TrimSpace(opts.Requester))
	fmt.Fprintf(&b, "Scope: %s\n", strings.TrimSpace(opts.Scope))
	fmt.Fprintf(&b, "Requested role: %s\n", strings.TrimSpace(opts.RequestedRole))
	b.WriteString("\nNo access was granted by this action. Review in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
