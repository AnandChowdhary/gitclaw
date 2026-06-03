package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelRoomPulseQueuesMetadataPulseWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-room-pulse-123",
	})
	commandBody := "@gitclaw /channels room-pulse handoff --message-id room-pulse-inbound-901 --notify-message-id room-pulse-notify-901 --pulse-id room-pulse-secret-901\nNote: Keep the room warm.\nDo not include this command hidden token in the receipt: CHANNEL_ROOM_PULSE_COMMAND_MARKER."
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-room-pulse-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-room-pulse-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels room-pulse handoff --message-id room-pulse-inbound-901 --notify-message-id room-pulse-notify-901 --pulse-id room-pulse-secret-901\nNote: Keep the room warm.\nDo not include this command hidden token in the receipt: CHANNEL_ROOM_PULSE_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-room-pulse-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {
			{
				ID: 90100,
				Body: RenderChannelMessageComment(ChannelIngestOptions{
					Channel:   "telegram",
					ThreadID:  "chat-room-pulse-123",
					MessageID: "room-pulse-inbound-901",
					Author:    "telegram",
					Body:      "Original mirrored room pulse command with CHANNEL_ROOM_PULSE_INGEST_MARKER.",
				}),
			},
			{
				ID:   90101,
				Body: RenderAssistantComment(Marker{RunID: "local", EventID: "initial", Model: "gitclaw/channels"}, "GitClaw Channel Report\n\n- channel_thread_issue: `true`"),
			},
			{
				ID:   90102,
				Body: commandBody,
			},
		}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel room pulse action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("room pulse action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 5 {
		t.Fatalf("source comments = %d, want message + report + command + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[3].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="room-pulse-notify-901"`,
		"GitClaw channel room pulse.",
		"Pulse: active",
		"Focus: handoff",
		"Comments observed: 3",
		"Mirrored channel messages: 1",
		"Assistant turns: 1",
		"Outbound cards: 0",
		"Status cards: 0",
		"Activity signals: 0",
		"Error markers: 0",
		"User commands: 1",
		"Other comments: 0",
		"Last observed kind: user-command",
		"Suggested next step: `/channels handoff --id <handoff-id> --message-id <id> --notify-message-id <id>`",
		"Note: Keep the room warm.",
		"Note hash: ",
		"Room pulse hash: ",
		"Suggested step hash: ",
		"Pulse source: GitHub channel issue metadata and GitClaw markers.",
		"Raw issue/comment bodies: not included.",
		"Model call: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Task/reminder creation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("room pulse notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_ROOM_PULSE_INGEST_MARKER", "CHANNEL_ROOM_PULSE_COMMAND_MARKER", "room-pulse-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("room pulse notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[4].Body
	for _, want := range []string{
		"GitClaw Channel Room Pulse Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels room-pulse`",
		"channel_room_pulse_status: `queued`",
		"room_pulse_mode: `metadata-only-channel-presence`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"room_pulse_id_sha256_12: `",
		"room_pulse_id_auto: `false`",
		"room_pulse_focus_sha256_12: `",
		"room_pulse_focus_bytes: `7`",
		"room_pulse_focus_terms: `1`",
		"room_pulse_focus_source: `positional`",
		"room_pulse_note_sha256_12: `",
		"room_pulse_note_bytes: `19`",
		"room_pulse_note_lines: `1`",
		"room_pulse_note_source: `trailing-note`",
		"room_pulse_state: `active`",
		"room_pulse_total_comments: `3`",
		"room_pulse_channel_messages: `1`",
		"room_pulse_assistant_turns: `1`",
		"room_pulse_outbound_cards: `0`",
		"room_pulse_status_cards: `0`",
		"room_pulse_activity_signals: `0`",
		"room_pulse_error_markers: `0`",
		"room_pulse_user_commands: `1`",
		"room_pulse_other_comments: `0`",
		"room_pulse_last_observed_kind: `user-command`",
		"room_pulse_snapshot_sha256_12: `",
		"room_pulse_next_step_sha256_12: `",
		"notification_body_sha256_12: `",
		"comment_body_read_performed: `true`",
		"model_call_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"task_issue_created: `false`",
		"reminder_created: `false`",
		"scheduled_workflow_created: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_room_pulse_id_included: `false`",
		"raw_room_pulse_focus_included: `false`",
		"raw_room_pulse_note_included: `false`",
		"raw_room_pulse_next_step_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_body_included: `false`",
		"raw_comment_bodies_included: `false`",
		"llm_e2e_required_after_channel_room_pulse_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel room pulse receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ROOM_PULSE_INGEST_MARKER", "CHANNEL_ROOM_PULSE_COMMAND_MARKER", "chat-room-pulse-123", "room-pulse-inbound-901", "room-pulse-notify-901", "room-pulse-secret-901", "handoff", "Keep the room warm."} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel room pulse receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-room-pulse-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-room-pulse-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90103,
			"body": "@gitclaw /channels thread-pulse handoff --message-id room-pulse-inbound-901 --notify-message-id room-pulse-notify-901 --pulse-id room-pulse-secret-901\nNote: Keep the room warm.\nDo not leak duplicate token CHANNEL_ROOM_PULSE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate room pulse created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 6 {
		t.Fatalf("duplicate room pulse posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][5].Body
	for _, want := range []string{
		"requested_channel_command: `/channels thread-pulse`",
		"channel_room_pulse_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"comment_body_read_performed: `true`",
		"model_call_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"task_issue_created: `false`",
		"reminder_created: `false`",
		"scheduled_workflow_created: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate room pulse receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ROOM_PULSE_DUPLICATE_MARKER", "chat-room-pulse-123", "room-pulse-inbound-901", "room-pulse-notify-901", "room-pulse-secret-901", "handoff", "Keep the room warm."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate room pulse receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelRoomPulseActionRequestParsesRouteAliasAndFocus(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel room pulse"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel thread-pulse team-demo tools --message-id source-1 --notify-message-id notify-1 --pulse-id Pulse.One
Note: Check the room.`,
		},
	}
	req, err := BuildChannelRoomPulseActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRoomPulseActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "thread-pulse" || req.Options.Route != "team-demo" || req.Options.Focus != "tools" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.PulseID != "pulse-one" {
		t.Fatalf("unexpected channel room pulse parsing: %#v", req)
	}
	if req.Options.Note != "Check the room." || req.NoteSource != "trailing-note" || req.FocusSource != "positional" {
		t.Fatalf("unexpected channel room pulse note/focus parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoPulseID || req.RequestedRouteHash == "" || req.PulseIDHash == "" || req.FocusSHA == "" || req.NoteSHA == "" {
		t.Fatalf("expected explicit route room pulse hashes: %#v", req)
	}
}
