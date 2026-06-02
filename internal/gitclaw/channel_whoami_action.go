package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelWhoamiOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	IdentityID      string
	DisplayName     string
	ProviderUserID  string
	ProviderHandle  string
	Role            string
	Notes           string
	Author          string
}

type ChannelWhoamiResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	BodyHash     string
}

type ChannelWhoamiActionRequest struct {
	Options             ChannelWhoamiOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoIdentityID      bool
	TargetFromIssue     bool
	DisplayNameSHA      string
	DisplayNameBytes    int
	DisplayNameLines    int
	ProviderUserIDSHA   string
	ProviderHandleSHA   string
	RoleSHA             string
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	IdentityIDHash      string
	NotificationBodySHA string
}

type channelWhoamiDetails struct {
	DisplayName string
	Role        string
	IdentityID  string
	Notes       string
}

func IsChannelWhoamiActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelWhoamiActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelWhoamiActionFields(fields)
}

func isChannelWhoamiActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "whoami", "who", "me", "identity-status", "access-status", "account":
		return true
	default:
		return false
	}
}

func BuildChannelWhoamiActionRequest(ev Event, cfg Config) (ChannelWhoamiActionRequest, error) {
	fields, trailing, ok := channelWhoamiActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelWhoamiActionRequest{}, fmt.Errorf("missing channel whoami command")
	}
	req := ChannelWhoamiActionRequest{
		Options: ChannelWhoamiOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--identity-id", "--identity", "--id":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.IdentityID = cleanChannelWhoamiIdentityID(fields[i+1])
			i++
		case "--display-name", "--name":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.DisplayName = fields[i+1]
			i++
		case "--provider-user-id", "--user-id", "--member-id":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProviderUserID = fields[i+1]
			i++
		case "--handle", "--username", "--provider-handle":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProviderHandle = fields[i+1]
			i++
		case "--role", "--tier", "--access-role":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Role = fields[i+1]
			i++
		case "--notes":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("--notes requires a value")
			}
			req.Options.Notes = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelWhoamiActionRequest{}, fmt.Errorf("unknown channel whoami argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelWhoamiActionRequest{}, fmt.Errorf("unexpected channel whoami argument %q", field)
		}
	}
	if err := applyChannelWhoamiIssueTarget(ev, &req); err != nil {
		return ChannelWhoamiActionRequest{}, err
	}
	details := parseChannelWhoamiDetails(trailing, ev)
	if strings.TrimSpace(req.Options.DisplayName) == "" {
		req.Options.DisplayName = details.DisplayName
	}
	if strings.TrimSpace(req.Options.Role) == "" {
		req.Options.Role = details.Role
	}
	if strings.TrimSpace(req.Options.IdentityID) == "" {
		req.Options.IdentityID = details.IdentityID
	}
	if strings.TrimSpace(req.Options.Notes) == "" {
		req.Options.Notes = details.Notes
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelWhoamiSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.IdentityID) == "" {
		req.Options.IdentityID = autoChannelWhoamiIdentityID(ev, req.Options)
		req.AutoIdentityID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelWhoamiNotifyMessageID(ev, req.Options.IdentityID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelWhoamiOptions(req.Options)
	if err := validateChannelWhoamiActionRequestOptions(req.Options); err != nil {
		return ChannelWhoamiActionRequest{}, err
	}
	req.DisplayNameSHA = shortDocumentHash(req.Options.DisplayName)
	req.DisplayNameBytes = len(req.Options.DisplayName)
	req.DisplayNameLines = lineCount(req.Options.DisplayName)
	req.ProviderUserIDSHA = optionalChannelWhoamiHash(req.Options.ProviderUserID)
	req.ProviderHandleSHA = optionalChannelWhoamiHash(req.Options.ProviderHandle)
	req.RoleSHA = shortDocumentHash(req.Options.Role)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.IdentityIDHash = shortDocumentHash(req.Options.IdentityID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelWhoamiNotificationBody(req.Options))
	return req, nil
}

func RunChannelWhoami(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelWhoamiOptions) (ChannelWhoamiResult, error) {
	opts = normalizeChannelWhoamiOptions(opts)
	var err error
	opts, err = applyChannelWhoamiRoute(cfg, opts)
	if err != nil {
		return ChannelWhoamiResult{}, err
	}
	if err := validateChannelWhoamiOptions(opts); err != nil {
		return ChannelWhoamiResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelWhoamiNotificationBody(opts),
	})
	if err != nil {
		return ChannelWhoamiResult{}, fmt.Errorf("queue channel whoami notification: %w", err)
	}
	return ChannelWhoamiResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		BodyHash:     shortDocumentHash(renderChannelWhoamiNotificationBody(opts)),
	}, nil
}

