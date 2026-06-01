package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelProbeActionRequest struct {
	Options            ChannelSendOptions
	Command            string
	Subcommand         string
	AutoMessageID      bool
	RequestedRouteHash string
	RequestedMsgHash   string
	ProbeBodySHA       string
	ProbeBodyBytes     int
	ProbeBodyLines     int
	SourceSHA          string
	SourceBytes        int
	SourceLines        int
	SourceKind         string
	SourceCommentID    int64
	OperatorNoteSHA    string
	OperatorNoteBytes  int
	OperatorNoteLines  int
}

func IsChannelProbeActionRequest(ev Event, cfg Config) bool {
	return isChannelProbeActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelProbeActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "probe", "test", "ping", "check":
		return true
	default:
		return false
	}
}

func BuildChannelProbeActionRequest(ev Event, cfg Config) (ChannelProbeActionRequest, error) {
	fields, trailingBody, ok := channelProbeActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelProbeActionRequest{}, fmt.Errorf("missing channel probe command")
	}
	sourceText := activeRequestText(ev)
	req := ChannelProbeActionRequest{
		Options: ChannelSendOptions{
			Repo:   ev.Repo,
			Author: "gitclaw:probe",
		},
		Command:     fields[0],
		Subcommand:  strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		SourceSHA:   shortDocumentHash(sourceText),
		SourceBytes: len(sourceText),
		SourceLines: lineCount(sourceText),
		SourceKind:  "issue",
	}
	if ev.Comment != nil {
		req.SourceKind = "comment"
		req.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelProbeActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--message-id", "--id":
			if i+1 >= len(fields) {
				return ChannelProbeActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelProbeActionRequest{}, fmt.Errorf("unknown channel probe argument %q", field)
			}
			if req.Options.Route == "" {
				req.Options.Route = field
				continue
			}
		}
	}
	req.Options.Route = cleanChannelRouteName(req.Options.Route)
	if req.Options.Route == "" {
		return ChannelProbeActionRequest{}, fmt.Errorf("missing channel probe route")
	}
	if strings.TrimSpace(req.Options.MessageID) == "" {
		req.Options.MessageID = autoChannelProbeActionMessageID(ev, req.Options.Route, sourceText)
		req.AutoMessageID = true
	}
	req.Options.MessageID = strings.TrimSpace(req.Options.MessageID)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	req.RequestedMsgHash = shortDocumentHash(req.Options.MessageID)
	req.Options.Body = RenderChannelProbeOutboundBody(ev, req)
	req.ProbeBodySHA = shortDocumentHash(req.Options.Body)
	req.ProbeBodyBytes = len(req.Options.Body)
	req.ProbeBodyLines = lineCount(req.Options.Body)
	trailingBody = strings.TrimSpace(trailingBody)
	if trailingBody != "" {
		req.OperatorNoteSHA = shortDocumentHash(trailingBody)
		req.OperatorNoteBytes = len(trailingBody)
		req.OperatorNoteLines = lineCount(trailingBody)
	}
	return req, nil
}

func channelProbeActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelProbeActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func autoChannelProbeActionMessageID(ev Event, route, sourceText string) string {
	seed := strings.Join([]string{eventID(ev), route, sourceText}, "|")
	return fmt.Sprintf("gitclaw-probe-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func RunChannelProbeAction(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelProbeActionRequest) (ChannelSendResult, error) {
	return RunChannelSend(ctx, cfg, github, req.Options)
}

func RenderChannelProbeOutboundBody(ev Event, req ChannelProbeActionRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel route probe.\n\n")
	fmt.Fprintf(&b, "- source_issue: #%d\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(ev.Repo, ev.Issue.Number))
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- route_sha256_12: %s\n", req.RequestedRouteHash)
	fmt.Fprintf(&b, "- message_id_sha256_12: %s\n", shortDocumentHash(req.Options.MessageID))
	b.WriteString("- generated_without_model_call: true\n")
	b.WriteString("- provider_delivery_performed: false\n")
	b.WriteString("- provider_delivery_strategy: channel-outbox + channel-delivery\n")
	b.WriteString("\nReply here only if this route probe reached the expected provider thread.")
	return strings.TrimSpace(b.String())
}

func RenderChannelProbeActionReport(ev Event, req ChannelProbeActionRequest, result ChannelSendResult) string {
	status := "queued"
	if result.Duplicate {
		status = "duplicate"
	}
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	threadHash := result.ThreadHash
	messageHash := result.MessageHash
	if messageHash == "" {
		messageHash = req.RequestedMsgHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.ProbeBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Probe Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_probe_status: `%s`\n", status)
	fmt.Fprintf(&b, "- target_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- target_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- outbound_comment_id: `%d`\n", result.CommentID)
	fmt.Fprintf(&b, "- target_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- probe_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- probe_body_bytes: `%d`\n", req.ProbeBodyBytes)
	fmt.Fprintf(&b, "- probe_body_lines: `%d`\n", req.ProbeBodyLines)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- operator_note_sha256_12: `%s`\n", noneIfEmpty(req.OperatorNoteSHA))
	fmt.Fprintf(&b, "- operator_note_bytes: `%d`\n", req.OperatorNoteBytes)
	fmt.Fprintf(&b, "- operator_note_lines: `%d`\n", req.OperatorNoteLines)
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_ids_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_message_ids_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_probe_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_probe_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing route probe as a `gitclaw:channel-outbound` comment on the canonical channel issue. The probe is meant to test reviewed route resolution, pending outbox visibility, and delivery receipts without calling provider APIs from the issue action.\n\n")
	b.WriteString("### Probe Path\n")
	b.WriteString("- provider gateway reads the pending probe with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateway records successful send with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate probes are suppressed by `channel + message_id`\n")
	return strings.TrimSpace(b.String())
}
