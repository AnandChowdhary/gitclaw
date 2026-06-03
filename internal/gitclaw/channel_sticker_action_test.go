package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelStickerQueuesStickerWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-sticker-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-sticker-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-sticker-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels sticker confetti --message-id sticker-inbound-901 --notify-message-id sticker-notify-901 --sticker-id sticker-secret-901 --scale 4\nNote: Release handoff is steady.\nDo not include this command hidden token in the receipt: CHANNEL_STICKER_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-sticker-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-sticker-123",
				MessageID: "sticker-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored sticker command with CHANNEL_STICKER_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel sticker action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("sticker action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="sticker-notify-901"`,
		"GitClaw channel sticker.",
		"Sticker: confetti",
		"Scale: 4/5",
		"Note: Release handoff is steady.",
		"Sticker hash: ",
		"Note hash: ",
		"Sticker source: GitHub channel action.",
		"Model call: not performed by this action.",
		"Image generation: not performed by this action.",
		"Media fetch: not performed by this action.",
		"File upload: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("sticker notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_STICKER_INGEST_MARKER", "CHANNEL_STICKER_COMMAND_MARKER", "sticker-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("sticker notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Sticker Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels sticker`",
		"channel_sticker_status: `queued`",
		"sticker_mode: `structured-channel-sticker`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"sticker_id_sha256_12: `",
		"sticker_id_auto: `false`",
		"sticker_sha256_12: `",
		"sticker_bytes: `8`",
		"sticker_scale_level: `4`",
		"sticker_note_sha256_12: `",
		"sticker_note_bytes: `26`",
		"sticker_note_lines: `1`",
		"sticker_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"image_generation_performed: `false`",
		"media_fetch_performed: `false`",
		"file_upload_performed: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_sticker_id_included: `false`",
		"raw_sticker_included: `false`",
		"raw_sticker_note_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_sticker_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel sticker receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_STICKER_INGEST_MARKER", "CHANNEL_STICKER_COMMAND_MARKER", "chat-sticker-123", "sticker-inbound-901", "sticker-notify-901", "sticker-secret-901", "confetti", "Release handoff is steady."} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel sticker receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-sticker-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-sticker-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels stamp confetti --message-id sticker-inbound-901 --notify-message-id sticker-notify-901 --sticker-id sticker-secret-901 --scale 4\nNote: Release handoff is steady.\nDo not leak duplicate token CHANNEL_STICKER_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate sticker created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate sticker posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels stamp`",
		"channel_sticker_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"image_generation_performed: `false`",
		"media_fetch_performed: `false`",
		"file_upload_performed: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate sticker receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_STICKER_DUPLICATE_MARKER", "chat-sticker-123", "sticker-inbound-901", "sticker-notify-901", "sticker-secret-901", "confetti", "Release handoff is steady."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate sticker receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelStickerActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel sticker"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel badge --route team-demo --message-id source-1 --notify-message-id notify-1 --sticker-id Sticker.One --scale 5
Note: Almost there.`,
		},
	}
	req, err := BuildChannelStickerActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelStickerActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "badge" || req.Options.Route != "team-demo" || req.Options.Sticker != "badge" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StickerID != "sticker-one" || req.Options.Scale != 5 {
		t.Fatalf("unexpected channel sticker parsing: %#v", req)
	}
	if req.Options.Note != "Almost there." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel sticker note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStickerID || req.RequestedRouteHash == "" || req.StickerIDHash == "" || req.StickerSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route sticker hashes: %#v", req)
	}
}

func TestBuildChannelStickerActionRequestParsesPositionalRouteAndSticker(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels stamp team-demo ship-it --message-id source-2 --notify-message-id notify-2 --sticker-id Sticker.Two --scale 2",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelStickerActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelStickerActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Sticker != "ship-it" || req.Options.Scale != 2 || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel sticker parsing: %#v", req)
	}
}
