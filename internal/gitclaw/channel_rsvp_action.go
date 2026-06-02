package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRsvpOptions struct {
	Repo              string
	RsvpID            string
	SourceIssueNumber int
	SourceCommentID   int64
	Title             string
	When              string
	Where             string
	Host              string
	Details           string
	Routes            []string
	MessageID         string
	Author            string
}

type ChannelRsvpResult struct {
	RsvpIssueNumber int
	RsvpIssueURL    string
	RsvpCreated     bool
	Broadcast       ChannelBroadcastResult
}

type ChannelRsvpActionRequest struct {
	Options               ChannelRsvpOptions
	Command               string
	Subcommand            string
	AutoRsvpID            bool
	AutoMessageID         bool
	TitleSHA              string
	TitleBytes            int
	TitleLines            int
	WhenSHA               string
	WhereSHA              string
	HostSHA               string
	DetailsSHA            string
	DetailsBytes          int
	DetailsLines          int
	RsvpSource            string
	RoutesSHA             string
	RouteCount            int
	OutboundBodySHA       string
	OutboundBodyBytes     int
	OutboundBodyLineCount int
}

func IsChannelRsvpActionRequest(ev Event, cfg Config) bool {
	return isChannelRsvpActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelRsvpActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rsvp", "invite-rsvp", "event", "meetup":
		return true
	default:
		return false
	}
}

func BuildChannelRsvpActionRequest(ev Event, cfg Config) (ChannelRsvpActionRequest, error) {
	fields, trailingBody, ok := channelRsvpActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRsvpActionRequest{}, fmt.Errorf("missing channel rsvp command")
	}
	req := ChannelRsvpActionRequest{
		Options: ChannelRsvpOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RsvpSource: "trailing-lines",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--routes":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("--routes requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--rsvp-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("--rsvp-id requires a value")
			}
			req.Options.RsvpID = cleanChannelRsvpID(fields[i+1])
			i++
		case "--message-id":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		case "--title", "--event":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Title = fields[i+1]
			i++
		case "--when", "--time", "--at":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.When = fields[i+1]
			i++
		case "--where", "--location":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Where = fields[i+1]
			i++
		case "--host", "--organizer":
			if i+1 >= len(fields) {
				return ChannelRsvpActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Host = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRsvpActionRequest{}, fmt.Errorf("unknown channel rsvp argument %q", field)
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(field)...)
		}
	}

	req.Options.Routes = normalizeChannelBroadcastRoutes(req.Options.Routes)
	if len(req.Options.Routes) == 0 {
		return ChannelRsvpActionRequest{}, fmt.Errorf("missing rsvp routes")
	}
	title, when, where, host, details := parseChannelRsvpDetails(trailingBody, ev)
	if req.Options.Title == "" {
		req.Options.Title = title
	}
	if req.Options.When == "" {
		req.Options.When = when
	}
	if req.Options.Where == "" {
		req.Options.Where = where
	}
	if req.Options.Host == "" {
		req.Options.Host = host
	}
	req.Options.Details = details
	if req.Options.RsvpID == "" {
		req.Options.RsvpID = autoChannelRsvpID(ev, req.Options.Routes, req.Options.Title, req.Options.When, req.Options.Where)
		req.AutoRsvpID = true
	}
	if req.Options.MessageID == "" {
		req.Options.MessageID = autoChannelRsvpMessageID(ev, req.Options.RsvpID, req.Options.Routes)
		req.AutoMessageID = true
	}
	if err := validateChannelRsvpOptions(req.Options); err != nil {
		return ChannelRsvpActionRequest{}, err
	}
	outboundPreview := renderChannelRsvpOutboundBody(req.Options, 0, issueURL(ev.Repo, 0))
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.WhenSHA = shortDocumentHash(req.Options.When)
	req.WhereSHA = shortDocumentHash(req.Options.Where)
	req.HostSHA = shortDocumentHash(req.Options.Host)
	req.DetailsSHA = shortDocumentHash(req.Options.Details)
	req.DetailsBytes = len(req.Options.Details)
	req.DetailsLines = lineCount(req.Options.Details)
	req.RoutesSHA = channelBroadcastRoutesHash(req.Options.Routes)
	req.RouteCount = len(req.Options.Routes)
	req.OutboundBodySHA = shortDocumentHash(outboundPreview)
	req.OutboundBodyBytes = len(outboundPreview)
	req.OutboundBodyLineCount = lineCount(outboundPreview)
	return req, nil
}

