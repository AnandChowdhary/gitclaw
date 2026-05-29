package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type ChannelIngestOptions struct {
	Repo      string
	Channel   string
	ThreadID  string
	MessageID string
	Author    string
	Body      string
}

type ChannelIngestResult struct {
	IssueNumber int
	IssueURL    string
	CommentID   int64
	Created     bool
	Duplicate   bool
}

func RunChannelIngest(ctx context.Context, cfg Config, github ChannelIngestGitHubClient, opts ChannelIngestOptions) (ChannelIngestResult, error) {
	opts = normalizeChannelIngestOptions(opts)
	if err := validateChannelIngestOptions(opts); err != nil {
		return ChannelIngestResult{}, err
	}

	issue, created, err := findOrCreateChannelIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelIngestResult{}, err
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelIngestResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		if channelMessageMatches(comment.Body, opts.Channel, opts.MessageID) {
			_ = github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.TriggerLabel, cfg.ChannelLabel})
			return ChannelIngestResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(opts.Repo, issue.Number),
				Created:     created,
				Duplicate:   true,
			}, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelMessageComment(opts))
	if err != nil {
		return ChannelIngestResult{}, fmt.Errorf("post channel message: %w", err)
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.TriggerLabel, cfg.ChannelLabel}); err != nil {
		return ChannelIngestResult{}, fmt.Errorf("label channel issue: %w", err)
	}
	return ChannelIngestResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(opts.Repo, issue.Number),
		CommentID:   posted.ID,
		Created:     created,
	}, nil
}

func normalizeChannelIngestOptions(opts ChannelIngestOptions) ChannelIngestOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	opts.Body = strings.TrimSpace(opts.Body)
	return opts
}

func validateChannelIngestOptions(opts ChannelIngestOptions) error {
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
		return fmt.Errorf("missing message id")
	}
	if opts.Body == "" {
		return fmt.Errorf("missing channel message body")
	}
	return nil
}

func findOrCreateChannelIssue(ctx context.Context, cfg Config, github ChannelIngestGitHubClient, opts ChannelIngestOptions) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, nil, 100)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list channel issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelThreadMatches(issue.Body, opts.Channel, opts.ThreadID) {
			return issue, false, nil
		}
	}

	title := fmt.Sprintf("GitClaw %s thread %s", opts.Channel, opts.ThreadID)
	body := RenderChannelThreadBody(opts)
	issue, err := github.CreateIssue(ctx, opts.Repo, title, body, nil)
	if err != nil {
		return Issue{}, false, fmt.Errorf("create channel issue: %w", err)
	}
	return issue, true, nil
}

func RenderChannelThreadBody(opts ChannelIngestOptions) string {
	return fmt.Sprintf(`<!-- gitclaw:channel-thread channel="%s" thread_id="%s" -->
GitClaw channel bridge thread.

- channel: %s
- thread_id: %s

Messages from this external channel are mirrored into this issue as comments
with gitclaw:channel-message provenance markers.`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), opts.Channel, opts.ThreadID)
}

func RenderChannelMessageComment(opts ChannelIngestOptions) string {
	author := opts.Author
	if author == "" {
		author = opts.Channel
	}
	return fmt.Sprintf(`<!-- gitclaw:channel-message channel="%s" thread_id="%s" message_id="%s" author="%s" -->
%s`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), escapeMarkerValue(opts.MessageID), escapeMarkerValue(author), strings.TrimSpace(opts.Body))
}

func channelThreadMatches(body, channel, threadID string) bool {
	return HasChannelThreadMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`thread_id="%s"`, escapeMarkerValue(threadID)))
}

func channelMessageMatches(body, channel, messageID string) bool {
	return HasChannelMessageMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`message_id="%s"`, escapeMarkerValue(messageID)))
}

func issueURL(repo string, issueNumber int) string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", repo, issueNumber)
}

func writeChannelIngestOutputs(result ChannelIngestResult) error {
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
	return nil
}
