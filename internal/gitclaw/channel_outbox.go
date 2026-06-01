package gitclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const defaultChannelOutboxLimit = 50

type ChannelOutboxOptions struct {
	Repo        string
	Channel     string
	AccountID   string
	IssueNumber int
	IncludeBody bool
	OutPath     string
	Limit       int
}

type ChannelOutboxResult struct {
	IssueNumber                int
	StateIssueNumber           int
	StateIssueURL              string
	SourceAssistantComments    int
	SourceOutboundComments     int
	SourceReactionComments     int
	SourceStatusComments       int
	SourceDeliverableComments  int
	DeliveredAssistantComments int
	PendingMessages            int
	MessagesReturned           int
	BodyIncluded               bool
	AccountHash                string
	OutPath                    string
	Messages                   []ChannelOutboxMessage
}

type ChannelOutboxMessage struct {
	IssueNumber     int    `json:"issue_number"`
	SourceCommentID int64  `json:"source_comment_id"`
	Kind            string `json:"kind"`
	BodySHA         string `json:"body_sha256_12"`
	BodyBytes       int    `json:"body_bytes"`
	BodyLines       int    `json:"body_lines"`
	CreatedAt       string `json:"created_at,omitempty"`
	MessageHash     string `json:"outbound_message_sha256_12,omitempty"`
	Body            string `json:"body,omitempty"`
}

type channelOutboxFile struct {
	Channel                   string                 `json:"channel"`
	AccountSHA                string                 `json:"account_sha256_12"`
	IssueNumber               int                    `json:"issue_number"`
	StateIssueNumber          int                    `json:"state_issue_number,omitempty"`
	SourceAssistantComments   int                    `json:"source_assistant_comments"`
	SourceOutboundComments    int                    `json:"source_outbound_comments"`
	SourceReactionComments    int                    `json:"source_reaction_comments"`
	SourceStatusComments      int                    `json:"source_status_comments"`
	SourceDeliverableComments int                    `json:"source_deliverable_comments"`
	DeliveredAssistantReplies int                    `json:"delivered_assistant_comments"`
	PendingMessages           int                    `json:"pending_messages"`
	MessagesReturned          int                    `json:"messages_returned"`
	BodyIncluded              bool                   `json:"body_included"`
	Messages                  []ChannelOutboxMessage `json:"messages"`
}

