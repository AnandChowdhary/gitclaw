package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

type ProactiveEnqueueOptions struct {
	Repo         string
	Name         string
	Slot         string
	Prompt       string
	PromptFile   string
	NotBefore    string
	NotifyRoutes []string
}

type ProactiveEnqueueResult struct {
	IssueNumber         int
	IssueURL            string
	Name                string
	Slot                string
	Created             bool
	Due                 bool
	Skipped             bool
	NotBefore           string
	ChannelNotification ProactiveChannelNotification
}

type ProactiveChannelNotification struct {
	Requested           bool
	Routes              int
	Queued              int
	Duplicates          int
	TargetIssuesCreated int
	RoutesSHA           string
	MessageSHA          string
	BodySHA             string
	BodyBytes           int
	BodyLines           int
	Destinations        []ChannelBroadcastDestinationResult
}

func RunProactiveEnqueue(ctx context.Context, cfg Config, github ProactiveGitHubClient, opts ProactiveEnqueueOptions, now time.Time) (ProactiveEnqueueResult, error) {
	opts = normalizeProactiveOptions(opts, now)
	notification := ProactiveChannelNotification{
		Requested: len(opts.NotifyRoutes) > 0,
		Routes:    len(opts.NotifyRoutes),
		RoutesSHA: channelBroadcastRoutesHash(opts.NotifyRoutes),
	}
	if err := validateProactiveEnvelopeOptions(opts); err != nil {
		return ProactiveEnqueueResult{}, err
	}
	due, err := proactiveDueReached(opts.NotBefore, now)
	if err != nil {
		return ProactiveEnqueueResult{}, err
	}
	if !due {
		return ProactiveEnqueueResult{
			Name:                opts.Name,
			Slot:                opts.Slot,
			Due:                 false,
			Skipped:             true,
			NotBefore:           opts.NotBefore,
			ChannelNotification: notification,
		}, nil
	}
	if opts.Prompt == "" && opts.PromptFile != "" {
		data, err := os.ReadFile(opts.PromptFile)
		if err != nil {
			return ProactiveEnqueueResult{}, fmt.Errorf("read proactive prompt file: %w", err)
		}
		opts.Prompt = strings.TrimSpace(string(data))
	}
	if err := validateProactiveOptions(opts); err != nil {
		return ProactiveEnqueueResult{}, err
	}

	issue, created, err := findOrCreateProactiveIssue(ctx, cfg, github, opts)
	if err != nil {
		return ProactiveEnqueueResult{}, err
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ProactiveLabel, cfg.TriggerLabel}); err != nil {
		return ProactiveEnqueueResult{}, fmt.Errorf("label proactive issue: %w", err)
	}
	result := ProactiveEnqueueResult{
		IssueNumber:         issue.Number,
		IssueURL:            issueURL(opts.Repo, issue.Number),
		Name:                opts.Name,
		Slot:                opts.Slot,
		Created:             created,
		Due:                 true,
		NotBefore:           opts.NotBefore,
		ChannelNotification: notification,
	}
	channelNotification, err := RunProactiveChannelNotification(ctx, cfg, github, opts, result)
	if err != nil {
		return ProactiveEnqueueResult{}, err
	}
	result.ChannelNotification = channelNotification
	return result, nil
}

func normalizeProactiveOptions(opts ProactiveEnqueueOptions, now time.Time) ProactiveEnqueueOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Name = normalizeProactiveName(opts.Name)
	opts.Slot = strings.TrimSpace(opts.Slot)
	opts.Prompt = strings.TrimSpace(opts.Prompt)
	opts.PromptFile = strings.TrimSpace(opts.PromptFile)
	opts.NotBefore = strings.TrimSpace(opts.NotBefore)
	opts.NotifyRoutes = normalizeChannelBroadcastRoutes(opts.NotifyRoutes)
	if opts.Slot == "" {
		if now.IsZero() {
			now = time.Now().UTC()
		}
		opts.Slot = now.UTC().Format("2006-01-02")
	}
	return opts
}

func normalizeProactiveName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func validateProactiveOptions(opts ProactiveEnqueueOptions) error {
	if err := validateProactiveEnvelopeOptions(opts); err != nil {
		return err
	}
	if opts.Prompt == "" {
		return fmt.Errorf("missing proactive prompt")
	}
	return nil
}

func validateProactiveEnvelopeOptions(opts ProactiveEnqueueOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Name == "" {
		return fmt.Errorf("missing proactive name")
	}
	if opts.Slot == "" {
		return fmt.Errorf("missing proactive slot")
	}
	return nil
}

func proactiveDueReached(notBefore string, now time.Time) (bool, error) {
	if strings.TrimSpace(notBefore) == "" {
		return true, nil
	}
	dueAt, err := parseProactiveNotBefore(notBefore)
	if err != nil {
		return false, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !now.UTC().Before(dueAt), nil
}

func parseProactiveNotBefore(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("missing proactive not-before time")
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid proactive not-before time %q: use RFC3339 or YYYY-MM-DD", value)
}

func findOrCreateProactiveIssue(ctx context.Context, cfg Config, github ProactiveGitHubClient, opts ProactiveEnqueueOptions) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.ProactiveLabel}, 100)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list proactive issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if proactiveRunMatches(issue.Body, opts.Name, opts.Slot) {
			return issue, false, nil
		}
	}

	title := fmt.Sprintf("GitClaw proactive %s %s", opts.Name, opts.Slot)
	issue, err := github.CreateIssue(ctx, opts.Repo, title, RenderProactiveRunBody(opts), nil)
	if err != nil {
		return Issue{}, false, fmt.Errorf("create proactive issue: %w", err)
	}
	return issue, true, nil
}

