package gitclaw

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunChannelOutboxReturnsPendingAssistantReplies(t *testing.T) {
	cfg := DefaultConfig()
	source := Issue{
		Number: 42,
		Title:  "GitClaw telegram thread chat-123",
		Body: RenderChannelThreadBody(ChannelIngestOptions{
			Channel:  "telegram",
			ThreadID: "chat-123",
		}),
		Labels: []string{cfg.ChannelLabel},
	}
	assistant := RenderAssistantComment(Marker{
		RunID:          "run-outbox",
		EventID:        "issue-42",
		Model:          "openai/gpt-5-nano",
		IdempotencyKey: "outbox-key",
	}, "Assistant reply body for channel outbox.\n\nOUTBOX_REPLY_TOKEN")
	github := &FakeGitHub{
		Issues: []Issue{source},
		CommentsByIssue: map[int][]Comment{
			42: {
				{ID: 10, Body: RenderChannelMessageComment(ChannelIngestOptions{Channel: "telegram", ThreadID: "chat-123", MessageID: "msg-1", Body: "user body"})},
				{ID: 11, Body: assistant, CreatedAt: "2026-06-01T10:00:00Z"},
			},
		},
	}
	outPath := filepath.Join(t.TempDir(), "outbox.json")

	result, err := RunChannelOutbox(context.Background(), cfg, github, ChannelOutboxOptions{
		Repo:        "owner/repo",
		Channel:     "Telegram",
		AccountID:   "telegram-account-secret",
		IssueNumber: 42,
		IncludeBody: true,
		OutPath:     outPath,
	})
	if err != nil {
		t.Fatalf("RunChannelOutbox returned error: %v", err)
	}
	if result.SourceAssistantComments != 1 || result.DeliveredAssistantComments != 0 || result.PendingMessages != 1 || result.MessagesReturned != 1 {
		t.Fatalf("unexpected outbox result: %#v", result)
	}
	if !result.BodyIncluded || result.AccountHash == "telegram-account-secret" {
		t.Fatalf("unexpected body/hash fields: %#v", result)
	}
	if got := result.Messages[0].Body; !strings.Contains(got, "OUTBOX_REPLY_TOKEN") || strings.Contains(got, "gitclaw:assistant-turn") {
		t.Fatalf("outbox body should include visible assistant reply only: %q", got)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read outbox file: %v", err)
	}
	var payload struct {
		AccountSHA      string `json:"account_sha256_12"`
		PendingMessages int    `json:"pending_messages"`
		Messages        []struct {
			SourceCommentID int64  `json:"source_comment_id"`
			Body            string `json:"body"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode outbox file: %v\n%s", err, data)
	}
	if payload.AccountSHA != result.AccountHash || payload.PendingMessages != 1 || len(payload.Messages) != 1 || payload.Messages[0].SourceCommentID != 11 {
		t.Fatalf("unexpected outbox file payload: %#v", payload)
	}
	if !strings.Contains(payload.Messages[0].Body, "OUTBOX_REPLY_TOKEN") {
		t.Fatalf("outbox file missing assistant body: %s", data)
	}
}

func TestRunChannelOutboxSkipsDeliveredAssistantReplies(t *testing.T) {
	cfg := DefaultConfig()
	accountID := "delivery-account-secret"
	accountHash := channelStateHash(accountID)
	source := Issue{
		Number: 42,
		Title:  "GitClaw telegram thread chat-123",
		Body:   RenderChannelThreadBody(ChannelIngestOptions{Channel: "telegram", ThreadID: "chat-123"}),
		Labels: []string{cfg.ChannelLabel},
	}
	state := Issue{
		Number: 100,
		Title:  "GitClaw telegram channel state " + accountHash,
		Body:   RenderChannelStateBody(ChannelStateOptions{Channel: "telegram", AccountID: accountID}, accountHash),
		Labels: []string{cfg.ChannelLabel},
	}
	github := &FakeGitHub{
		Issues: []Issue{source, state},
		CommentsByIssue: map[int][]Comment{
			42: {
				{ID: 11, Body: RenderAssistantComment(Marker{RunID: "run-1", EventID: "issue-42", Model: "openai/gpt-5-nano", IdempotencyKey: "one"}, "already delivered")},
				{ID: 12, Body: RenderAssistantComment(Marker{RunID: "run-2", EventID: "issue-42", Model: "openai/gpt-5-nano", IdempotencyKey: "two"}, "pending reply")},
			},
			100: {{
				ID: 21,
				Body: RenderChannelDeliveryComment(ChannelDeliveryOptions{
					Channel:           "telegram",
					AccountID:         accountID,
					IssueNumber:       42,
					CommentID:         11,
					ExternalMessageID: "provider-message-secret",
				}, accountHash, channelStateHash("provider-message-secret")),
			}},
		},
	}

	result, err := RunChannelOutbox(context.Background(), cfg, github, ChannelOutboxOptions{
		Repo:        "owner/repo",
		Channel:     "telegram",
		AccountID:   accountID,
		IssueNumber: 42,
	})
	if err != nil {
		t.Fatalf("RunChannelOutbox returned error: %v", err)
	}
	if result.StateIssueNumber != 100 || result.SourceAssistantComments != 2 || result.DeliveredAssistantComments != 1 || result.PendingMessages != 1 {
		t.Fatalf("unexpected outbox result: %#v", result)
	}
	if len(result.Messages) != 1 || result.Messages[0].SourceCommentID != 12 || result.Messages[0].Body != "" {
		t.Fatalf("outbox should return only pending metadata without bodies: %#v", result.Messages)
	}
}

func TestRunChannelOutboxRequiresExplicitFileForBodies(t *testing.T) {
	_, err := RunChannelOutbox(context.Background(), DefaultConfig(), &FakeGitHub{}, ChannelOutboxOptions{
		Repo:        "owner/repo",
		Channel:     "telegram",
		AccountID:   "account",
		IssueNumber: 42,
		IncludeBody: true,
	})
	if err == nil || !strings.Contains(err.Error(), "--include-body requires --out") {
		t.Fatalf("expected include-body file error, got %v", err)
	}
}