func RunChannelOutbox(ctx context.Context, cfg Config, github ChannelOutboxGitHubClient, opts ChannelOutboxOptions) (ChannelOutboxResult, error) {
	opts = normalizeChannelOutboxOptions(opts)
	if err := validateChannelOutboxOptions(opts); err != nil {
		return ChannelOutboxResult{}, err
	}
	if opts.IncludeBody && opts.OutPath == "" {
		return ChannelOutboxResult{}, fmt.Errorf("--include-body requires --out")
	}

	issue, err := github.GetIssue(ctx, opts.Repo, opts.IssueNumber)
	if err != nil {
		return ChannelOutboxResult{}, fmt.Errorf("get channel issue: %w", err)
	}
	if issue.IsPullRequest {
		return ChannelOutboxResult{}, fmt.Errorf("channel outbox source must be an issue")
	}
	threadChannel, threadID := channelThreadMarkerFields(issue.Body)
	if threadChannel == "" {
		return ChannelOutboxResult{}, fmt.Errorf("source issue is missing gitclaw:channel-thread marker")
	}
	if threadChannel != opts.Channel {
		return ChannelOutboxResult{}, fmt.Errorf("source issue channel does not match requested channel")
	}

	accountHash := channelStateHash(opts.AccountID)
	result := ChannelOutboxResult{
		IssueNumber:  opts.IssueNumber,
		BodyIncluded: opts.IncludeBody,
		AccountHash:  accountHash,
		OutPath:      opts.OutPath,
	}

	delivered := map[int64]bool{}
	if stateIssue, ok, err := findExistingChannelStateIssue(ctx, cfg, github, opts.Repo, opts.Channel, accountHash); err != nil {
		return ChannelOutboxResult{}, err
	} else if ok {
		result.StateIssueNumber = stateIssue.Number
		result.StateIssueURL = issueURL(opts.Repo, stateIssue.Number)
		comments, err := github.ListIssueComments(ctx, opts.Repo, stateIssue.Number)
		if err != nil {
			return ChannelOutboxResult{}, fmt.Errorf("list channel state comments: %w", err)
		}
		delivered = deliveredChannelComments(comments, opts.Channel, accountHash, opts.IssueNumber)
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, opts.IssueNumber)
	if err != nil {
		return ChannelOutboxResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		kind, visibleBody, messageHash, ok := channelOutboxDeliverable(comment.Body, opts.Channel, threadID)
		if !ok {
			continue
		}
		result.SourceDeliverableComments++
		if kind == "assistant" {
			result.SourceAssistantComments++
		} else if kind == "channel-outbound" {
			result.SourceOutboundComments++
		} else if kind == "channel-reaction" {
			result.SourceReactionComments++
		} else if kind == "channel-status" {
			result.SourceStatusComments++
		}
		if delivered[comment.ID] {
			result.DeliveredAssistantComments++
			continue
		}
		message := ChannelOutboxMessage{
			IssueNumber:     opts.IssueNumber,
			SourceCommentID: comment.ID,
			Kind:            kind,
			BodySHA:         shortDocumentHash(visibleBody),
			BodyBytes:       len(visibleBody),
			BodyLines:       lineCount(visibleBody),
			CreatedAt:       comment.CreatedAt,
			MessageHash:     messageHash,
		}
		if opts.IncludeBody {
			message.Body = visibleBody
		}
		result.PendingMessages++
		if len(result.Messages) < opts.Limit {
			result.Messages = append(result.Messages, message)
		}
	}
	result.MessagesReturned = len(result.Messages)
	if opts.OutPath != "" {
		if err := writeChannelOutboxFile(opts, result); err != nil {
			return ChannelOutboxResult{}, err
		}
	}
	return result, nil
}

func channelOutboxDeliverable(body, channel, threadID string) (string, string, string, bool) {
	if HasGitClawMarker(body) {
		return "assistant", StripMarker(body), "", true
	}
	outboundChannel, outboundThread, outboundMessageID := channelOutboundMarkerFields(body)
	if outboundChannel == "" {
		reactionChannel, reactionThread, reactionMessageID, reaction := channelReactionMarkerFields(body)
		if reactionChannel == "" {
			statusChannel, statusThread, _, statusID, state := channelStatusMarkerFields(body)
			if statusChannel == "" {
				return "", "", "", false
			}
			if statusChannel != channel {
				return "", "", "", false
			}
			if statusThread != "" && threadID != "" && statusThread != threadID {
				return "", "", "", false
			}
			return "channel-status", StripChannelStatusMarker(body), channelStateHash(statusID + "|" + state), true
		}
		if reactionChannel != channel {
			return "", "", "", false
		}
		if reactionThread != "" && threadID != "" && reactionThread != threadID {
			return "", "", "", false
		}
		return "channel-reaction", StripChannelReactionMarker(body), channelStateHash(reactionMessageID + "|" + reaction), true
	}
	if outboundChannel != channel {
		return "", "", "", false
	}
	if outboundThread != "" && threadID != "" && outboundThread != threadID {
		return "", "", "", false
	}
	return "channel-outbound", StripChannelOutboundMarker(body), channelStateHash(outboundMessageID), true
}

func normalizeChannelOutboxOptions(opts ChannelOutboxOptions) ChannelOutboxOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.AccountID = strings.TrimSpace(opts.AccountID)
	opts.OutPath = strings.TrimSpace(opts.OutPath)
	if opts.Limit <= 0 {
		opts.Limit = defaultChannelOutboxLimit
	}
	return opts
}

func validateChannelOutboxOptions(opts ChannelOutboxOptions) error {
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
	if opts.Limit <= 0 {
		return fmt.Errorf("invalid outbox limit")
	}
	return nil
}

