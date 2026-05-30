package gitclaw

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunChannelGatewayRecordsHashOnlyLeaseAndDedupes(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{}
	opts := ChannelGatewayOptions{
		Repo:        "octo/repo",
		Channel:     "Telegram",
		AccountID:   "gateway-account-secret",
		GatewaySlot: "slot-20260530",
		LeaseRunID:  "run-abc",
		Renew:       true,
	}

	result, err := RunChannelGateway(context.Background(), cfg, github, opts, time.Time{})
	if err != nil {
		t.Fatalf("RunChannelGateway returned error: %v", err)
	}
	if !result.Created || !result.Updated || result.Duplicate || !result.Renew {
		t.Fatalf("unexpected gateway result flags: %#v", result)
	}
	if result.GatewaySlot != opts.GatewaySlot || result.AccountHash == "" || result.LeaseHash == "" {
		t.Fatalf("missing gateway result metadata: %#v", result)
	}
	if result.AccountHash == opts.AccountID || result.LeaseHash == channelGatewayLeaseOffset(normalizeChannelGatewayOptions(opts, time.Time{})) {
		t.Fatalf("gateway hashes leaked raw values: %#v", result)
	}

	if len(github.Issues) != 1 {
		t.Fatalf("created %d issues, want one", len(github.Issues))
	}
	issue := github.Issues[0]
	if !HasChannelStateMarker(issue.Body) {
		t.Fatalf("gateway should use channel state marker: %s", issue.Body)
	}
	comments := github.CommentsByIssue[result.IssueNumber]
	if len(comments) != 1 {
		t.Fatalf("posted %d comments, want one", len(comments))
	}
	comment := comments[0].Body
	if !HasChannelStateUpdateMarker(comment) {
		t.Fatalf("gateway lease comment missing channel state update marker: %s", comment)
	}
	if !strings.Contains(comment, `offset_sha256_12="`+result.LeaseHash+`"`) {
		t.Fatalf("gateway lease comment missing lease hash: %s", comment)
	}
	for _, raw := range []string{opts.AccountID, channelGatewayLeaseOffset(normalizeChannelGatewayOptions(opts, time.Time{}))} {
		if strings.Contains(issue.Title, raw) || strings.Contains(issue.Body, raw) || strings.Contains(comment, raw) {
			t.Fatalf("gateway leaked raw state value %q:\ntitle=%s\nbody=%s\ncomment=%s", raw, issue.Title, issue.Body, comment)
		}
	}

	again, err := RunChannelGateway(context.Background(), cfg, github, opts, time.Time{})
	if err != nil {
		t.Fatalf("duplicate RunChannelGateway returned error: %v", err)
	}
	if again.Created || again.Updated || !again.Duplicate || !again.Renew {
		t.Fatalf("duplicate gateway lease should reuse state without posting: %#v", again)
	}
	if got := len(github.CommentsByIssue[result.IssueNumber]); got != 1 {
		t.Fatalf("duplicate gateway run posted %d comments, want one", got)
	}
}

func TestRunChannelGatewayDefaultsSlotAndLeaseRunID(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{}
	now := time.Date(2026, 5, 30, 10, 11, 12, 0, time.UTC)

	result, err := RunChannelGateway(context.Background(), cfg, github, ChannelGatewayOptions{
		Repo:      "octo/repo",
		Channel:   "slack",
		AccountID: "workspace-secret",
	}, now)
	if err != nil {
		t.Fatalf("RunChannelGateway returned error: %v", err)
	}
	if result.GatewaySlot != "20260530T101112Z" {
		t.Fatalf("unexpected default gateway slot: %#v", result)
	}
	if !result.Created || !result.Updated || result.Duplicate || result.Renew {
		t.Fatalf("unexpected default gateway result flags: %#v", result)
	}
}
