package gitclaw

import (
	"fmt"
	"strings"
)

type ChannelSendActionRequest struct {
	Options             ChannelSendOptions
	Command             string
	Subcommand          string
	AutoMessageID       bool
	OutboundBodySHA     string
	OutboundBodyBytes   int
	OutboundBodyLines   int
	BodySource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
}

func IsChannelSendActionRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return isChannelSendActionFields(fields)
}

func isChannelSendActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "send", "say", "notify":
		return true
	default:
		return false
	}
}

func BuildChannelSendActionRequest(ev Event, cfg Config) (ChannelSendActionRequest, error) {
	fields, trailingBody, ok := channelSendActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSendActionRequest{}, fmt.Errorf("missing channel send command")
	}
	req := ChannelSendActionRequest{
		Options: ChannelSendOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		BodySource: "inline",
	}
	var bodyParts []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSendActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSendActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSendActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSendActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSendActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		case "--body":
			if i+1 >= len(fields) {
				return ChannelSendActionRequest{}, fmt.Errorf("--body requires a value")
			}
			bodyParts = append(bodyParts, fields[i+1:]...)
			i = len(fields)
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSendActionRequest{}, fmt.Errorf("unknown channel send argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			bodyParts = append(bodyParts, fields[i:]...)
			i = len(fields)
		}
	}

	body := strings.TrimSpace(strings.Join(bodyParts, " "))
	trailingBody = strings.TrimSpace(trailingBody)
	if trailingBody != "" {
		if body != "" {
			body += "\n" + trailingBody
		} else {
			body = trailingBody
			req.BodySource = "trailing-lines"
		}
	}
	req.Options.Body = body
	if strings.TrimSpace(req.Options.Body) == "" {
		return ChannelSendActionRequest{}, fmt.Errorf("missing outbound channel body")
	}
	if strings.TrimSpace(req.Options.MessageID) == "" {
		req.Options.MessageID = autoChannelSendActionMessageID(ev, req.Options.Route, req.Options.Channel, req.Options.Body)
		req.AutoMessageID = true
	}
	req.OutboundBodySHA = shortDocumentHash(req.Options.Body)
	req.OutboundBodyBytes = len(req.Options.Body)
	req.OutboundBodyLines = lineCount(req.Options.Body)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if strings.TrimSpace(req.Options.ThreadID) != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.MessageID)
	return req, nil
}

func channelSendActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSendActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func autoChannelSendActionMessageID(ev Event, route, channel, body string) string {
	seed := strings.Join([]string{eventID(ev), route, channel, body}, "|")
	return fmt.Sprintf("gitclaw-slash-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func RenderChannelSendActionReport(ev Event, req ChannelSendActionRequest, result ChannelSendResult) string {
	status := "queued"
	if result.Duplicate {
		status = "duplicate"
	}
	threadHash := req.RequestedThreadHash
	if result.ThreadHash != "" {
		threadHash = result.ThreadHash
	} else if strings.TrimSpace(req.Options.ThreadID) != "" {
		threadHash = shortDocumentHash(req.Options.ThreadID)
	}
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	messageHash := result.MessageHash
	if messageHash == "" {
		messageHash = req.RequestedMsgHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.OutboundBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Send Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_send_status: `%s`\n", status)
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
	fmt.Fprintf(&b, "- outbound_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- outbound_body_bytes: `%d`\n", req.OutboundBodyBytes)
	fmt.Fprintf(&b, "- outbound_body_lines: `%d`\n", req.OutboundBodyLines)
	fmt.Fprintf(&b, "- outbound_body_source: `%s`\n", req.BodySource)
	fmt.Fprintf(&b, "- raw_outbound_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_send_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued an outbound channel message as a `gitclaw:channel-outbound` comment on the canonical channel issue. Provider tokens, provider APIs, raw thread IDs, raw message IDs, and outbound message bodies are not included in this receipt.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateway reads pending work with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateway records sent messages with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate channel sends are suppressed by `channel + message_id`\n")
	return strings.TrimSpace(b.String())
}
