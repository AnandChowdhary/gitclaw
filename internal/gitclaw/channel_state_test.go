package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRunChannelStateCreatesBodyFreeStateIssueAndDedupesOffset(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{}
	opts := ChannelStateOptions{
		Repo:       "octo/repo",
		Channel:    "Telegram",
		AccountID:  "account-secret-token",
		Offset:     "offset-secret-token",
		LeaseRunID: "run-123",
	}

	result, err := RunChannelState(context.Background(), cfg, github, opts)
	if err != nil {
		t.Fatalf("RunChannelState returned error: %v", err)
	}
	if !result.Created || !result.Updated || result.Duplicate {
		t.Fatalf("unexpected result flags: %#v", result)
	}
	if result.IssueNumber == 0 || result.CommentID == 0 || result.IssueURL == "" {
		t.Fatalf("missing issue/comment result fields: %#v", result)
	}
	if result.AccountHash == opts.AccountID || result.OffsetHash == opts.Offset {
		t.Fatalf("state hashes leaked raw values: %#v", result)
	}

	if len(github.Issues) != 1 {
		t.Fatalf("created %d issues, want one", len(github.Issues))
	}
	issue := github.Issues[0]
	if !HasChannelStateMarker(issue.Body) {
		t.Fatalf("state issue body missing marker: %s", issue.Body)
	}
	if !strings.Contains(issue.Body, `channel="telegram"`) || !strings.Contains(issue.Body, `account_sha256_12="`+result.AccountHash+`"`) {
		t.Fatalf("state issue marker missing normalized channel/hash: %s", issue.Body)
	}
	for _, raw := range []string{opts.AccountID, opts.Offset} {
		if strings.Contains(issue.Title, raw) || strings.Contains(issue.Body, raw) {
			t.Fatalf("state issue leaked raw state value %q:\ntitle=%s\nbody=%s", raw, issue.Title, issue.Body)
		}
	}
	if !hasLabel(github.IssueLabels[result.IssueNumber], cfg.ChannelLabel) {
		t.Fatalf("state issue missing channel label: %#v", github.IssueLabels[result.IssueNumber])
	}

	comments := github.CommentsByIssue[result.IssueNumber]
	if len(comments) != 1 {
		t.Fatalf("posted %d comments, want one", len(comments))
	}
	comment := comments[0].Body
	if !HasChannelStateUpdateMarker(comment) {
		t.Fatalf("state update comment missing marker: %s", comment)
	}
	if !strings.Contains(comment, `offset_sha256_12="`+result.OffsetHash+`"`) {
		t.Fatalf("state update comment missing offset hash: %s", comment)
	}
	for _, raw := range []string{opts.AccountID, opts.Offset} {
		if strings.Contains(comment, raw) {
			t.Fatalf("state update comment leaked raw state value %q:\n%s", raw, comment)
		}
	}

	again, err := RunChannelState(context.Background(), cfg, github, opts)
	if err != nil {
		t.Fatalf("duplicate RunChannelState returned error: %v", err)
	}
	if again.Created || again.Updated || !again.Duplicate {
		t.Fatalf("duplicate offset should reuse state without posting: %#v", again)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate run created %d issues, want one", len(github.Issues))
	}
	if got := len(github.CommentsByIssue[result.IssueNumber]); got != 1 {
		t.Fatalf("duplicate run posted %d comments, want one", got)
	}
}

func TestRunChannelStateCanCreateStateIssueWithoutOffsetUpdate(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{}
	opts := ChannelStateOptions{
		Repo:      "octo/repo",
		Channel:   "slack",
		AccountID: "workspace-secret",
	}

	result, err := RunChannelState(context.Background(), cfg, github, opts)
	if err != nil {
		t.Fatalf("RunChannelState returned error: %v", err)
	}
	if !result.Created || result.Updated || result.Duplicate || result.CommentID != 0 || result.OffsetHash != "" {
		t.Fatalf("unexpected result without offset: %#v", result)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created %d issues, want one", len(github.Issues))
	}
	if comments := github.CommentsByIssue[result.IssueNumber]; len(comments) != 0 {
		t.Fatalf("posted %d comments without offset, want none", len(comments))
	}
}
