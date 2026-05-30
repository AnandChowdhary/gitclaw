package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRunChannelDeliveryRecordsHashOnlyReceiptAndDedupes(t *testing.T) {
	cfg := DefaultConfig()
	sourceComment := RenderAssistantComment(Marker{
		RunID:          "run-delivery",
		EventID:        "issue-42",
		Model:          "openai/gpt-5-nano",
		IdempotencyKey: "delivery-key",
	}, "assistant reply body")
	github := &FakeGitHub{
		CommentsByIssue: map[int][]Comment{
			42: {{ID: 12345, Body: sourceComment}},
		},
	}
	opts := ChannelDeliveryOptions{
		Repo:              "octo/repo",
		Channel:           "Telegram",
		AccountID:         "delivery-account-secret",
		IssueNumber:       42,
		CommentID:         12345,
		ExternalMessageID: "external-message-secret",
		GatewayRunID:      "gateway-run-1",
	}

	result, err := RunChannelDelivery(context.Background(), cfg, github, opts)
	if err != nil {
		t.Fatalf("RunChannelDelivery returned error: %v", err)
	}
	if !result.CreatedStateIssue || !result.Delivered || result.Duplicate {
		t.Fatalf("unexpected delivery flags: %#v", result)
	}
	if result.StateIssueNumber == 0 || result.ReceiptCommentID == 0 || result.StateIssueURL == "" {
		t.Fatalf("missing delivery result fields: %#v", result)
	}
	if result.AccountHash == opts.AccountID || result.ExternalMessageHash == opts.ExternalMessageID {
		t.Fatalf("delivery hashes leaked raw values: %#v", result)
	}

	if len(github.Issues) != 1 {
		t.Fatalf("created %d state issues, want one", len(github.Issues))
	}
	issue := github.Issues[0]
	if !HasChannelStateMarker(issue.Body) {
		t.Fatalf("delivery should use channel state issue marker: %s", issue.Body)
	}
	if !hasLabel(github.IssueLabels[result.StateIssueNumber], cfg.ChannelLabel) {
		t.Fatalf("state issue missing channel label: %#v", github.IssueLabels[result.StateIssueNumber])
	}

	comments := github.CommentsByIssue[result.StateIssueNumber]
	if len(comments) != 1 {
		t.Fatalf("posted %d state comments, want one", len(comments))
	}
	receipt := comments[0].Body
	if !HasChannelDeliveryMarker(receipt) {
		t.Fatalf("delivery receipt missing marker: %s", receipt)
	}
	for _, want := range []string{
		`channel="telegram"`,
		`account_sha256_12="` + result.AccountHash + `"`,
		`issue_number="42"`,
		`source_comment_id="12345"`,
		`external_message_sha256_12="` + result.ExternalMessageHash + `"`,
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("delivery receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, raw := range []string{opts.AccountID, opts.ExternalMessageID, "assistant reply body"} {
		if strings.Contains(issue.Title, raw) || strings.Contains(issue.Body, raw) || strings.Contains(receipt, raw) {
			t.Fatalf("delivery leaked raw value %q:\ntitle=%s\nbody=%s\nreceipt=%s", raw, issue.Title, issue.Body, receipt)
		}
	}

	again, err := RunChannelDelivery(context.Background(), cfg, github, opts)
	if err != nil {
		t.Fatalf("duplicate RunChannelDelivery returned error: %v", err)
	}
	if again.CreatedStateIssue || again.Delivered || !again.Duplicate {
		t.Fatalf("duplicate delivery should reuse state without posting: %#v", again)
	}
	if got := len(github.CommentsByIssue[result.StateIssueNumber]); got != 1 {
		t.Fatalf("duplicate delivery posted %d receipts, want one", got)
	}
}

func TestRunChannelDeliveryRequiresAssistantSourceComment(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{
		CommentsByIssue: map[int][]Comment{
			42: {{ID: 12345, Body: "plain user comment"}},
		},
	}

	_, err := RunChannelDelivery(context.Background(), cfg, github, ChannelDeliveryOptions{
		Repo:              "octo/repo",
		Channel:           "slack",
		AccountID:         "workspace-secret",
		IssueNumber:       42,
		CommentID:         12345,
		ExternalMessageID: "external-message-secret",
	})
	if err == nil || !strings.Contains(err.Error(), "not a GitClaw assistant turn") {
		t.Fatalf("expected assistant marker error, got %v", err)
	}
	if len(github.Issues) != 0 {
		t.Fatalf("delivery should not create state issue for invalid source comment")
	}
}
