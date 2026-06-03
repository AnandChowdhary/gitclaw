package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelBingoQueuesMiniGameWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-bingo-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 906,
			"title": "GitClaw telegram thread chat-bingo-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-bingo-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90601,
			"body": "@gitclaw /channels bingo release --message-id bingo-inbound-906 --notify-message-id bingo-notify-906 --bingo-id bingo-secret-906 --size 3\nNote: Ship room warmup.\nDo not include this command hidden token in the receipt: CHANNEL_BINGO_COMMAND_MARKER.",
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
			Number: 906,
			Title:  "GitClaw telegram thread chat-bingo-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{906: {{
			ID: 90600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-bingo-123",
				MessageID: "bingo-inbound-906",
				Author:    "telegram",
				Body:      "Original mirrored bingo command with CHANNEL_BINGO_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{906: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel bingo action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("bingo action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[906]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="bingo-notify-906"`,
		"GitClaw channel bingo.",
		"Theme: release",
		"Grid: 3x3",
		"Note: Ship room warmup.",
		"- [ ] ",
		"Free: keep it on GitHub",
		"Bingo hash: ",
		"Theme hash: ",
		"Note hash: ",
		"Bingo source: GitHub channel action.",
		"Model call: not performed by this action.",
		"External randomness: not used by this action.",
		"Game state: not persisted by this action.",
		"Score tracking: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("bingo notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BINGO_INGEST_MARKER", "CHANNEL_BINGO_COMMAND_MARKER", "bingo-secret-906"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("bingo notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Bingo Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels bingo`",
		"channel_bingo_status: `queued`",
		"bingo_mode: `provider-facing-deterministic-mini-game`",
		"notification_target_issue: `#906`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"bingo_id_sha256_12: `",
		"bingo_id_auto: `false`",
		"bingo_theme_sha256_12: `",
		"bingo_theme_bytes: `7`",
		"bingo_theme_source: `positional-theme`",
		"bingo_grid_size: `3`",
		"bingo_cell_count: `9`",
		"bingo_board_sha256_12: `",
		"bingo_note_sha256_12: `",
		"bingo_note_bytes: `17`",
		"bingo_note_lines: `1`",
		"bingo_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"external_randomness_used: `false`",
		"game_state_persisted: `false`",
		"score_tracking_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_bingo_id_included: `false`",
		"raw_bingo_theme_included: `false`",
		"raw_bingo_note_included: `false`",
		"raw_bingo_board_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_bingo_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel bingo receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BINGO_INGEST_MARKER", "CHANNEL_BINGO_COMMAND_MARKER", "chat-bingo-123", "bingo-inbound-906", "bingo-notify-906", "bingo-secret-906", "release", "Ship room warmup."} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel bingo receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 906,
			"title": "GitClaw telegram thread chat-bingo-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-bingo-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90602,
			"body": "@gitclaw /channels channel-bingo release --message-id bingo-inbound-906 --notify-message-id bingo-notify-906 --bingo-id bingo-secret-906 --size 3\nNote: Ship room warmup.\nDo not leak duplicate token CHANNEL_BINGO_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate bingo created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[906]); got != 4 {
		t.Fatalf("duplicate bingo posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[906])
	}
	duplicateReceipt := github.CommentsByIssue[906][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels channel-bingo`",
		"channel_bingo_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"external_randomness_used: `false`",
		"game_state_persisted: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel bingo receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BINGO_DUPLICATE_MARKER", "chat-bingo-123", "bingo-inbound-906", "bingo-notify-906", "bingo-secret-906", "release", "Ship room warmup."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel bingo receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBingoActionRequestParsesRouteAliasAndSize(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel bingo"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel icebreaker-bingo --route team-demo --message-id source-1 --notify-message-id notify-1 --bingo-id Bingo.One --theme triage --size 4
Note: Keep it gentle.`,
		},
	}
	req, err := BuildChannelBingoActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBingoActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "icebreaker-bingo" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.BingoID != "bingo-one" || req.Options.Theme != "triage" || req.Options.GridSize != 4 || req.Options.Note != "Keep it gentle." {
		t.Fatalf("unexpected channel bingo parsing: %#v", req)
	}
	if req.ThemeSource != "flag" || req.NoteSource != "trailing-note" || req.BoardCellCount != 16 || req.GridSize != 4 {
		t.Fatalf("unexpected channel bingo source/grid parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoBingoID || req.RequestedRouteHash == "" || req.BingoIDHash == "" || req.ThemeSHA == "" || req.BoardSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route bingo hashes: %#v", req)
	}
}
