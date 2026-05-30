package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type ChannelStateOptions struct {
	Repo       string
	Channel    string
	AccountID  string
	Offset     string
	LeaseRunID string
}

type ChannelStateResult struct {
	IssueNumber int
	IssueURL    string
	CommentID   int64
	Created     bool
	Updated     bool
	Duplicate   bool
	AccountHash string
	OffsetHash  string
}

func RunChannelState(ctx context.Context, cfg Config, github ChannelStateGitHubClient, opts ChannelStateOptions) (ChannelStateResult, error) {
	opts = normalizeChannelStateOptions(opts)
	if err := validateChannelStateOptions(opts); err != nil {
		return ChannelStateResult{}, err
	}

	accountHash := channelStateHash(opts.AccountID)
	offsetHash := ""
	if opts.Offset != "" {
		offsetHash = channelStateHash(opts.Offset)
	}
	issue, created, err := findOrCreateChannelStateIssue(ctx, cfg, github, opts, accountHash)
	if err != nil {
		return ChannelStateResult{}, err
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelStateResult{}, fmt.Errorf("label channel state issue: %w", err)
	}

	result := ChannelStateResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(opts.Repo, issue.Number),
		Created:     created,
		AccountHash: accountHash,
		OffsetHash:  offsetHash,
	}
	if opts.Offset == "" {
		return result, nil
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelStateResult{}, fmt.Errorf("list channel state comments: %w", err)
	}
	for _, comment := range comments {
		if channelStateUpdateMatches(comment.Body, opts.Channel, accountHash, offsetHash) {
			result.Duplicate = true
			return result, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelStateUpdateComment(opts, accountHash, offsetHash))
	if err != nil {
		return ChannelStateResult{}, fmt.Errorf("post channel state update: %w", err)
	}
	result.CommentID = posted.ID
	result.Updated = true
	return result, nil
}

func normalizeChannelStateOptions(opts ChannelStateOptions) ChannelStateOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.AccountID = strings.TrimSpace(opts.AccountID)
	opts.Offset = strings.TrimSpace(opts.Offset)
	opts.LeaseRunID = strings.TrimSpace(opts.LeaseRunID)
	return opts
}

func validateChannelStateOptions(opts ChannelStateOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.AccountID == "" {
		return fmt.Errorf("missing channel account id")
	}
	return nil
}

func findOrCreateChannelStateIssue(ctx context.Context, cfg Config, github ChannelStateGitHubClient, opts ChannelStateOptions, accountHash string) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.ChannelLabel}, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list channel state issues: %w", err)
	}
	if issue, ok := findChannelStateIssue(issues, opts.Channel, accountHash); ok {
		return issue, false, nil
	}
	issues, err = github.ListOpenIssues(ctx, opts.Repo, nil, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list unlabeled channel state issues: %w", err)
	}
	if issue, ok := findChannelStateIssue(issues, opts.Channel, accountHash); ok {
		return issue, false, nil
	}

	title := fmt.Sprintf("GitClaw %s channel state %s", opts.Channel, accountHash)
	issue, err := github.CreateIssue(ctx, opts.Repo, title, RenderChannelStateBody(opts, accountHash), []string{cfg.ChannelLabel})
	if err != nil {
		return Issue{}, false, fmt.Errorf("create channel state issue: %w", err)
	}
	return issue, true, nil
}

func findChannelStateIssue(issues []Issue, channel, accountHash string) (Issue, bool) {
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelStateMatches(issue.Body, channel, accountHash) {
			return issue, true
		}
	}
	return Issue{}, false
}

func RenderChannelStateBody(opts ChannelStateOptions, accountHash string) string {
	return fmt.Sprintf(`<!-- gitclaw:channel-state channel="%s" account_sha256_12="%s" -->
GitClaw channel bridge state.

- channel: %s
- account_sha256_12: %s
- raw_state_bodies_included: false

Provider pollers use this issue as durable GitHub-native state. Raw channel account IDs, offsets, tokens, and message bodies are not written here.`, escapeMarkerValue(opts.Channel), escapeMarkerValue(accountHash), opts.Channel, accountHash)
}

func RenderChannelStateUpdateComment(opts ChannelStateOptions, accountHash, offsetHash string) string {
	return fmt.Sprintf(`<!-- gitclaw:channel-state-update channel="%s" account_sha256_12="%s" offset_sha256_12="%s" lease_run_id="%s" -->
GitClaw channel state update.

- channel: %s
- account_sha256_12: %s
- offset_sha256_12: %s
- lease_run_id: %s
- raw_state_bodies_included: false`, escapeMarkerValue(opts.Channel), escapeMarkerValue(accountHash), escapeMarkerValue(offsetHash), escapeMarkerValue(opts.LeaseRunID), opts.Channel, accountHash, offsetHash, inlineCodeOrNone(opts.LeaseRunID))
}

func channelStateMatches(body, channel, accountHash string) bool {
	return HasChannelStateMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`account_sha256_12="%s"`, escapeMarkerValue(accountHash)))
}

func channelStateUpdateMatches(body, channel, accountHash, offsetHash string) bool {
	return HasChannelStateUpdateMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`account_sha256_12="%s"`, escapeMarkerValue(accountHash))) &&
		strings.Contains(body, fmt.Sprintf(`offset_sha256_12="%s"`, escapeMarkerValue(offsetHash)))
}

func channelStateHash(value string) string {
	return shortDocumentHash(value)
}

func inlineCodeOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

func writeChannelStateOutputs(result ChannelStateResult) error {
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
	fmt.Fprintf(file, "updated=%t\n", result.Updated)
	fmt.Fprintf(file, "duplicate=%t\n", result.Duplicate)
	fmt.Fprintf(file, "account_sha256_12=%s\n", result.AccountHash)
	fmt.Fprintf(file, "offset_sha256_12=%s\n", result.OffsetHash)
	return nil
}