func RunChannelRsvp(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRsvpOptions) (ChannelRsvpResult, error) {
	opts = normalizeChannelRsvpOptions(opts)
	if err := validateChannelRsvpOptions(opts); err != nil {
		return ChannelRsvpResult{}, err
	}
	rsvp, created, err := findOrCreateChannelRsvpIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelRsvpResult{}, err
	}
	broadcastOpts := ChannelBroadcastOptions{
		Repo:      opts.Repo,
		Routes:    opts.Routes,
		MessageID: opts.MessageID,
		Author:    opts.Author,
		Body:      renderChannelRsvpOutboundBody(opts, rsvp.Number, issueURL(opts.Repo, rsvp.Number)),
	}
	broadcast, err := RunChannelBroadcast(ctx, cfg, github, broadcastOpts)
	if err != nil {
		return ChannelRsvpResult{}, err
	}
	return ChannelRsvpResult{
		RsvpIssueNumber: rsvp.Number,
		RsvpIssueURL:    issueURL(opts.Repo, rsvp.Number),
		RsvpCreated:     created,
		Broadcast:       broadcast,
	}, nil
}

func RenderChannelRsvpActionReport(ev Event, req ChannelRsvpActionRequest, result ChannelRsvpResult) string {
	status := "queued"
	switch {
	case !result.RsvpCreated && result.Broadcast.Queued == 0 && result.Broadcast.Duplicates > 0:
		status = "duplicate"
	case result.Broadcast.Queued > 0 && result.Broadcast.Duplicates > 0:
		status = "partially-queued"
	}
	outboundBody := renderChannelRsvpOutboundBody(req.Options, result.RsvpIssueNumber, result.RsvpIssueURL)
	outboundBodySHA := shortDocumentHash(outboundBody)
	outboundBodyBytes := len(outboundBody)
	outboundBodyLines := lineCount(outboundBody)
	var b strings.Builder
	b.WriteString("## GitClaw Channel RSVP Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_rsvp_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rsvp_issue: `#%d`\n", result.RsvpIssueNumber)
	fmt.Fprintf(&b, "- rsvp_issue_url: `%s`\n", result.RsvpIssueURL)
	fmt.Fprintf(&b, "- rsvp_issue_created: `%t`\n", result.RsvpCreated)
	fmt.Fprintf(&b, "- rsvp_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.RsvpID))
	fmt.Fprintf(&b, "- rsvp_id_auto: `%t`\n", req.AutoRsvpID)
	fmt.Fprintf(&b, "- rsvp_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- rsvp_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- rsvp_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- rsvp_when_sha256_12: `%s`\n", noneIfEmpty(req.WhenSHA))
	fmt.Fprintf(&b, "- rsvp_where_sha256_12: `%s`\n", noneIfEmpty(req.WhereSHA))
	fmt.Fprintf(&b, "- rsvp_host_sha256_12: `%s`\n", noneIfEmpty(req.HostSHA))
	fmt.Fprintf(&b, "- rsvp_details_sha256_12: `%s`\n", noneIfEmpty(req.DetailsSHA))
	fmt.Fprintf(&b, "- rsvp_details_bytes: `%d`\n", req.DetailsBytes)
	fmt.Fprintf(&b, "- rsvp_details_lines: `%d`\n", req.DetailsLines)
	fmt.Fprintf(&b, "- rsvp_source: `%s`\n", req.RsvpSource)
	fmt.Fprintf(&b, "- rsvp_routes: `%d`\n", req.RouteCount)
	fmt.Fprintf(&b, "- rsvp_invites_queued: `%d`\n", result.Broadcast.Queued)
	fmt.Fprintf(&b, "- rsvp_invite_duplicates: `%d`\n", result.Broadcast.Duplicates)
	fmt.Fprintf(&b, "- target_issues_created: `%d`\n", result.Broadcast.Created)
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", req.RoutesSHA)
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.MessageID))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- outbound_body_sha256_12: `%s`\n", outboundBodySHA)
	fmt.Fprintf(&b, "- outbound_body_bytes: `%d`\n", outboundBodyBytes)
	fmt.Fprintf(&b, "- outbound_body_lines: `%d`\n", outboundBodyLines)
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rsvp_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_when_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_where_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_host_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_details_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outbound_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_rsvp_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a GitHub RSVP issue, then queued one provider-facing RSVP card per reviewed route. The RSVP issue contains the human-readable event details; this source receipt keeps route names, RSVP IDs, event text, thread IDs, message IDs, and outbound bodies out of band.\n\n")
	b.WriteString("### Destinations\n")
	if len(result.Broadcast.Destinations) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, destination := range result.Broadcast.Destinations {
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
	b.WriteString("- provider gateways read pending RSVP cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent RSVP cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate RSVP cards are suppressed independently for each route by `channel + message_id`\n")
	return strings.TrimSpace(b.String())
}

func findOrCreateChannelRsvpIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRsvpOptions) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list channel rsvp issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelRsvpMatches(issue.Body, opts.RsvpID) {
			return issue, false, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelRsvpIssueTitle(opts), RenderChannelRsvpIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, fmt.Errorf("create channel rsvp issue: %w", err)
	}
	return issue, true, nil
}

func RenderChannelRsvpIssueBody(opts ChannelRsvpOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-rsvp rsvp_id=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" routes_sha256_12=\"%s\" details_sha256_12=\"%s\" -->\n", escapeMarkerValue(opts.RsvpID), opts.SourceIssueNumber, opts.SourceCommentID, escapeMarkerValue(channelBroadcastRoutesHash(opts.Routes)), escapeMarkerValue(shortDocumentHash(opts.Details)))
	b.WriteString("GitClaw channel RSVP.\n\n")
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: `%s`\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- rsvp_id_sha256_12: `%s`\n", shortDocumentHash(opts.RsvpID))
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", channelBroadcastRoutesHash(opts.Routes))
	fmt.Fprintf(&b, "- route_count: `%d`\n", len(normalizeChannelBroadcastRoutes(opts.Routes)))
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n\n", false)
	b.WriteString("## Event\n\n")
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.When) != "" {
		fmt.Fprintf(&b, "When: %s\n", strings.TrimSpace(opts.When))
	}
	if strings.TrimSpace(opts.Where) != "" {
		fmt.Fprintf(&b, "Where: %s\n", strings.TrimSpace(opts.Where))
	}
	if strings.TrimSpace(opts.Host) != "" {
		fmt.Fprintf(&b, "Host: %s\n", strings.TrimSpace(opts.Host))
	}
	if strings.TrimSpace(opts.Details) != "" {
		b.WriteString("\n## Details\n\n")
		b.WriteString(strings.TrimSpace(opts.Details))
		b.WriteString("\n")
	}
	b.WriteString("\nRSVP by commenting `yes`, `no`, or `maybe`. Participants can continue here from GitHub, or through a mirrored channel thread invited into this RSVP.")
	return strings.TrimSpace(b.String())
}

func channelRsvpActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRsvpActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func renderChannelRsvpOutboundBody(opts ChannelRsvpOptions, issueNumber int, url string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel RSVP\n\n")
	if issueNumber > 0 {
		fmt.Fprintf(&b, "RSVP: #%d\n", issueNumber)
	}
	fmt.Fprintf(&b, "URL: %s\n", url)
	fmt.Fprintf(&b, "Source: #%d\n", opts.SourceIssueNumber)
	b.WriteString("\nEvent:\n")
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.When) != "" {
		fmt.Fprintf(&b, "When: %s\n", strings.TrimSpace(opts.When))
	}
	if strings.TrimSpace(opts.Where) != "" {
		fmt.Fprintf(&b, "Where: %s\n", strings.TrimSpace(opts.Where))
	}
	if strings.TrimSpace(opts.Host) != "" {
		fmt.Fprintf(&b, "Host: %s\n", strings.TrimSpace(opts.Host))
	}
	if strings.TrimSpace(opts.Details) != "" {
		b.WriteString("\nDetails:\n")
		b.WriteString(strings.TrimSpace(opts.Details))
		b.WriteString("\n")
	}
	b.WriteString("\nReply yes/no/maybe in the linked GitHub RSVP or continue through the mirrored channel thread.")
	return strings.TrimSpace(b.String())
}

