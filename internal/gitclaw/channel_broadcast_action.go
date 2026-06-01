package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelBroadcastOptions struct {
	Repo      string
	Routes    []string
	MessageID string
	Author    string
	Body      string
}

type ChannelBroadcastDestinationResult struct {
	Index       int
	RouteHash   string
	IssueNumber int
	IssueURL    string
	CommentID   int64
	Created     bool
	Duplicate   bool
	Channel     string
	ThreadHash  string
	MessageHash string
	BodyHash    string
}

type ChannelBroadcastResult struct {
	Destinations []ChannelBroadcastDestinationResult
	Queued       int
	Duplicates   int
	Created      int
}

type ChannelBroadcastActionRequest struct {
	Options           ChannelBroadcastOptions
	Command           string
	Subcommand        string
	AutoMessageID     bool
	OutboundBodySHA   string
	OutboundBodyBytes int
	OutboundBodyLines int
	BodySource        string
	RoutesSHA         string
	RouteCount        int
}

func IsChannelBroadcastActionRequest(ev Event, cfg Config) bool {
	return isChannelBroadcastActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBroadcastActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "broadcast", "fanout", "announce":
		return true
	default:
		return false
	}
}

func BuildChannelBroadcastActionRequest(ev Event, cfg Config) (ChannelBroadcastActionRequest, error) {
	fields, trailingBody, ok := channelBroadcastActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBroadcastActionRequest{}, fmt.Errorf("missing channel broadcast command")
	}
	req := ChannelBroadcastActionRequest{
		Options: ChannelBroadcastOptions{
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
				return ChannelBroadcastActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--routes":
			if i+1 >= len(fields) {
				return ChannelBroadcastActionRequest{}, fmt.Errorf("--routes requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--message-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBroadcastActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBroadcastActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		case "--body":
			if i+1 >= len(fields) {
				return ChannelBroadcastActionRequest{}, fmt.Errorf("--body requires a value")
			}
			bodyParts = append(bodyParts, fields[i+1:]...)
			i = len(fields)
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBroadcastActionRequest{}, fmt.Errorf("unknown channel broadcast argument %q", field)
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(field)...)
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
	req.Options.Routes = normalizeChannelBroadcastRoutes(req.Options.Routes)
	if strings.TrimSpace(req.Options.Body) == "" {
		return ChannelBroadcastActionRequest{}, fmt.Errorf("missing outbound channel body")
	}
	if len(req.Options.Routes) == 0 {
		return ChannelBroadcastActionRequest{}, fmt.Errorf("missing broadcast routes")
	}
	if strings.TrimSpace(req.Options.MessageID) == "" {
		req.Options.MessageID = autoChannelBroadcastActionMessageID(ev, req.Options.Routes, req.Options.Body)
		req.AutoMessageID = true
	}
	req.OutboundBodySHA = shortDocumentHash(req.Options.Body)
	req.OutboundBodyBytes = len(req.Options.Body)
	req.OutboundBodyLines = lineCount(req.Options.Body)
	req.RoutesSHA = channelBroadcastRoutesHash(req.Options.Routes)
	req.RouteCount = len(req.Options.Routes)
	return req, nil
}

func RunChannelBroadcast(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBroadcastOptions) (ChannelBroadcastResult, error) {
	opts = normalizeChannelBroadcastOptions(opts)
	if err := validateChannelBroadcastOptions(opts); err != nil {
		return ChannelBroadcastResult{}, err
	}
	resolved := make([]ChannelSendOptions, 0, len(opts.Routes))
	for _, route := range opts.Routes {
		sendOpts := ChannelSendOptions{
			Repo:      opts.Repo,
			Route:     route,
			MessageID: opts.MessageID,
			Author:    opts.Author,
			Body:      opts.Body,
		}
		routeOpts, err := applyChannelSendRoute(cfg, sendOpts)
		if err != nil {
			return ChannelBroadcastResult{}, err
		}
		if err := validateChannelSendOptions(routeOpts); err != nil {
			return ChannelBroadcastResult{}, err
		}
		resolved = append(resolved, routeOpts)
	}

	result := ChannelBroadcastResult{}
	for i, sendOpts := range resolved {
		sendResult, err := RunChannelSend(ctx, cfg, github, sendOpts)
		if err != nil {
			return result, fmt.Errorf("broadcast route %d: %w", i+1, err)
		}
		destination := ChannelBroadcastDestinationResult{
			Index:       i + 1,
			RouteHash:   sendResult.RouteHash,
			IssueNumber: sendResult.IssueNumber,
			IssueURL:    sendResult.IssueURL,
			CommentID:   sendResult.CommentID,
			Created:     sendResult.Created,
			Duplicate:   sendResult.Duplicate,
			Channel:     sendResult.Channel,
			ThreadHash:  sendResult.ThreadHash,
			MessageHash: sendResult.MessageHash,
			BodyHash:    sendResult.BodyHash,
		}
		result.Destinations = append(result.Destinations, destination)
		if sendResult.Duplicate {
			result.Duplicates++
		} else {
			result.Queued++
		}
		if sendResult.Created {
			result.Created++
		}
	}
	return result, nil
}

func RenderChannelBroadcastActionReport(ev Event, req ChannelBroadcastActionRequest, result ChannelBroadcastResult) string {
	status := "queued"
	switch {
	case result.Queued == 0 && result.Duplicates > 0:
		status = "duplicate"
	case result.Queued > 0 && result.Duplicates > 0:
		status = "partially-queued"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Broadcast Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_broadcast_status: `%s`\n", status)
	fmt.Fprintf(&b, "- broadcast_routes: `%d`\n", req.RouteCount)
	fmt.Fprintf(&b, "- broadcast_queued: `%d`\n", result.Queued)
	fmt.Fprintf(&b, "- broadcast_duplicates: `%d`\n", result.Duplicates)
	fmt.Fprintf(&b, "- target_issues_created: `%d`\n", result.Created)
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", req.RoutesSHA)
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.MessageID))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- outbound_body_sha256_12: `%s`\n", req.OutboundBodySHA)
	fmt.Fprintf(&b, "- outbound_body_bytes: `%d`\n", req.OutboundBodyBytes)
	fmt.Fprintf(&b, "- outbound_body_lines: `%d`\n", req.OutboundBodyLines)
	fmt.Fprintf(&b, "- outbound_body_source: `%s`\n", req.BodySource)
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outbound_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_broadcast_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued one outbound channel message per reviewed route. Provider tokens, provider APIs, raw route names, raw thread IDs, raw message IDs, and outbound message bodies are not included in this receipt.\n\n")
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
	b.WriteString("- provider gateways read pending work with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent messages with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate broadcasts are suppressed independently for each route by `channel + message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelBroadcastActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBroadcastActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func normalizeChannelBroadcastOptions(opts ChannelBroadcastOptions) ChannelBroadcastOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Routes = normalizeChannelBroadcastRoutes(opts.Routes)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	opts.Body = strings.TrimSpace(opts.Body)
	return opts
}

func validateChannelBroadcastOptions(opts ChannelBroadcastOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if len(opts.Routes) == 0 {
		return fmt.Errorf("missing broadcast routes")
	}
	if opts.MessageID == "" {
		return fmt.Errorf("missing outbound message id")
	}
	if opts.Body == "" {
		return fmt.Errorf("missing outbound channel body")
	}
	return nil
}

func splitChannelBroadcastRoutes(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ','
	})
	routes := make([]string, 0, len(fields))
	for _, field := range fields {
		if route := cleanChannelRouteName(field); route != "" {
			routes = append(routes, route)
		}
	}
	return routes
}

func normalizeChannelBroadcastRoutes(routes []string) []string {
	normalized := make([]string, 0, len(routes))
	seen := map[string]bool{}
	for _, route := range routes {
		route = cleanChannelRouteName(route)
		if route == "" || seen[route] {
			continue
		}
		seen[route] = true
		normalized = append(normalized, route)
	}
	return normalized
}

func autoChannelBroadcastActionMessageID(ev Event, routes []string, body string) string {
	seed := strings.Join([]string{eventID(ev), strings.Join(routes, ","), body}, "|")
	return fmt.Sprintf("gitclaw-broadcast-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelBroadcastRoutesHash(routes []string) string {
	return shortDocumentHash(strings.Join(normalizeChannelBroadcastRoutes(routes), "\n"))
}
