package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToastQueuesCelebrationWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-toast-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-toast-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-toast-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels toast launch-ready --message-id toast-inbound-901 --notify-message-id toast-notify-901 --toast-id toast-secret-901 --tone bright\nReason: Release handoff is steady.\nDo not include this command hidden token in the receipt: CHANNEL_TOAST_COMMAND_MARKER.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 901,
			Title:  "GitClaw telegram thread chat-toast-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-toast-123",
				MessageID: "toast-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored toast command with CHANNEL_TOAST_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel toast action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("toast action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="toast-notify-901"`,
		"GitClaw channel toast.",
		"Toast: launch-ready",
		"Tone: bright",
		"Reason: Release handoff is steady.",
		"Toast hash: ",
		"Reason hash: ",
		"Toast source: GitHub channel action.",
		"Model call: not performed by this action.",
		"Kudos issue: not created by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("toast notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOAST_INGEST_MARKER", "CHANNEL_TOAST_COMMAND_MARKER", "toast-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("toast notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Toast Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels toast`",
		"channel_toast_status: `queued`",
		"toast_mode: `provider-facing-celebration`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"toast_id_sha256_12: `",
		"toast_id_auto: `false`",
		"toast_title_sha256_12: `",
		"toast_title_bytes: `12`",
		"toast_title_lines: `1`",
		"toast_title_source: `positional-title`",
		"toast_reason_sha256_12: `",
		"toast_reason_bytes: `26`",
		"toast_reason_lines: `1`",
		"toast_reason_source: `trailing-reason`",
		"toast_tone_sha256_12: `",
		"toast_tone_bytes: `6`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"kudos_issue_created: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_toast_id_included: `false`",
		"raw_toast_title_included: `false`",
		"raw_toast_reason_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_toast_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel toast receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOAST_INGEST_MARKER", "CHANNEL_TOAST_COMMAND_MARKER", "chat-toast-123", "toast-inbound-901", "toast-notify-901", "toast-secret-901", "launch-ready", "Release handoff is steady."} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel toast receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-toast-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-toast-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels cheer launch-ready --message-id toast-inbound-901 --notify-message-id toast-notify-901 --toast-id toast-secret-901 --tone bright\nReason: Release handoff is steady.\nDo not leak duplicate token CHANNEL_TOAST_DUPLICATE_MARKER.",
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
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate toast created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate toast posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels cheer`",
		"channel_toast_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"kudos_issue_created: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate toast receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOAST_DUPLICATE_MARKER", "chat-toast-123", "toast-inbound-901", "toast-notify-901", "toast-secret-901", "launch-ready", "Release handoff is steady."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate toast receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelToastActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel toast"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel ship-it --route team-demo --message-id source-1 --notify-message-id notify-1 --toast-id Toast.One --tone victory
Toast: Release crossed the line.
Reason: Tests are green.`,
		},
	}
	req, err := BuildChannelToastActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelToastActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "ship-it" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.ToastID != "toast-one" || req.Options.Title != "Release crossed the line." || req.Options.Reason != "Tests are green." || req.Options.Tone != "victory" {
		t.Fatalf("unexpected channel toast parsing: %#v", req)
	}
	if req.TitleSource != "trailing-title" || req.ReasonSource != "trailing-reason" {
		t.Fatalf("unexpected channel toast text source parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoToastID || req.RequestedRouteHash == "" || req.ToastIDHash == "" || req.TitleSHA == "" || req.ReasonSHA == "" || req.ToneSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route toast hashes: %#v", req)
	}
}

func TestBuildChannelToastActionRequestParsesPositionalRouteAndTitle(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels cheers team-demo launch-complete --message-id source-2 --notify-message-id notify-2 --toast-id Toast.Two --tone warm",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelToastActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelToastActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Title != "launch-complete" || req.Options.Tone != "warm" || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel toast parsing: %#v", req)
	}
}
