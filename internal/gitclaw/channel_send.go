package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type ChannelSendOptions struct {
	Repo      string
	Route     string
	Channel   string
	ThreadID  string
	MessageID string
	Author    string
	Body      string
}

type ChannelSendResult struct {
	IssueNumber int
	IssueURL    string
	CommentID   int64
	Created     bool
	Duplicate   bool
	RouteName   string
	RouteHash   string
}

func RunChannelSend(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSendOptions) (ChannelSendResult, error) {
	opts = normalizeChannelSendOptions(opts)
	var err error
	opts, err = applyChannelSendRoute(cfg, opts)
	if err != nil {
		return ChannelSendResult{}, err
	}
	if err := validateChannelSendOptions(opts); err != nil {
		return ChannelSendResult{}, err
	}

	issue, created, err := findOrCreateChannelIssue(ctx, cfg, github, ChannelIngestOptions{
		Repo:     opts.Repo,
		Channel:  opts.Channel,
		ThreadID: opts.ThreadID,
	})
	if err != nil {
		return ChannelSendResult{}, err
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelSendResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		if channelOutboundMatches(comment.Body, opts.Channel, opts.MessageID) {
			_ = github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel})
			return ChannelSendResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(opts.Repo, issue.Number),
				Created:     created,
				Duplicate:   true,
				RouteName:   opts.Route,
				RouteHash:   channelRouteHash(opts.Route),
			}, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelOutboundComment(opts))
	if err != nil {
		return ChannelSendResult{}, fmt.Errorf("post outbound channel message: %w", err)
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelSendResult{}, fmt.Errorf("label channel issue: %w", err)
	}
	return ChannelSendResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(opts.Repo, issue.Number),
		CommentID:   posted.ID,
		Created:     created,
		RouteName:   opts.Route,
		RouteHash:   channelRouteHash(opts.Route),
	}, nil
}

func normalizeChannelSendOptions(opts ChannelSendOptions) ChannelSendOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	opts.Body = strings.TrimSpace(opts.Body)
	return opts
}

func validateChannelSendOptions(opts ChannelSendOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.MessageID == "" {
		return fmt.Errorf("missing outbound message id")
	}
	if opts.Body == "" {
		return fmt.Errorf("missing outbound channel body")
	}
	return nil
}

func RenderChannelOutboundComment(opts ChannelSendOptions) string {
	author := opts.Author
	if author == "" {
		author = "gitclaw"
	}
	return fmt.Sprintf(`<!-- gitclaw:channel-outbound channel="%s" thread_id="%s" message_id="%s" author="%s" -->
%s`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), escapeMarkerValue(opts.MessageID), escapeMarkerValue(author), strings.TrimSpace(opts.Body))
}

func channelOutboundMatches(body, channel, messageID string) bool {
	return HasChannelOutboundMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`message_id="%s"`, escapeMarkerValue(messageID)))
}

func channelOutboundMarkerFields(body string) (string, string, string) {
	match := channelOutboundMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", "", ""
	}
	return markerAttribute(match[1], "channel"), markerAttribute(match[1], "thread_id"), markerAttribute(match[1], "message_id")
}

func channelRouteHash(route string) string {
	if strings.TrimSpace(route) == "" {
		return ""
	}
	return shortDocumentHash(route)
}

func writeChannelSendOutputs(result ChannelSendResult) error {
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
	fmt.Fprintf(file, "comment_id=%d\n", result.CommentID)
	fmt.Fprintf(file, "created=%t\n", result.Created)
	fmt.Fprintf(file, "duplicate=%t\n", result.Duplicate)
	fmt.Fprintf(file, "route_resolved=%t\n", result.RouteName != "")
	fmt.Fprintf(file, "route_sha256_12=%s\n", result.RouteHash)
	return nil
}