func RenderChannelWhoamiActionReport(ev Event, req ChannelWhoamiActionRequest, result ChannelWhoamiResult) string {
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
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Whoami Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_whoami_status: `%s`\n", status)
	fmt.Fprintf(&b, "- identity_record_status: `%s`\n", "pending-github-review")
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
	fmt.Fprintf(&b, "- identity_id_sha256_12: `%s`\n", noneIfEmpty(req.IdentityIDHash))
	fmt.Fprintf(&b, "- identity_id_auto: `%t`\n", req.AutoIdentityID)
	fmt.Fprintf(&b, "- display_name_sha256_12: `%s`\n", req.DisplayNameSHA)
	fmt.Fprintf(&b, "- display_name_bytes: `%d`\n", req.DisplayNameBytes)
	fmt.Fprintf(&b, "- display_name_lines: `%d`\n", req.DisplayNameLines)
	fmt.Fprintf(&b, "- provider_user_id_sha256_12: `%s`\n", noneIfEmpty(req.ProviderUserIDSHA))
	fmt.Fprintf(&b, "- provider_handle_sha256_12: `%s`\n", noneIfEmpty(req.ProviderHandleSHA))
	fmt.Fprintf(&b, "- role_sha256_12: `%s`\n", req.RoleSHA)
	fmt.Fprintf(&b, "- notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- permission_grant_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- allowlist_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- pairing_code_issued: `%t`\n", false)
	fmt.Fprintf(&b, "- contact_card_created: `%t`\n", false)
	fmt.Fprintf(&b, "- access_review_created: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_identity_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_display_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_provider_user_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_provider_handle_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_role_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_whoami_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel identity status message on the canonical channel issue. This is the GitHub-native `/whoami` primitive: it tells the sender what GitClaw can safely say about the channel identity, but it does not create a contact card, open an access review, grant access, mutate allowlists, issue pairing codes, call provider APIs, or call a model. The source receipt keeps provider identifiers, display names, roles, notes, IDs, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the identity-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent identity-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate identity-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/channels contact` when a durable GitHub contact card is needed, or `/channels access-request` when an access or pairing review is needed\n")
	return strings.TrimSpace(b.String())
}

func channelWhoamiActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelWhoamiActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelWhoamiIssueTarget(ev Event, req *ChannelWhoamiActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel whoami requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelWhoamiDetails(trailing string, ev Event) channelWhoamiDetails {
	details := channelWhoamiDetails{
		DisplayName: fmt.Sprintf("Channel participant from issue #%d", ev.Issue.Number),
		Role:        "user",
	}
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	var notesLines []string
	section := ""
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if section == "notes" && len(notesLines) > 0 {
				notesLines = append(notesLines, "")
			}
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "display name:"):
			details.DisplayName = strings.TrimSpace(trimmed[len("display name:"):])
			section = ""
		case strings.HasPrefix(lower, "display_name:"):
			details.DisplayName = strings.TrimSpace(trimmed[len("display_name:"):])
			section = ""
		case strings.HasPrefix(lower, "displayname:"):
			details.DisplayName = strings.TrimSpace(trimmed[len("displayname:"):])
			section = ""
		case strings.HasPrefix(lower, "name:"):
			details.DisplayName = strings.TrimSpace(trimmed[len("name:"):])
			section = ""
		case strings.HasPrefix(lower, "role:"):
			details.Role = strings.TrimSpace(trimmed[len("role:"):])
			section = ""
		case strings.HasPrefix(lower, "tier:"):
			details.Role = strings.TrimSpace(trimmed[len("tier:"):])
			section = ""
		case strings.HasPrefix(lower, "access role:"):
			details.Role = strings.TrimSpace(trimmed[len("access role:"):])
			section = ""
		case strings.HasPrefix(lower, "identity id:"):
			details.IdentityID = cleanChannelWhoamiIdentityID(strings.TrimSpace(trimmed[len("identity id:"):]))
			section = ""
		case strings.HasPrefix(lower, "identity_id:"):
			details.IdentityID = cleanChannelWhoamiIdentityID(strings.TrimSpace(trimmed[len("identity_id:"):]))
			section = ""
		case strings.HasPrefix(lower, "identity:"):
			details.IdentityID = cleanChannelWhoamiIdentityID(strings.TrimSpace(trimmed[len("identity:"):]))
			section = ""
		case strings.HasPrefix(lower, "notes:"):
			remainder := strings.TrimSpace(trimmed[len("notes:"):])
			if remainder != "" {
				notesLines = append(notesLines, remainder)
			}
			section = "notes"
		default:
			if section == "notes" {
				notesLines = append(notesLines, line)
			} else {
				notesLines = append(notesLines, line)
				section = "notes"
			}
		}
	}
	if strings.TrimSpace(details.DisplayName) == "" {
		details.DisplayName = fmt.Sprintf("Channel participant from issue #%d", ev.Issue.Number)
	}
	if strings.TrimSpace(details.Role) == "" {
		details.Role = "user"
	}
	details.Notes = strings.TrimSpace(strings.Join(notesLines, "\n"))
	return details
}

