package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelNudgeQueuesNudgeWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-nudge-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-nudge-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-nudge-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels nudge release-captain --message-id nudge-inbound-901 --notify-message-id nudge-notify-901 --nudge-id nudge-secret-901 --tone gentle\nNote: Please take a look.\nDo not include this command hidden token in the receipt: CHANNEL_NUDGE_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-nudge-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-nudge-123",
				MessageID: "nudge-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored nudge command with CHANNEL_NUDGE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel nudge action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("nudge action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="nudge-notify-901"`,
		"GitClaw channel nudge.",
		"Target: release-captain",
		"Tone: gentle",
		"Note: Please take a look.",
		"Target hash: ",
		"Tone hash: ",
		"Note hash: ",
		"Nudge source: GitHub channel action.",
		"Model call: not performed by this action.",
		"Task issue: not created by this action.",
		"Reminder: not created by this action.",
		"Watch: not created by this action.",
		"Scheduled workflow: not created by this action.",
		"Repository mutation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("nudge notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_NUDGE_INGEST_MARKER", "CHANNEL_NUDGE_COMMAND_MARKER", "nudge-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("nudge notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Nudge Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels nudge`",
		"channel_nudge_status: `queued`",
		"nudge_mode: `structured-channel-nudge`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"nudge_id_sha256_12: `",
		"nudge_id_auto: `false`",
		"nudge_target_sha256_12: `",
		"nudge_target_bytes: `15`",
		"nudge_tone_sha256_12: `",
		"nudge_tone_bytes: `6`",
		"nudge_note_sha256_12: `",
		"nudge_note_bytes: `19`",
		"nudge_note_lines: `1`",
		"nudge_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"task_issue_created: `false`",
		"reminder_created: `false`",
		"watch_created: `false`",
		"scheduled_workflow_created: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_nudge_id_included: `false`",
		"raw_nudge_target_included: `false`",
		"raw_nudge_tone_included: `false`",
		"raw_nudge_note_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_nudge_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel nudge receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_NUDGE_INGEST_MARKER", "CHANNEL_NUDGE_COMMAND_MARKER", "chat-nudge-123", "nudge-inbound-901", "nudge-notify-901", "nudge-secret-901", "release-captain", "gentle", "Please take a look."} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel nudge receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-nudge-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-nudge-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels tap release-captain --message-id nudge-inbound-901 --notify-message-id nudge-notify-901 --nudge-id nudge-secret-901 --tone gentle\nNote: Please take a look.\nDo not leak duplicate token CHANNEL_NUDGE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate nudge created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate nudge posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels tap`",
		"channel_nudge_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"task_issue_created: `false`",
		"reminder_created: `false`",
		"watch_created: `false`",
		"scheduled_workflow_created: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate nudge receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_NUDGE_DUPLICATE_MARKER", "chat-nudge-123", "nudge-inbound-901", "nudge-notify-901", "nudge-secret-901", "release-captain", "gentle", "Please take a look."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate nudge receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelNudgeActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel nudge"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel heads-up --route team-demo --message-id source-1 --notify-message-id notify-1 --nudge-id Nudge.One --tone high
Note: Almost there.`,
		},
	}
	req, err := BuildChannelNudgeActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelNudgeActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "heads-up" || req.Options.Route != "team-demo" || req.Options.Target != "current-thread" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.NudgeID != "nudge-one" || req.Options.Tone != "urgent" {
		t.Fatalf("unexpected channel nudge parsing: %#v", req)
	}
	if req.Options.Note != "Almost there." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel nudge note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoNudgeID || req.RequestedRouteHash == "" || req.NudgeIDHash == "" || req.TargetSHA == "" || req.ToneSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route nudge hashes: %#v", req)
	}
}

func TestBuildChannelNudgeActionRequestParsesPositionalRouteAndTarget(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels bump team-demo release-captain --message-id source-2 --notify-message-id notify-2 --nudge-id Nudge.Two --tone normal",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelNudgeActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelNudgeActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Target != "release-captain" || req.Options.Tone != "normal" || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel nudge parsing: %#v", req)
	}
}

func TestIsChannelNudgeActionRequestDoesNotClaimProbePing(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 44, Title: "Channel probe"},
		Comment: &Comment{
			ID:   4401,
			Body: "@gitclaw /channels ping --route team-demo",
		},
	}
	if IsChannelNudgeActionRequest(ev, DefaultConfig()) {
		t.Fatal("/channels ping must remain outside channel nudge aliases")
	}
}
