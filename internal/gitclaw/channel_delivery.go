package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ChannelDeliveryOptions struct {
	Repo              string
	Channel           string
	AccountID         string
	IssueNumber       int
	CommentID         int64
	ExternalMessageID string
	GatewayRunID      string
}

type ChannelDeliveryResult struct {
	StateIssueNumber    int
	StateIssueURL       string
	ReceiptCommentID    int64
	CreatedStateIssue   bool
	Delivered           bool
	Duplicate           bool
	IssueNumber         int
	SourceCommentID     int64
	AccountHash         string
	ExternalMessageHash string
}

func RunChannelDelivery(ctx context.Context, cfg Config, github ChannelDeliveryGitHubClient, opts ChannelDeliveryOptions) (ChannelDeliveryResult, error) {
	opts = normalizeChannelDeliveryOptions(opts)
	if err := validateChannelDeliveryOptions(opts); err != nil {
		return ChannelDeliveryResult{}, err
	}
	if err := verifyChannelDeliverySource(ctx, github, opts); err != nil {
		return ChannelDeliveryResult{}, err
	}

	accountHash := channelStateHash(opts.AccountID)
	externalHash := channelStateHash(opts.ExternalMessageID)
	stateOpts := ChannelStateOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		AccountID: opts.AccountID,
	}
	issue, created, err := findOrCreateChannelStateIssue(ctx, cfg, github, stateOpts, accountHash)
	if err != nil {
		return ChannelDeliveryResult{}, err
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelDeliveryResult{}, fmt.Errorf("label channel delivery state issue: %w", err)
	}

	result := ChannelDeliveryResult{
		StateIssueNumber:    issue.Number,
		StateIssueURL:       issueURL(opts.Repo, issue.Number),
		CreatedStateIssue:   created,
		IssueNumber:         opts.IssueNumber,
		SourceCommentID:     opts.CommentID,
		AccountHash:         accountHash,
		ExternalMessageHash: externalHash,
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelDeliveryResult{}, fmt.Errorf("list channel delivery comments: %w", err)
	}
	for _, comment := range comments {
		if channelDeliveryMatches(comment.Body, opts.Channel, accountHash, opts.IssueNumber, opts.CommentID) {
			result.Duplicate = true
			return result, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelDeliveryComment(opts, accountHash, externalHash))
	if err != nil {
		return ChannelDeliveryResult{}, fmt.Errorf("post channel delivery receipt: %w", err)
	}
	result.ReceiptCommentID = posted.ID
	result.Delivered = true
	return result, nil
}

func normalizeChannelDeliveryOptions(opts ChannelDeliveryOptions) ChannelDeliveryOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.AccountID = strings.TrimSpace(opts.AccountID)
	opts.ExternalMessageID = strings.TrimSpace(opts.ExternalMessageID)
	opts.GatewayRunID = strings.TrimSpace(opts.GatewayRunID)
	return opts
}

func validateChannelDeliveryOptions(opts ChannelDeliveryOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.AccountID == "" {
		return fmt.Errorf("missing channel account id")
	}
	if opts.IssueNumber <= 0 {
		return fmt.Errorf("missing source issue number")
	}
	if opts.CommentID <= 0 {
		return fmt.Errorf("missing source comment id")
	}
	if opts.ExternalMessageID == "" {
		return fmt.Errorf("missing external message id")
	}
	return nil
}

func verifyChannelDeliverySource(ctx context.Context, github ChannelDeliveryGitHubClient, opts ChannelDeliveryOptions) error {
	comments, err := github.ListIssueComments(ctx, opts.Repo, opts.IssueNumber)
	if err != nil {
		return fmt.Errorf("list source issue comments: %w", err)
	}
	for _, comment := range comments {
		if comment.ID != opts.CommentID {
			continue
		}
		if !isChannelDeliverableComment(comment.Body) {
			return fmt.Errorf("source comment is not a GitClaw assistant turn or channel deliverable message")
		}
		return nil
	}
	return fmt.Errorf("source assistant comment not found")
}

func isChannelDeliverableComment(body string) bool {
	return HasGitClawMarker(body) || HasChannelOutboundMarker(body) || HasChannelDeliverableMarker(body) || HasChannelReactionMarker(body) || HasChannelStatusMarker(body) || HasChannelEditMarker(body) || HasChannelTopicMarker(body) || HasChannelActivityMarker(body)
}

func RenderChannelDeliveryComment(opts ChannelDeliveryOptions, accountHash, externalHash string) string {
	return fmt.Sprintf(`<!-- gitclaw:channel-delivery channel="%s" account_sha256_12="%s" issue_number="%d" source_comment_id="%d" external_message_sha256_12="%s" gateway_run_id="%s" -->
GitClaw channel delivery receipt.

- channel: %s
- account_sha256_12: %s
- source_issue: #%d
- source_comment_id: %d
- external_message_sha256_12: %s
- gateway_run_id: %s
- raw_delivery_bodies_included: false`, escapeMarkerValue(opts.Channel), escapeMarkerValue(accountHash), opts.IssueNumber, opts.CommentID, escapeMarkerValue(externalHash), escapeMarkerValue(opts.GatewayRunID), opts.Channel, accountHash, opts.IssueNumber, opts.CommentID, externalHash, inlineCodeOrNone(opts.GatewayRunID))
}

func channelDeliveryMatches(body, channel, accountHash string, issueNumber int, commentID int64) bool {
	return HasChannelDeliveryMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`account_sha256_12="%s"`, escapeMarkerValue(accountHash))) &&
		strings.Contains(body, fmt.Sprintf(`issue_number="%d"`, issueNumber)) &&
		strings.Contains(body, fmt.Sprintf(`source_comment_id="%d"`, commentID))
}

func writeChannelDeliveryOutputs(result ChannelDeliveryResult) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil
	}
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	defer file.Close()
	fmt.Fprintf(file, "state_issue_number=%d\n", result.StateIssueNumber)
	fmt.Fprintf(file, "state_issue_url=%s\n", result.StateIssueURL)
	fmt.Fprintf(file, "receipt_comment_id=%d\n", result.ReceiptCommentID)
	fmt.Fprintf(file, "created_state_issue=%t\n", result.CreatedStateIssue)
	fmt.Fprintf(file, "delivered=%t\n", result.Delivered)
	fmt.Fprintf(file, "duplicate=%t\n", result.Duplicate)
	fmt.Fprintf(file, "issue_number=%d\n", result.IssueNumber)
	fmt.Fprintf(file, "source_comment_id=%d\n", result.SourceCommentID)
	fmt.Fprintf(file, "account_sha256_12=%s\n", result.AccountHash)
	fmt.Fprintf(file, "external_message_sha256_12=%s\n", result.ExternalMessageHash)
	return nil
}

func parsePositiveInt(value, name string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s: %q", name, value)
	}
	return parsed, nil
}

func parsePositiveInt64(value, name string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s: %q", name, value)
	}
	return parsed, nil
}
