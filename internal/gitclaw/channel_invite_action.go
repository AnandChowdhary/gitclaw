package gitclaw

import (
	"fmt"
	"strings"
)

type ChannelInviteActionRequest struct {
	Options           ChannelBroadcastOptions
	Command           string
	Subcommand        string
	AutoMessageID     bool
	InviteNoteSHA     string
	InviteNoteBytes   int
	InviteNoteLines   int
	InviteNoteSource  string
	OutboundBodySHA   string
	OutboundBodyBytes int
	OutboundBodyLines int
	RoutesSHA         string
	RouteCount        int
}

func IsChannelInviteActionRequest(ev Event, cfg Config) bool {
	return isChannelInviteActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelInviteActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "invite", "share", "summon":
		return true
	default:
		return false
	}
}

func BuildChannelInviteActionRequest(ev Event, cfg Config) (ChannelInviteActionRequest, error) {
	fields, trailingNote, ok := channelInviteActionFieldsAndTrailingNote(ev, cfg)
	if !ok {
		return ChannelInviteActionRequest{}, fmt.Errorf("missing channel invite command")
	}
	req := ChannelInviteActionRequest{
		Options: ChannelBroadcastOptions{
			Repo: ev.Repo,
		},
		Command:          fields[0],
		Subcommand:       strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		InviteNoteSource: "none",
	}
	var noteParts []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelInviteActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--routes":
			if i+1 >= len(fields) {
				return ChannelInviteActionRequest{}, fmt.Errorf("--routes requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--message-id", "--id":
			if i+1 >= len(fields) {
				return ChannelInviteActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelInviteActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelInviteActionRequest{}, fmt.Errorf("--note requires a value")
			}
			noteParts = append(noteParts, fields[i+1:]...)
			i = len(fields)
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelInviteActionRequest{}, fmt.Errorf("unknown channel invite argument %q", field)
			}
			if len(req.Options.Routes) == 0 {
				req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(field)...)
				continue
			}
			noteParts = append(noteParts, fields[i:]...)
			i = len(fields)
		}
	}

	note := strings.TrimSpace(strings.Join(noteParts, " "))
	trailingNote = strings.TrimSpace(trailingNote)
	switch {
	case note != "" && trailingNote != "":
		note += "\n" + trailingNote
		req.InviteNoteSource = "inline+trailing-lines"
	case note != "":
		req.InviteNoteSource = "inline"
	case trailingNote != "":
		note = trailingNote
		req.InviteNoteSource = "trailing-lines"
	}

	req.Options.Routes = normalizeChannelBroadcastRoutes(req.Options.Routes)
	if len(req.Options.Routes) == 0 {
		return ChannelInviteActionRequest{}, fmt.Errorf("missing invite routes")
	}
	req.Options.Body = renderChannelInviteOutboundBody(ev, note)
	if strings.TrimSpace(req.Options.MessageID) == "" {
		req.Options.MessageID = autoChannelInviteActionMessageID(ev, req.Options.Routes, req.Options.Body)
		req.AutoMessageID = true
	}
	req.InviteNoteSHA = shortDocumentHash(note)
	req.InviteNoteBytes = len(note)
	req.InviteNoteLines = lineCount(note)
	req.OutboundBodySHA = shortDocumentHash(req.Options.Body)
	req.OutboundBodyBytes = len(req.Options.Body)
	req.OutboundBodyLines = lineCount(req.Options.Body)
	req.RoutesSHA = channelBroadcastRoutesHash(req.Options.Routes)
	req.RouteCount = len(req.Options.Routes)
	return req, nil
}

func RenderChannelInviteActionReport(ev Event, req ChannelInviteActionRequest, result ChannelBroadcastResult) string {
	status := "queued"
	switch {
	case result.Queued == 0 && result.Duplicates > 0:
		status = "duplicate"
	case result.Queued > 0 && result.Duplicates > 0:
		status = "partially-queued"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Invite Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_invite_status: `%s`\n", status)
	fmt.Fprintf(&b, "- invite_routes: `%d`\n", req.RouteCount)
	fmt.Fprintf(&b, "- invite_queued: `%d`\n", result.Queued)
	fmt.Fprintf(&b, "- invite_duplicates: `%d`\n", result.Duplicates)
	fmt.Fprintf(&b, "- target_issues_created: `%d`\n", result.Created)
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", req.RoutesSHA)
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.MessageID))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- invite_note_sha256_12: `%s`\n", req.InviteNoteSHA)
	fmt.Fprintf(&b, "- invite_note_bytes: `%d`\n", req.InviteNoteBytes)
	fmt.Fprintf(&b, "- invite_note_lines: `%d`\n", req.InviteNoteLines)
	fmt.Fprintf(&b, "- invite_note_source: `%s`\n", req.InviteNoteSource)
	fmt.Fprintf(&b, "- outbound_body_sha256_12: `%s`\n", req.OutboundBodySHA)
	fmt.Fprintf(&b, "- outbound_body_bytes: `%d`\n", req.OutboundBodyBytes)
	fmt.Fprintf(&b, "- outbound_body_lines: `%d`\n", req.OutboundBodyLines)
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_invite_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outbound_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_invite_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued an issue invitation on each reviewed route. The outbound channel body includes the source issue reference and optional note, but this receipt keeps route names, issue titles, notes, thread IDs, message IDs, and outbound bodies out of the source issue.\n\n")
	b.WriteString("### Destinations\n")
	if len(result.Destinations) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, destination := range result.Destinations {
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
	b.WriteString("\n### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending invitations with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent invitations with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate invitations are suppressed independently for each route by `channel + message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelInviteActionFieldsAndTrailingNote(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelInviteActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func renderChannelInviteOutboundBody(ev Event, note string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel invite\n\n")
	fmt.Fprintf(&b, "Issue: #%d %s\n", ev.Issue.Number, strings.TrimSpace(ev.Issue.Title))
	fmt.Fprintf(&b, "URL: %s\n", issueURL(ev.Repo, ev.Issue.Number))
	note = strings.TrimSpace(note)
	if note != "" {
		b.WriteString("\nNote:\n")
		b.WriteString(note)
		b.WriteString("\n")
	}
	b.WriteString("\nReply in the linked GitHub issue or continue through the mirrored channel thread.")
	return strings.TrimSpace(b.String())
}

func autoChannelInviteActionMessageID(ev Event, routes []string, body string) string {
	seed := strings.Join([]string{eventID(ev), strings.Join(normalizeChannelBroadcastRoutes(routes), ","), body}, "|")
	return fmt.Sprintf("gitclaw-invite-%s-%s", eventID(ev), shortDocumentHash(seed))
}
