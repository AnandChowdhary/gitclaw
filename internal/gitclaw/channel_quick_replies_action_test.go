package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelQuickRepliesQueuesReplyChipsWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-quick-replies-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-quick-replies-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-quick-replies-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels quick-replies handoff --message-id quick-replies-inbound-901 --notify-message-id quick-replies-notify-901 --reply-id quick-replies-secret-901\nNote: Make replies easy.\nDo not include this command hidden token in the receipt: CHANNEL_QUICK_REPLIES_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-quick-replies-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-quick-replies-123",
				MessageID: "quick-replies-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored quick-replies command with CHANNEL_QUICK_REPLIES_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel quick replies action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("quick replies action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="quick-replies-notify-901"`,
		"GitClaw channel quick replies.",
		"Lane: handoff",
		"Reply chips:",
		"1. Pulse - `/channels room-pulse handoff --pulse-id <id> --message-id <id> --notify-message-id <id>`",
		"check whether the room is ready for handoff",
		"2. Handoff - `/channels handoff --id <handoff-id> --message-id <id> --notify-message-id <id>`",
		"open a session handoff from this channel thread",
		"3. Nudge - `/channels nudge release-captain --nudge-id <id> --message-id <id> --notify-message-id <id> --tone gentle`",
		"ask the next human to look without creating a task",
		"4. Mood - `/channels mood focused --message-id <id> --notify-message-id <id> --intensity 4`",
		"mark the room posture without invoking a model",
		"Note: Make replies easy.",
		"Note hash: ",
		"Quick replies hash: ",
		"Quick replies source: GitHub channel action.",
		"Command execution: not performed by this action.",
		"Artifact issue creation: not performed by this action.",
		"Task/reminder creation: not performed by this action.",
		"Skill install: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Model call: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("quick replies notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_QUICK_REPLIES_INGEST_MARKER", "CHANNEL_QUICK_REPLIES_COMMAND_MARKER", "quick-replies-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("quick replies notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Quick Replies Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels quick-replies`",
		"channel_quick_replies_status: `queued`",
		"quick_replies_mode: `provider-facing-reply-chips`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"quick_replies_id_sha256_12: `",
		"quick_replies_id_auto: `false`",
		"quick_replies_lane_sha256_12: `",
		"quick_replies_lane_bytes: `7`",
		"quick_replies_lane_terms: `1`",
		"quick_replies_lane_source: `positional`",
		"quick_replies_option_count: `4`",
		"quick_replies_options_sha256_12: `",
		"quick_replies_note_sha256_12: `",
		"quick_replies_note_bytes: `18`",
		"quick_replies_note_lines: `1`",
		"quick_replies_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"artifact_issue_created: `false`",
		"task_issue_created: `false`",
		"reminder_created: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_quick_replies_id_included: `false`",
		"raw_quick_replies_lane_included: `false`",
		"raw_quick_replies_note_included: `false`",
		"raw_quick_replies_options_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_quick_replies_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel quick replies receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_QUICK_REPLIES_INGEST_MARKER", "CHANNEL_QUICK_REPLIES_COMMAND_MARKER", "chat-quick-replies-123", "quick-replies-inbound-901", "quick-replies-notify-901", "quick-replies-secret-901", "handoff", "Make replies easy.", "/channels room-pulse handoff"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel quick replies receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-quick-replies-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-quick-replies-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels reply-chips handoff --message-id quick-replies-inbound-901 --notify-message-id quick-replies-notify-901 --reply-id quick-replies-secret-901\nNote: Make replies easy.\nDo not leak duplicate token CHANNEL_QUICK_REPLIES_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate quick replies created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate quick replies posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels reply-chips`",
		"channel_quick_replies_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"artifact_issue_created: `false`",
		"task_issue_created: `false`",
		"reminder_created: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate quick replies receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_QUICK_REPLIES_DUPLICATE_MARKER", "chat-quick-replies-123", "quick-replies-inbound-901", "quick-replies-notify-901", "quick-replies-secret-901", "handoff", "Make replies easy.", "/channels room-pulse handoff"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate quick replies receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelQuickRepliesActionRequestParsesRouteAliasAndLane(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel quick replies"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel reply-options team-demo skill --message-id source-1 --notify-message-id notify-1 --reply-id Replies.One
Note: Pick one.`,
		},
	}
	req, err := BuildChannelQuickRepliesActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelQuickRepliesActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "reply-options" || req.Options.Route != "team-demo" || req.Options.Lane != "skills" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.ReplyID != "replies-one" || req.OptionCount != 4 {
		t.Fatalf("unexpected channel quick replies parsing: %#v", req)
	}
	if req.Options.Note != "Pick one." || req.NoteSource != "trailing-note" || req.LaneSource != "positional" {
		t.Fatalf("unexpected channel quick replies note/lane parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoReplyID || req.RequestedRouteHash == "" || req.ReplyIDHash == "" || req.LaneSHA == "" || req.OptionsSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route quick replies hashes: %#v", req)
	}
}