func findExistingChannelStateIssue(ctx context.Context, cfg Config, github ChannelOutboxGitHubClient, repo, channel, accountHash string) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, repo, []string{cfg.ChannelLabel}, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list channel state issues: %w", err)
	}
	if issue, ok := findChannelStateIssue(issues, channel, accountHash); ok {
		return issue, true, nil
	}
	issues, err = github.ListOpenIssues(ctx, repo, nil, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list unlabeled channel state issues: %w", err)
	}
	if issue, ok := findChannelStateIssue(issues, channel, accountHash); ok {
		return issue, true, nil
	}
	return Issue{}, false, nil
}

func deliveredChannelComments(comments []Comment, channel, accountHash string, issueNumber int) map[int64]bool {
	delivered := map[int64]bool{}
	for _, comment := range comments {
		receiptChannel, receiptAccountHash, receiptIssue, sourceCommentID := channelDeliveryMarkerFields(comment.Body)
		if receiptChannel != channel || receiptAccountHash != accountHash || receiptIssue != issueNumber || sourceCommentID <= 0 {
			continue
		}
		delivered[sourceCommentID] = true
	}
	return delivered
}

func channelThreadMarkerFields(body string) (string, string) {
	match := channelThreadMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", ""
	}
	return markerAttribute(match[1], "channel"), markerAttribute(match[1], "thread_id")
}

func channelDeliveryMarkerFields(body string) (string, string, int, int64) {
	match := channelDeliveryMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", "", 0, 0
	}
	channel := markerAttribute(match[1], "channel")
	accountHash := markerAttribute(match[1], "account_sha256_12")
	issueNumber, _ := strconv.Atoi(markerAttribute(match[1], "issue_number"))
	sourceCommentID, _ := strconv.ParseInt(markerAttribute(match[1], "source_comment_id"), 10, 64)
	return channel, accountHash, issueNumber, sourceCommentID
}

func writeChannelOutboxFile(opts ChannelOutboxOptions, result ChannelOutboxResult) error {
	payload := channelOutboxFile{
		Channel:                   opts.Channel,
		AccountSHA:                result.AccountHash,
		IssueNumber:               result.IssueNumber,
		StateIssueNumber:          result.StateIssueNumber,
		SourceAssistantComments:   result.SourceAssistantComments,
		SourceOutboundComments:    result.SourceOutboundComments,
		SourceReactionComments:    result.SourceReactionComments,
		SourceStatusComments:      result.SourceStatusComments,
		SourceDeliverableComments: result.SourceDeliverableComments,
		DeliveredAssistantReplies: result.DeliveredAssistantComments,
		PendingMessages:           result.PendingMessages,
		MessagesReturned:          result.MessagesReturned,
		BodyIncluded:              result.BodyIncluded,
		Messages:                  result.Messages,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(opts.OutPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(opts.OutPath, append(data, '\n'), 0o600)
}

func writeChannelOutboxOutputs(result ChannelOutboxResult) error {
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
	fmt.Fprintf(file, "state_issue_number=%d\n", result.StateIssueNumber)
	fmt.Fprintf(file, "state_issue_url=%s\n", result.StateIssueURL)
	fmt.Fprintf(file, "source_assistant_comments=%d\n", result.SourceAssistantComments)
	fmt.Fprintf(file, "delivered_assistant_comments=%d\n", result.DeliveredAssistantComments)
	fmt.Fprintf(file, "pending_messages=%d\n", result.PendingMessages)
	fmt.Fprintf(file, "messages_returned=%d\n", result.MessagesReturned)
	fmt.Fprintf(file, "body_included=%t\n", result.BodyIncluded)
	fmt.Fprintf(file, "account_sha256_12=%s\n", result.AccountHash)
	fmt.Fprintf(file, "out_path=%s\n", result.OutPath)
	if len(result.Messages) > 0 {
		fmt.Fprintf(file, "first_comment_id=%d\n", result.Messages[0].SourceCommentID)
	}
	return nil
}

func sortChannelOutboxMessages(messages []ChannelOutboxMessage) {
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].SourceCommentID < messages[j].SourceCommentID
	})
}