func RunProactiveChannelNotification(ctx context.Context, cfg Config, github ProactiveGitHubClient, opts ProactiveEnqueueOptions, result ProactiveEnqueueResult) (ProactiveChannelNotification, error) {
	notification := ProactiveChannelNotification{
		Requested: len(opts.NotifyRoutes) > 0,
		Routes:    len(opts.NotifyRoutes),
		RoutesSHA: channelBroadcastRoutesHash(opts.NotifyRoutes),
	}
	if len(opts.NotifyRoutes) == 0 {
		return notification, nil
	}
	if result.IssueNumber <= 0 {
		return notification, fmt.Errorf("missing proactive issue for channel notification")
	}
	channelClient, ok := github.(ChannelSendGitHubClient)
	if !ok {
		return notification, fmt.Errorf("proactive channel notification requires channel-capable GitHub client")
	}
	body := RenderProactiveChannelNotificationBody(opts, result)
	messageID := proactiveChannelNotificationMessageID(opts)
	broadcast, err := RunChannelBroadcast(ctx, cfg, channelClient, ChannelBroadcastOptions{
		Repo:      opts.Repo,
		Routes:    opts.NotifyRoutes,
		MessageID: messageID,
		Body:      body,
	})
	if err != nil {
		return notification, err
	}
	notification.Queued = broadcast.Queued
	notification.Duplicates = broadcast.Duplicates
	notification.TargetIssuesCreated = broadcast.Created
	notification.MessageSHA = shortDocumentHash(messageID)
	notification.BodySHA = shortDocumentHash(body)
	notification.BodyBytes = len(body)
	notification.BodyLines = lineCount(body)
	notification.Destinations = broadcast.Destinations
	return notification, nil
}

func RenderProactiveRunBody(opts ProactiveEnqueueOptions) string {
	return fmt.Sprintf(`<!-- gitclaw:proactive-run name="%s" slot="%s" -->
GitClaw proactive run.

- name: %s
- slot: %s

Proactive instruction:

%s`, escapeMarkerValue(opts.Name), escapeMarkerValue(opts.Slot), opts.Name, opts.Slot, strings.TrimSpace(opts.Prompt))
}

func RenderProactiveChannelNotificationBody(opts ProactiveEnqueueOptions, result ProactiveEnqueueResult) string {
	var b strings.Builder
	b.WriteString("GitClaw proactive run\n\n")
	fmt.Fprintf(&b, "Run issue: #%d %s\n", result.IssueNumber, result.IssueURL)
	fmt.Fprintf(&b, "Name: %s\n", opts.Name)
	fmt.Fprintf(&b, "Slot: %s\n", opts.Slot)
	fmt.Fprintf(&b, "Created: %t\n", result.Created)
	fmt.Fprintf(&b, "Due: %t\n", result.Due)
	fmt.Fprintf(&b, "Not before: %s\n", valueOrNone(opts.NotBefore))
	b.WriteString("\nComment on the GitHub issue to continue the proactive conversation. This notification did not call a model, run tools, include the proactive prompt, or deliver to an external provider directly.")
	return strings.TrimSpace(b.String())
}

func proactiveChannelNotificationMessageID(opts ProactiveEnqueueOptions) string {
	return fmt.Sprintf("gitclaw-proactive-%s-%s", opts.Name, proactiveNotificationIDPart(opts.Slot))
}

func proactiveNotificationIDPart(value string) string {
	cleaned := normalizeProactiveName(value)
	if cleaned == "" {
		return shortDocumentHash(value)
	}
	return cleaned
}

func proactiveRunMatches(body, name, slot string) bool {
	return HasProactiveRunMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`name="%s"`, escapeMarkerValue(name))) &&
		strings.Contains(body, fmt.Sprintf(`slot="%s"`, escapeMarkerValue(slot)))
}

func writeProactiveOutputs(result ProactiveEnqueueResult) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil
	}
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	defer file.Close()
	fmt.Fprintf(file, "issue_number=%d\n", result.IssueNumber)
	fmt.Fprintf(file, "issue_url=%s\n", result.IssueURL)
	fmt.Fprintf(file, "name=%s\n", result.Name)
	fmt.Fprintf(file, "slot=%s\n", result.Slot)
	fmt.Fprintf(file, "created=%t\n", result.Created)
	fmt.Fprintf(file, "due=%t\n", result.Due)
	fmt.Fprintf(file, "skipped=%t\n", result.Skipped)
	fmt.Fprintf(file, "not_before=%s\n", result.NotBefore)
	fmt.Fprintf(file, "channel_notification_requested=%t\n", result.ChannelNotification.Requested)
	fmt.Fprintf(file, "channel_notification_routes=%d\n", result.ChannelNotification.Routes)
	fmt.Fprintf(file, "channel_notification_queued=%d\n", result.ChannelNotification.Queued)
	fmt.Fprintf(file, "channel_notification_duplicates=%d\n", result.ChannelNotification.Duplicates)
	fmt.Fprintf(file, "channel_notification_target_issues_created=%d\n", result.ChannelNotification.TargetIssuesCreated)
	fmt.Fprintf(file, "channel_notification_routes_sha256_12=%s\n", result.ChannelNotification.RoutesSHA)
	fmt.Fprintf(file, "channel_notification_message_id_sha256_12=%s\n", result.ChannelNotification.MessageSHA)
	fmt.Fprintf(file, "channel_notification_body_sha256_12=%s\n", result.ChannelNotification.BodySHA)
	fmt.Fprintf(file, "channel_notification_body_bytes=%d\n", result.ChannelNotification.BodyBytes)
	fmt.Fprintf(file, "channel_notification_body_lines=%d\n", result.ChannelNotification.BodyLines)
	return nil
}
