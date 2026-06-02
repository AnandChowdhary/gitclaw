package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelContactOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ContactID         string
	DisplayName       string
	ProviderUserID    string
	ProviderHandle    string
	ContactRole       string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelContactResult struct {
	ContactIssueNumber int
	ContactIssueURL    string
	ContactCreated     bool
	ContactDuplicate   bool
	Notification       ChannelSendResult
	RouteName          string
	RouteHash          string
	Channel            string
	ThreadHash         string
	MessageHash        string
	NotifyHash         string
}

type ChannelContactActionRequest struct {
	Options             ChannelContactOptions
	Command             string
	Subcommand          string
	AutoContactID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	DisplayNameSHA      string
	DisplayNameBytes    int
	DisplayNameLines    int
	ProviderUserIDSHA   string
	ProviderHandleSHA   string
	ContactRoleSHA      string
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

type channelContactDetails struct {
	DisplayName string
	ContactRole string
	Notes       string
}

func IsChannelContactActionRequest(ev Event, cfg Config) bool {
	return isChannelContactActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelContactActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "contact", "contact-card", "save-contact", "person", "profile", "identity", "member":
		return true
	default:
		return false
	}
}

func BuildChannelContactActionRequest(ev Event, cfg Config) (ChannelContactActionRequest, error) {
	fields, trailing, ok := channelContactActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelContactActionRequest{}, fmt.Errorf("missing channel contact command")
	}
	req := ChannelContactActionRequest{
		Options: ChannelContactOptions{
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
				return ChannelContactActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--contact-id", "--profile-id", "--person-id", "--id":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ContactID = cleanChannelContactID(fields[i+1])
			i++
		case "--display-name", "--name":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.DisplayName = fields[i+1]
			i++
		case "--provider-user-id", "--user-id", "--member-id":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProviderUserID = fields[i+1]
			i++
		case "--handle", "--username", "--provider-handle":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProviderHandle = fields[i+1]
			i++
		case "--role", "--contact-role":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ContactRole = fields[i+1]
			i++
		case "--notes":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("--notes requires a value")
			}
			req.Options.Notes = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelContactActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelContactActionRequest{}, fmt.Errorf("unknown channel contact argument %q", field)
			}
			if req.Options.ContactID == "" {
				req.Options.ContactID = cleanChannelContactID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelContactActionRequest{}, fmt.Errorf("unexpected channel contact argument %q", field)
		}
	}
	if err := applyChannelContactIssueTarget(ev, &req); err != nil {
		return ChannelContactActionRequest{}, err
	}
	details := parseChannelContactDetails(trailing, ev)
	if strings.TrimSpace(req.Options.DisplayName) == "" {
		req.Options.DisplayName = details.DisplayName
	}
	if strings.TrimSpace(req.Options.ContactRole) == "" {
		req.Options.ContactRole = details.ContactRole
	}
	if strings.TrimSpace(req.Options.Notes) == "" {
		req.Options.Notes = details.Notes
	}
	if strings.TrimSpace(req.Options.ContactID) == "" {
		req.Options.ContactID = autoChannelContactID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.DisplayName, req.Options.ProviderUserID, req.Options.ProviderHandle, req.Options.ContactRole)
		req.AutoContactID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelContactNotifyMessageID(ev, req.Options.ContactID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelContactOptions(req.Options)
	if err := validateChannelContactActionRequestOptions(req.Options); err != nil {
		return ChannelContactActionRequest{}, err
	}
	req.DisplayNameSHA = shortDocumentHash(req.Options.DisplayName)
	req.DisplayNameBytes = len(req.Options.DisplayName)
	req.DisplayNameLines = lineCount(req.Options.DisplayName)
	req.ProviderUserIDSHA = optionalChannelContactHash(req.Options.ProviderUserID)
	req.ProviderHandleSHA = optionalChannelContactHash(req.Options.ProviderHandle)
	req.ContactRoleSHA = shortDocumentHash(req.Options.ContactRole)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelContactNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelContact(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelContactOptions) (ChannelContactResult, error) {
	opts = normalizeChannelContactOptions(opts)
	var err error
	opts, err = applyChannelContactRoute(cfg, opts)
	if err != nil {
		return ChannelContactResult{}, err
	}
	if err := validateChannelContactOptions(opts); err != nil {
		return ChannelContactResult{}, err
	}
	contactIssue, created, duplicate, err := findOrCreateChannelContactIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelContactResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelContactNotificationBody(opts, contactIssue.Number, issueURL(opts.Repo, contactIssue.Number)),
	})
	if err != nil {
		return ChannelContactResult{}, fmt.Errorf("queue channel contact notification: %w", err)
	}
	return ChannelContactResult{
		ContactIssueNumber: contactIssue.Number,
		ContactIssueURL:    issueURL(opts.Repo, contactIssue.Number),
		ContactCreated:     created,
		ContactDuplicate:   duplicate,
		Notification:       notification,
		RouteName:          opts.Route,
		RouteHash:          channelRouteHash(opts.Route),
		Channel:            opts.Channel,
		ThreadHash:         shortDocumentHash(opts.ThreadID),
		MessageHash:        shortDocumentHash(opts.SourceMessageID),
		NotifyHash:         shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelContactActionReport(ev Event, req ChannelContactActionRequest, result ChannelContactResult) string {
	status := "opened"
	switch {
	case result.ContactDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ContactDuplicate:
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
	b.WriteString("## GitClaw Channel Contact Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_contact_status: `%s`\n", status)
	fmt.Fprintf(&b, "- contact_issue: `#%d`\n", result.ContactIssueNumber)
	fmt.Fprintf(&b, "- contact_issue_url: `%s`\n", result.ContactIssueURL)
	fmt.Fprintf(&b, "- contact_issue_created: `%t`\n", result.ContactCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ContactDuplicate)
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
	fmt.Fprintf(&b, "- contact_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ContactID))
	fmt.Fprintf(&b, "- contact_id_auto: `%t`\n", req.AutoContactID)
	fmt.Fprintf(&b, "- display_name_sha256_12: `%s`\n", req.DisplayNameSHA)
	fmt.Fprintf(&b, "- display_name_bytes: `%d`\n", req.DisplayNameBytes)
	fmt.Fprintf(&b, "- display_name_lines: `%d`\n", req.DisplayNameLines)
	fmt.Fprintf(&b, "- provider_user_id_sha256_12: `%s`\n", noneIfEmpty(req.ProviderUserIDSHA))
	fmt.Fprintf(&b, "- provider_handle_sha256_12: `%s`\n", noneIfEmpty(req.ProviderHandleSHA))
	fmt.Fprintf(&b, "- contact_role_sha256_12: `%s`\n", req.ContactRoleSHA)
	fmt.Fprintf(&b, "- notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- permission_grant_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- allowlist_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- pairing_code_issued: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_contact_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_display_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_provider_user_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_provider_handle_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_contact_role_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_contact_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw saved a GitHub-native contact card for a channel-origin identity, then queued a provider-facing contact-card link back to the mirrored thread. This action does not grant access, mutate allowlists, issue pairing codes, call provider APIs, or call a model. The source receipt keeps provider user identifiers, display names, roles, notes, IDs, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the contact-card notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent contact-card links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate contact card issues are suppressed by `contact_id`; duplicate contact-card notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the contact issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelContactIssueBody(opts ChannelContactOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-contact contact_id=\"%s\" channel=\"%s\" display_name_sha256_12=\"%s\" provider_user_id_sha256_12=\"%s\" provider_handle_sha256_12=\"%s\" contact_role_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ContactID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.DisplayName), optionalChannelContactHash(opts.ProviderUserID), optionalChannelContactHash(opts.ProviderHandle), shortDocumentHash(opts.ContactRole), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel contact card.\n\n")
	fmt.Fprintf(&b, "- contact_id: %s\n", opts.ContactID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- display_name: %s\n", opts.DisplayName)
	fmt.Fprintf(&b, "- provider_user_id_sha256_12: %s\n", noneIfEmpty(optionalChannelContactHash(opts.ProviderUserID)))
	fmt.Fprintf(&b, "- provider_handle_sha256_12: %s\n", noneIfEmpty(optionalChannelContactHash(opts.ProviderHandle)))
	fmt.Fprintf(&b, "- contact_role: %s\n", opts.ContactRole)
	fmt.Fprintf(&b, "- contact_mode: github-issue-contact-card\n")
	fmt.Fprintf(&b, "- permission_grant_performed: false\n")
	fmt.Fprintf(&b, "- allowlist_mutation_performed: false\n")
	fmt.Fprintf(&b, "- pairing_code_issued: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_provider_user_id_included: false\n")
	fmt.Fprintf(&b, "- raw_provider_handle_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
		b.WriteString("\n\n")
	}
	b.WriteString("Use this issue as the durable GitHub home for reviewing the channel-origin identity. This action did not grant access, mutate allowlists, or issue pairing codes.")
	return strings.TrimSpace(b.String())
}

func channelContactActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelContactActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelContactIssueTarget(ev Event, req *ChannelContactActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel contact requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelContactDetails(trailing string, ev Event) channelContactDetails {
	details := channelContactDetails{
		DisplayName: fmt.Sprintf("Channel contact from issue #%d", ev.Issue.Number),
		ContactRole: "user",
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
			details.ContactRole = strings.TrimSpace(trimmed[len("role:"):])
			section = ""
		case strings.HasPrefix(lower, "contact role:"):
			details.ContactRole = strings.TrimSpace(trimmed[len("contact role:"):])
			section = ""
		case strings.HasPrefix(lower, "contact_role:"):
			details.ContactRole = strings.TrimSpace(trimmed[len("contact_role:"):])
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
		details.DisplayName = fmt.Sprintf("Channel contact from issue #%d", ev.Issue.Number)
	}
	if strings.TrimSpace(details.ContactRole) == "" {
		details.ContactRole = "user"
	}
	details.Notes = strings.TrimSpace(strings.Join(notesLines, "\n"))
	return details
}

func normalizeChannelContactOptions(opts ChannelContactOptions) ChannelContactOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ContactID = cleanChannelContactID(opts.ContactID)
	opts.DisplayName = strings.TrimSpace(opts.DisplayName)
	opts.ProviderUserID = strings.TrimSpace(opts.ProviderUserID)
	opts.ProviderHandle = strings.TrimSpace(opts.ProviderHandle)
	opts.ContactRole = strings.ToLower(strings.TrimSpace(opts.ContactRole))
	if opts.ContactRole == "" {
		opts.ContactRole = "user"
	}
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelContactRoute(cfg Config, opts ChannelContactOptions) (ChannelContactOptions, error) {
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

func validateChannelContactOptions(opts ChannelContactOptions) error {
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
	if opts.ContactID == "" {
		return fmt.Errorf("missing contact id")
	}
	if opts.DisplayName == "" {
		return fmt.Errorf("missing contact display name")
	}
	if opts.ContactRole == "" {
		return fmt.Errorf("missing contact role")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing contact source issue")
	}
	return nil
}

func validateChannelContactActionRequestOptions(opts ChannelContactOptions) error {
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
	if opts.ContactID == "" {
		return fmt.Errorf("missing contact id")
	}
	if opts.DisplayName == "" {
		return fmt.Errorf("missing contact display name")
	}
	if opts.ContactRole == "" {
		return fmt.Errorf("missing contact role")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing contact source issue")
	}
	return nil
}

func findOrCreateChannelContactIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelContactOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel contact issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelContactMatches(issue.Body, opts.ContactID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelContactIssueTitle(opts), RenderChannelContactIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel contact issue: %w", err)
	}
	return issue, true, false, nil
}

func channelContactIssueTitle(opts ChannelContactOptions) string {
	displayName := strings.ReplaceAll(strings.TrimSpace(opts.DisplayName), "\n", " ")
	if displayName == "" {
		displayName = opts.ContactID
	}
	if len(displayName) > 80 {
		displayName = strings.TrimSpace(displayName[:80])
	}
	return "GitClaw channel contact: " + displayName
}

func channelContactMatches(body, contactID string) bool {
	return HasChannelContactMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`contact_id="%s"`, escapeMarkerValue(cleanChannelContactID(contactID))))
}

func cleanChannelContactID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelContactID(ev Event, channel, threadID, sourceMessageID, displayName, providerUserID, providerHandle, contactRole string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, displayName, providerUserID, providerHandle, contactRole}, "|")
	return fmt.Sprintf("contact-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelContactNotifyMessageID(ev Event, contactID string) string {
	seed := strings.Join([]string{eventID(ev), contactID}, "|")
	return fmt.Sprintf("gitclaw-channel-contact-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func optionalChannelContactHash(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return shortDocumentHash(value)
}

func renderChannelContactNotificationBody(opts ChannelContactOptions, contactIssueNumber int, contactIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel contact card saved.\n\n")
	if contactIssueNumber > 0 {
		fmt.Fprintf(&b, "Contact card: #%d\n", contactIssueNumber)
	}
	if contactIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", contactIssueURL)
	}
	fmt.Fprintf(&b, "Display name: %s\n", strings.TrimSpace(opts.DisplayName))
	fmt.Fprintf(&b, "Contact role: %s\n", strings.TrimSpace(opts.ContactRole))
	b.WriteString("\nNo access was granted by this action. Review the contact in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
