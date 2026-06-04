package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelArcadeQueuesPlayMenuWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo with CHANNEL_ARCADE_DOC_SECRET\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-arcade-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 915,
			"title": "GitClaw telegram thread chat-arcade-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-arcade-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91501,
			"body": "@gitclaw /channels arcade fun --message-id arcade-inbound-915 --notify-message-id arcade-notify-915 --arcade-id Arcade.Secret.915\nNote: Pick the room move\nDo not include this command hidden token in the receipt: CHANNEL_ARCADE_COMMAND_MARKER.",
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
			Number: 915,
			Title:  "GitClaw telegram thread chat-arcade-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{915: {{
			ID: 91500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-arcade-123",
				MessageID: "arcade-inbound-915",
				Author:    "telegram",
				Body:      "Original mirrored arcade command with CHANNEL_ARCADE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{915: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel arcade action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("arcade action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[915]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="arcade-notify-915"`,
		"GitClaw channel arcade.",
		"Mode: fun",
		"Frame: Pick one bounded move; GitHub keeps the receipt.",
		"Moves:",
		"Story dice [ready] - start a tiny prompt-game card",
		"Try: `@gitclaw /channels story-dice fun --story-dice-id <id> --message-id <id> --notify-message-id <id>`",
		"Spark [ready] - turn the room toward one experiment",
		"Try: `@gitclaw /channels spark --spark-id <id> --message-id <id> --notify-message-id <id>`",
		"Postcard [ready] - send a small scene card",
		"Try: `@gitclaw /channels postcard launch-ready --postcard-id <id> --message-id <id> --notify-message-id <id>`",
		"Cockpit [watch] - switch from play to operator view",
		"Try: `@gitclaw /channels cockpit fun --cockpit-id <id> --message-id <id> --notify-message-id <id>`",
		"Note: Pick the room move",
		"Arcade hash: ",
		"Move hash: ",
		"Note hash: ",
		"Arcade persistence: advisory only; no score or game state changed.",
		"Arcade source: bounded GitHub channel action deck.",
		"Model call: not performed by this action.",
		"Dynamic play generation: not performed by this action.",
		"External randomness: not used by this action.",
		"Game-state persistence: not performed by this action.",
		"Score tracking: not performed by this action.",
		"Command execution: not performed by this action.",
		"Skill install: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Backup payload read: not performed by this action.",
		"Soul body read: not performed by this action.",
		"Memory write: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Policy mutation: not performed by this action.",
		"Schedule creation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("arcade notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_ARCADE_INGEST_MARKER", "CHANNEL_ARCADE_COMMAND_MARKER", "CHANNEL_ARCADE_DOC_SECRET", "Arcade.Secret.915"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("arcade notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Arcade Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels arcade`",
		"channel_arcade_status: `queued`",
		"arcade_card_mode: `bounded-channel-play-menu`",
		"notification_target_issue: `#915`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"arcade_id_sha256_12: `",
		"arcade_id_auto: `false`",
		"arcade_mode_sha256_12: `",
		"arcade_mode_bytes: `3`",
		"arcade_move_count: `4`",
		"arcade_move_sha256_12: `",
		"arcade_note_sha256_12: `",
		"arcade_note_bytes: `18`",
		"arcade_note_lines: `1`",
		"arcade_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"dynamic_play_generation_performed: `false`",
		"external_randomness_used: `false`",
		"game_state_persisted: `false`",
		"score_tracking_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"memory_write_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"policy_mutation_performed: `false`",
		"schedule_created: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_arcade_id_included: `false`",
		"raw_arcade_mode_included: `false`",
		"raw_arcade_note_included: `false`",
		"raw_arcade_moves_included: `false`",
		"raw_arcade_commands_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_arcade_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel arcade receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"fun", "Story dice", "CHANNEL_ARCADE_INGEST_MARKER", "CHANNEL_ARCADE_COMMAND_MARKER", "CHANNEL_ARCADE_DOC_SECRET", "chat-arcade-123", "arcade-inbound-915", "arcade-notify-915", "Arcade.Secret.915", "Pick the room move", "start a tiny prompt-game card"} {
		if strings.Contains(strings.ToLower(receipt), strings.ToLower(leaked)) {
			t.Fatalf("channel arcade receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 915,
			"title": "GitClaw telegram thread chat-arcade-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-arcade-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91502,
			"body": "@gitclaw /channels play-menu fun --message-id arcade-inbound-915 --notify-message-id arcade-notify-915 --arcade-id Arcade.Secret.915\nNote: Pick the room move\nDo not leak duplicate token CHANNEL_ARCADE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate arcade created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[915]); got != 4 {
		t.Fatalf("duplicate arcade posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[915])
	}
	duplicateReceipt := github.CommentsByIssue[915][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels play-menu`",
		"channel_arcade_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"dynamic_play_generation_performed: `false`",
		"external_randomness_used: `false`",
		"game_state_persisted: `false`",
		"score_tracking_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"memory_write_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"policy_mutation_performed: `false`",
		"schedule_created: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate arcade receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"fun", "Story dice", "CHANNEL_ARCADE_DUPLICATE_MARKER", "chat-arcade-123", "arcade-inbound-915", "arcade-notify-915", "Arcade.Secret.915", "Pick the room move", "start a tiny prompt-game card"} {
		if strings.Contains(strings.ToLower(duplicateReceipt), strings.ToLower(leaked)) {
			t.Fatalf("duplicate arcade receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelArcadeActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel arcade"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel play-menu --route team-demo --mode tools --message-id source-1 --notify-message-id notify-1 --arcade-id Arcade.One
Note: Keep the tool game visible.`,
		},
	}
	req, err := BuildChannelArcadeActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelArcadeActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "play-menu" || req.Options.Route != "team-demo" || req.Options.Mode != "tools" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.ArcadeID != "arcade-one" || req.MoveCount != 4 {
		t.Fatalf("unexpected channel arcade parsing: %#v", req)
	}
	if req.NoteSource != "trailing-note" || req.Options.Note != "Keep the tool game visible." || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoArcadeID {
		t.Fatalf("unexpected channel arcade defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.ArcadeIDHash == "" || req.ModeSHA == "" || req.NoteSHA == "" || req.MoveSHA == "" || req.NotificationBodySHA == "" || req.NotificationBytes == 0 || req.NotificationLines == 0 {
		t.Fatalf("expected arcade hashes and notification metadata: %#v", req)
	}
	if !IsChannelArcadeActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel play-menu alias to be recognized")
	}
}
