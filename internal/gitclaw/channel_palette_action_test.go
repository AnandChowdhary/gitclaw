package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelPaletteQueuesPaletteWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-palette-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-palette-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-palette-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels palette fun --message-id palette-inbound-901 --notify-message-id palette-notify-901 --palette-id palette-secret-901\nNote: Use this tiny launcher.\nDo not include this command hidden token in the receipt: CHANNEL_PALETTE_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-palette-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-palette-123",
				MessageID: "palette-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored palette command with CHANNEL_PALETTE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel palette action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("palette action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="palette-notify-901"`,
		"GitClaw channel palette.",
		"Lane: fun",
		"Shortcuts:",
		"/channels roll --dice 2d6 --message-id <id> --notify-message-id <id>",
		"/channels choose --message-id <id> --notify-message-id <id>",
		"/channels mood <mood> --message-id <id> --notify-message-id <id>",
		"/channels sticker <sticker> --sticker-id <id> --message-id <id> --notify-message-id <id>",
		"/channels nudge <target> --nudge-id <id> --message-id <id> --notify-message-id <id>",
		"Note: Use this tiny launcher.",
		"Palette hash: ",
		"Note hash: ",
		"Palette source: GitHub channel action.",
		"Model call: not performed by this action.",
		"Command execution: not performed by this action.",
		"Skill install: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Backup payload read: not performed by this action.",
		"Soul body read: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("palette notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_PALETTE_INGEST_MARKER", "CHANNEL_PALETTE_COMMAND_MARKER", "palette-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("palette notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Palette Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels palette`",
		"channel_palette_status: `queued`",
		"palette_mode: `structured-channel-command-palette`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"palette_id_sha256_12: `",
		"palette_id_auto: `false`",
		"palette_lane_sha256_12: `",
		"palette_lane_bytes: `3`",
		"palette_command_count: `5`",
		"palette_note_sha256_12: `",
		"palette_note_bytes: `23`",
		"palette_note_lines: `1`",
		"palette_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_palette_id_included: `false`",
		"raw_palette_lane_included: `false`",
		"raw_palette_note_included: `false`",
		"raw_palette_commands_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_palette_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel palette receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PALETTE_INGEST_MARKER", "CHANNEL_PALETTE_COMMAND_MARKER", "chat-palette-123", "palette-inbound-901", "palette-notify-901", "palette-secret-901", "Use this tiny launcher.", "/channels roll --dice 2d6"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel palette receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-palette-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-palette-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels menu fun --message-id palette-inbound-901 --notify-message-id palette-notify-901 --palette-id palette-secret-901\nNote: Use this tiny launcher.\nDo not leak duplicate token CHANNEL_PALETTE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate palette created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate palette posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels menu`",
		"channel_palette_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate palette receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PALETTE_DUPLICATE_MARKER", "chat-palette-123", "palette-inbound-901", "palette-notify-901", "palette-secret-901", "Use this tiny launcher.", "/channels roll --dice 2d6"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate palette receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelPaletteActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel palette"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel shortcuts --route team-demo --lane skill --message-id source-1 --notify-message-id notify-1 --palette-id Palette.One
Note: Almost there.`,
		},
	}
	req, err := BuildChannelPaletteActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPaletteActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "shortcuts" || req.Options.Route != "team-demo" || req.Options.Lane != "skills" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.PaletteID != "palette-one" || req.CommandCount != 4 {
		t.Fatalf("unexpected channel palette parsing: %#v", req)
	}
	if req.Options.Note != "Almost there." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel palette note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoPaletteID || req.RequestedRouteHash == "" || req.PaletteIDHash == "" || req.LaneSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route palette hashes: %#v", req)
	}
}

func TestBuildChannelPaletteActionRequestParsesPositionalRouteAndLane(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels command-palette team-demo backups --message-id source-2 --notify-message-id notify-2 --palette-id Palette.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelPaletteActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPaletteActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Lane != "backups" || req.CommandCount != 4 || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel palette parsing: %#v", req)
	}
}