func normalizeChannelWhoamiOptions(opts ChannelWhoamiOptions) ChannelWhoamiOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.IdentityID = cleanChannelWhoamiIdentityID(opts.IdentityID)
	opts.DisplayName = strings.TrimSpace(opts.DisplayName)
	opts.ProviderUserID = strings.TrimSpace(opts.ProviderUserID)
	opts.ProviderHandle = strings.TrimSpace(opts.ProviderHandle)
	opts.Role = cleanChannelReaction(opts.Role)
	if opts.Role == "" {
		opts.Role = "user"
	}
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelWhoamiRoute(cfg Config, opts ChannelWhoamiOptions) (ChannelWhoamiOptions, error) {
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
		Body:      opts.DisplayName,
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

func validateChannelWhoamiOptions(opts ChannelWhoamiOptions) error {
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
	if opts.IdentityID == "" {
		return fmt.Errorf("missing identity id")
	}
	if opts.DisplayName == "" {
		return fmt.Errorf("missing identity display name")
	}
	if opts.Role == "" {
		return fmt.Errorf("missing identity role")
	}
	return nil
}

func validateChannelWhoamiActionRequestOptions(opts ChannelWhoamiOptions) error {
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
	if opts.IdentityID == "" {
		return fmt.Errorf("missing identity id")
	}
	if opts.DisplayName == "" {
		return fmt.Errorf("missing identity display name")
	}
	if opts.Role == "" {
		return fmt.Errorf("missing identity role")
	}
	return nil
}

func cleanChannelWhoamiIdentityID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelWhoamiSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-whoami-source-%s", eventID(ev))
}

func autoChannelWhoamiIdentityID(ev Event, opts ChannelWhoamiOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.DisplayName, opts.ProviderUserID, opts.ProviderHandle, opts.Role}, "|")
	return fmt.Sprintf("identity-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelWhoamiNotifyMessageID(ev Event, identityID string) string {
	seed := strings.Join([]string{eventID(ev), identityID}, "|")
	return fmt.Sprintf("gitclaw-channel-whoami-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func optionalChannelWhoamiHash(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return shortDocumentHash(value)
}

func renderChannelWhoamiNotificationBody(opts ChannelWhoamiOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel identity status.\n\n")
	fmt.Fprintf(&b, "Display name: %s\n", strings.TrimSpace(opts.DisplayName))
	fmt.Fprintf(&b, "Role: %s\n", strings.TrimSpace(opts.Role))
	b.WriteString("Identity record: pending GitHub review\n")
	b.WriteString("Contact card: not saved by this action; use /channels contact when a durable contact card is needed.\n")
	b.WriteString("Access: not granted or changed by this action.\n\n")
	b.WriteString("No access was granted by this action.")
	return strings.TrimSpace(b.String())
}
