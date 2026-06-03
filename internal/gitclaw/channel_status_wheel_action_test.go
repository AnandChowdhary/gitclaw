package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelStatusWheelQueuesDeterministicSpinWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-status-wheel-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-status-wheel-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-status-wheel-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels status-wheel release --message-id status-wheel-inbound-901 --notify-message-id status-wheel-notify-901 --wheel-id status-wheel-secret-901\nNote: Keep the spin small.\nDo not include this command hidden token in the receipt: CHANNEL_STATUS_WHEEL_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-status-wheel-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-status-wheel-123",
				MessageID: "status-wheel-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored status wheel command with CHANNEL_STATUS_WHEEL_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}
	pick := buildChannelStatusWheelPick(ChannelStatusWheelOptions{
		Repo:            "owner/repo",
		Channel:         "telegram",
		ThreadID:        "chat-status-wheel-123",
		SourceMessageID: "status-wheel-inbound-901",
		NotifyMessageID: "status-wheel-notify-901",
		WheelID:         "status-wheel-secret-901",
		Lane:            "release",
		Note:            "Keep the spin small.",
	})

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel status wheel action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("status wheel action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="status-wheel-notify-901"`,
		"GitClaw channel status wheel.",
		"Lane: release",
		"Picked: #",
		"Status: " + pick.Entry.Status,
		"Micro-action: " + pick.Entry.Action,
		"Status hash: ",
		"Action hash: ",
		"Deck hash: ",
		"Seed hash: ",
		"Note: Keep the spin small.",
		"Note hash: ",
		"Selection source: deterministic GitHub channel action seed.",
		"Status persistence: advisory only; no durable channel state changed.",
		"Model call: not performed by this action.",
		"External randomness: not used.",
		"Command execution: not performed by this action.",
		"Artifact issue creation: not performed by this action.",
		"Task/reminder creation: not performed by this action.",
		"Skill install: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("status wheel notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_STATUS_WHEEL_INGEST_MARKER", "CHANNEL_STATUS_WHEEL_COMMAND_MARKER", "status-wheel-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("status wheel notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Status Wheel Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels status-wheel`",
		"channel_status_wheel_status: `queued`",
		"status_wheel_mode: `deterministic-channel-status-wheel`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"status_wheel_id_sha256_12: `",
		"status_wheel_id_auto: `false`",
		"status_wheel_lane_sha256_12: `",
		"status_wheel_lane_bytes: `7`",
		"status_wheel_lane_terms: `1`",
		"status_wheel_lane_source: `positional`",
		"status_wheel_status_count: `5`",
		"status_wheel_status_index: `",
		"status_wheel_deck_sha256_12: `",
		"status_wheel_status_sha256_12: `",
		"status_wheel_action_sha256_12: `",
		"status_wheel_seed_sha256_12: `",
		"status_wheel_note_sha256_12: `",
		"status_wheel_note_bytes: `20`",
		"status_wheel_note_lines: `1`",
		"status_wheel_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"external_randomness_used: `false`",
		"command_execution_performed: `false`",
		"artifact_issue_created: `false`",
		"task_issue_created: `false`",
		"reminder_created: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"status_persistence_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_status_wheel_id_included: `false`",
		"raw_status_wheel_lane_included: `false`",
		"raw_status_wheel_note_included: `false`",
		"raw_status_wheel_deck_included: `false`",
		"raw_status_wheel_status_included: `false`",
		"raw_status_wheel_action_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_status_wheel_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel status wheel receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_STATUS_WHEEL_INGEST_MARKER", "CHANNEL_STATUS_WHEEL_COMMAND_MARKER", "chat-status-wheel-123", "status-wheel-inbound-901", "status-wheel-notify-901", "status-wheel-secret-901", "release", "Keep the spin small.", pick.Entry.Status, pick.Entry.Action} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel status wheel receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-status-wheel-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-status-wheel-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels spin release --message-id status-wheel-inbound-901 --notify-message-id status-wheel-notify-901 --wheel-id status-wheel-secret-901\nNote: Keep the spin small.\nDo not leak duplicate token CHANNEL_STATUS_WHEEL_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate status wheel created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate status wheel posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels spin`",
		"channel_status_wheel_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"external_randomness_used: `false`",
		"command_execution_performed: `false`",
		"artifact_issue_created: `false`",
		"task_issue_created: `false`",
		"reminder_created: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"status_persistence_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate status wheel receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_STATUS_WHEEL_DUPLICATE_MARKER", "chat-status-wheel-123", "status-wheel-inbound-901", "status-wheel-notify-901", "status-wheel-secret-901", "release", "Keep the spin small.", pick.Entry.Status, pick.Entry.Action} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate status wheel receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelStatusWheelActionRequestParsesRouteAliasAndLane(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel status wheel"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel signal-wheel team-demo tool --message-id source-1 --notify-message-id notify-1 --wheel-id Wheel.One
Note: Spin carefully.`,
		},
	}
	req, err := BuildChannelStatusWheelActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelStatusWheelActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "signal-wheel" || req.Options.Route != "team-demo" || req.Options.Lane != "tools" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.WheelID != "wheel-one" || req.StatusCount != 5 || req.StatusIndex < 1 {
		t.Fatalf("unexpected channel status wheel parsing: %#v", req)
	}
	if req.Options.Note != "Spin carefully." || req.NoteSource != "trailing-note" || req.LaneSource != "positional" {
		t.Fatalf("unexpected channel status wheel note/lane parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoWheelID || req.RequestedRouteHash == "" || req.WheelIDHash == "" || req.LaneSHA == "" || req.DeckSHA == "" || req.StatusSHA == "" || req.ActionSHA == "" || req.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route status wheel hashes: %#v", req)
	}
}