func parseChannelRsvpDetails(trailing string, ev Event) (string, string, string, string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	defaultTitle := fmt.Sprintf("RSVP for issue #%d", ev.Issue.Number)
	var title string
	var when string
	var where string
	var host string
	var details []string
	inDetails := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inDetails && len(details) > 0 {
				details = append(details, "")
			}
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "title:"):
			inDetails = false
			title = strings.TrimSpace(trimmed[len("title:"):])
		case strings.HasPrefix(lower, "event:"):
			inDetails = false
			title = strings.TrimSpace(trimmed[len("event:"):])
		case strings.HasPrefix(lower, "when:"):
			inDetails = false
			when = strings.TrimSpace(trimmed[len("when:"):])
		case strings.HasPrefix(lower, "time:"):
			inDetails = false
			when = strings.TrimSpace(trimmed[len("time:"):])
		case strings.HasPrefix(lower, "where:"):
			inDetails = false
			where = strings.TrimSpace(trimmed[len("where:"):])
		case strings.HasPrefix(lower, "location:"):
			inDetails = false
			where = strings.TrimSpace(trimmed[len("location:"):])
		case strings.HasPrefix(lower, "host:"):
			inDetails = false
			host = strings.TrimSpace(trimmed[len("host:"):])
		case strings.HasPrefix(lower, "organizer:"):
			inDetails = false
			host = strings.TrimSpace(trimmed[len("organizer:"):])
		case strings.HasPrefix(lower, "details:"):
			inDetails = true
			value := strings.TrimSpace(trimmed[len("details:"):])
			if value != "" {
				details = append(details, value)
			}
		case strings.HasPrefix(lower, "notes:"):
			inDetails = true
			value := strings.TrimSpace(trimmed[len("notes:"):])
			if value != "" {
				details = append(details, value)
			}
		case inDetails:
			details = append(details, line)
		case title == "":
			title = trimmed
		default:
			details = append(details, line)
		}
	}
	if strings.TrimSpace(title) == "" {
		title = defaultTitle
	}
	return strings.TrimSpace(title), strings.TrimSpace(when), strings.TrimSpace(where), strings.TrimSpace(host), strings.TrimSpace(strings.Join(details, "\n"))
}

func normalizeChannelRsvpOptions(opts ChannelRsvpOptions) ChannelRsvpOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.RsvpID = cleanChannelRsvpID(opts.RsvpID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.When = strings.TrimSpace(opts.When)
	opts.Where = strings.TrimSpace(opts.Where)
	opts.Host = strings.TrimSpace(opts.Host)
	opts.Details = strings.TrimSpace(opts.Details)
	opts.Routes = normalizeChannelBroadcastRoutes(opts.Routes)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelRsvpOptions(opts ChannelRsvpOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.RsvpID == "" {
		return fmt.Errorf("missing rsvp id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing rsvp source issue")
	}
	if strings.TrimSpace(opts.Title) == "" {
		return fmt.Errorf("missing rsvp title")
	}
	if len(normalizeChannelBroadcastRoutes(opts.Routes)) == 0 {
		return fmt.Errorf("missing rsvp routes")
	}
	if strings.TrimSpace(opts.MessageID) == "" {
		return fmt.Errorf("missing rsvp message id")
	}
	return nil
}

func channelRsvpIssueTitle(opts ChannelRsvpOptions) string {
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = fmt.Sprintf("issue #%d", opts.SourceIssueNumber)
	}
	title = strings.ReplaceAll(title, "\n", " ")
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel RSVP: " + title
}

func channelRsvpMatches(body, rsvpID string) bool {
	return HasChannelRsvpMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`rsvp_id="%s"`, escapeMarkerValue(cleanChannelRsvpID(rsvpID))))
}

func cleanChannelRsvpID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelRsvpID(ev Event, routes []string, title, when, where string) string {
	seed := strings.Join([]string{eventID(ev), strings.Join(normalizeChannelBroadcastRoutes(routes), ","), title, when, where}, "|")
	return fmt.Sprintf("rsvp-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelRsvpMessageID(ev Event, rsvpID string, routes []string) string {
	seed := strings.Join([]string{eventID(ev), rsvpID, strings.Join(normalizeChannelBroadcastRoutes(routes), ",")}, "|")
	return fmt.Sprintf("gitclaw-rsvp-%s-%s", eventID(ev), shortDocumentHash(seed))
}
