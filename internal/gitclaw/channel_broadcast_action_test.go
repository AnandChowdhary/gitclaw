package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelBroadcastQueuesMultipleRoutesWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: e2e-slack-route
    channel: slack
    thread_id_template: slack-broadcast-{route}-{message_id}
    author: gitclaw:test
  - name: e2e-telegram-route
    channel: telegram
    thread_id_template: telegram-broadcast-{route}-{message_id}
    author: gitclaw:test
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 240,
			"title": "@gitclaw /channels broadcast e2e-slack-route,e2e-telegram-route --message-id broadcast-1",
			"body": "Broadcast this outbound body.\n\nCHANNEL_BROADCAST_BODY_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{240: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel broadcast action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want two channel issues: %#v", len(github.Issues), github.Issues)
	}
	for _, issue := range github.Issues {
		if !HasChannelThreadMarker(issue.Body) {
			t.Fatalf("broadcast target issue missing channel thread marker: %#v", issue)
		}
		comments := github.CommentsByIssue[issue.Number]
		if len(comments) != 1 {
			t.Fatalf("target issue %d comments = %d, want one outbound: %#v", issue.Number, len(comments), comments)
		}
		if !strings.Contains(comments[0].Body, "CHANNEL_BROADCAST_BODY_SECRET") ||
			!strings.Contains(comments[0].Body, `message_id="broadcast-1"`) ||
			!strings.Contains(comments[0].Body, "gitclaw:channel-outbound") {
			t.Fatalf("target issue %d missing outbound body/marker:\n%s", issue.Number, comments[0].Body)
		}
	}

	sourceComments := github.CommentsByIssue[240]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want action receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Channel Broadcast Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels broadcast`",
		"channel_broadcast_status: `queued`",
		"broadcast_routes: `2`",
		"broadcast_queued: `2`",
		"broadcast_duplicates: `0`",
		"target_issues_created: `2`",
		"raw_route_names_included: `false`",
		"raw_outbound_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_broadcast_action_change: `true`",
		"channel=`slack`",
		"channel=`telegram`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("broadcast receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BROADCAST_BODY_SECRET", "Broadcast this outbound body", "e2e-slack-route", "e2e-telegram-route", "broadcast-1"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("broadcast receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 240,
			"title": "@gitclaw /channels broadcast e2e-slack-route,e2e-telegram-route --message-id broadcast-1",
			"body": "Broadcast this outbound body.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 24001,
			"body": "@gitclaw /channels broadcast e2e-slack-route,e2e-telegram-route --message-id broadcast-1\n\nRepeat body.\n\nCHANNEL_BROADCAST_DUPLICATE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, cfg, github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate broadcast created more target issues: %#v", github.Issues)
	}
	for _, issue := range github.Issues {
		if got := len(github.CommentsByIssue[issue.Number]); got != 1 {
			t.Fatalf("duplicate broadcast posted another outbound comment on issue %d: %d", issue.Number, got)
		}
	}
	duplicateReceipt := github.CommentsByIssue[240][1].Body
	for _, want := range []string{
		"channel_broadcast_status: `duplicate`",
		"broadcast_queued: `0`",
		"broadcast_duplicates: `2`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate broadcast receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_BROADCAST_DUPLICATE_SECRET") {
		t.Fatalf("duplicate broadcast receipt leaked body:\n%s", duplicateReceipt)
	}
}
