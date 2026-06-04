package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelMissionControlQueuesOperatingLoopWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo with CHANNEL_MISSION_CONTROL_DOC_SECRET\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-mission-control-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 913,
			"title": "GitClaw telegram thread chat-mission-control-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-mission-control-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91301,
			"body": "@gitclaw /channels mission-control research --message-id mission-control-inbound-913 --notify-message-id mission-control-notify-913 --mission-id Mission.Control.Secret.913\nNote: Keep the loop review-first\nDo not include this command hidden token in the receipt: CHANNEL_MISSION_CONTROL_COMMAND_MARKER.",
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
			Number: 913,
			Title:  "GitClaw telegram thread chat-mission-control-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{913: {{
			ID: 91300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-mission-control-123",
				MessageID: "mission-control-inbound-913",
				Author:    "telegram",
				Body:      "Original mirrored mission-control command with CHANNEL_MISSION_CONTROL_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{913: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel mission-control action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("mission-control action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[913]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="mission-control-notify-913"`,
		"GitClaw channel mission control.",
		"Lane: research",
		"Control loop: Turn OpenClaw and Hermes research into one review-first GitClaw move.",
		"Loop steps:",
		"Signal [research] - review the static OpenClaw/Hermes source and pattern catalog",
		"Next: `@gitclaw /research catalog`",
		"Gate [research] - draw one provider-facing source or pattern card",
		"Next: `@gitclaw /channels research-spotlight openclaw --spotlight-id <id> --message-id <id> --notify-message-id <id>`",
		"Rehearse [research] - convert the selected pattern into a safe command path",
		"Next: `@gitclaw /channels research-map openclaw --map-id <id> --message-id <id> --notify-message-id <id>`",
		"Commit [research] - return with a safe next action without executing tools",
		"Next: `@gitclaw /channels compass research --compass-id <id> --message-id <id> --notify-message-id <id>`",
		"Note: Keep the loop review-first",
		"Mission hash: ",
		"Note hash: ",
		"Mission persistence: advisory only; no durable channel state changed.",
		"Mission source: bounded GitHub channel action deck.",
		"Model call: not performed by this action.",
		"Dynamic loop generation: not performed by this action.",
		"External randomness: not used by this action.",
		"Command execution: not performed by this action.",
		"Skill install: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Backup payload read: not performed by this action.",
		"Soul body read: not performed by this action.",
		"Memory write: not performed by this action.",
		"Source fetch: not performed by this action.",
		"Live source browse: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Policy mutation: not performed by this action.",
		"Schedule creation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("mission-control notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MISSION_CONTROL_INGEST_MARKER", "CHANNEL_MISSION_CONTROL_COMMAND_MARKER", "CHANNEL_MISSION_CONTROL_DOC_SECRET", "Mission.Control.Secret.913"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("mission-control notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Mission Control Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels mission-control`",
		"channel_mission_control_status: `queued`",
		"mission_control_card_mode: `bounded-operating-loop`",
		"notification_target_issue: `#913`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"mission_id_sha256_12: `",
		"mission_id_auto: `false`",
		"mission_lane_sha256_12: `",
		"mission_lane_bytes: `8`",
		"mission_step_count: `4`",
		"mission_command_count: `4`",
		"mission_step_sha256_12: `",
		"mission_note_sha256_12: `",
		"mission_note_bytes: `26`",
		"mission_note_lines: `1`",
		"mission_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"dynamic_loop_generation_performed: `false`",
		"external_randomness_used: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"memory_write_performed: `false`",
		"source_fetch_performed: `false`",
		"live_source_browse_performed: `false`",
		"artifact_issue_created: `false`",
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
		"raw_mission_id_included: `false`",
		"raw_mission_lane_included: `false`",
		"raw_mission_note_included: `false`",
		"raw_mission_steps_included: `false`",
		"raw_mission_commands_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_mission_control_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel mission-control receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"research", "openclaw", "hermes", "CHANNEL_MISSION_CONTROL_INGEST_MARKER", "CHANNEL_MISSION_CONTROL_COMMAND_MARKER", "CHANNEL_MISSION_CONTROL_DOC_SECRET", "chat-mission-control-123", "mission-control-inbound-913", "mission-control-notify-913", "Mission.Control.Secret.913", "Keep the loop review-first", "review the static"} {
		if strings.Contains(strings.ToLower(receipt), strings.ToLower(leaked)) {
			t.Fatalf("channel mission-control receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 913,
			"title": "GitClaw telegram thread chat-mission-control-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-mission-control-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91302,
			"body": "@gitclaw /channels control-loop research --message-id mission-control-inbound-913 --notify-message-id mission-control-notify-913 --mission-id Mission.Control.Secret.913\nNote: Keep the loop review-first\nDo not leak duplicate token CHANNEL_MISSION_CONTROL_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate mission-control created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[913]); got != 4 {
		t.Fatalf("duplicate mission-control posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[913])
	}
	duplicateReceipt := github.CommentsByIssue[913][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels control-loop`",
		"channel_mission_control_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"dynamic_loop_generation_performed: `false`",
		"external_randomness_used: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"memory_write_performed: `false`",
		"source_fetch_performed: `false`",
		"live_source_browse_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"policy_mutation_performed: `false`",
		"schedule_created: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate mission-control receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"research", "openclaw", "hermes", "CHANNEL_MISSION_CONTROL_DUPLICATE_MARKER", "chat-mission-control-123", "mission-control-inbound-913", "mission-control-notify-913", "Mission.Control.Secret.913", "Keep the loop review-first", "review the static"} {
		if strings.Contains(strings.ToLower(duplicateReceipt), strings.ToLower(leaked)) {
			t.Fatalf("duplicate mission-control receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelMissionControlActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel mission control"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel flight-plan --route team-demo --lane tools --message-id source-1 --notify-message-id notify-1 --mission-id Mission.One
Note: Keep the tool lane visible.`,
		},
	}
	req, err := BuildChannelMissionControlActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelMissionControlActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "flight-plan" || req.Options.Route != "team-demo" || req.Options.Lane != "tools" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MissionID != "mission-one" || req.LoopStepCount != 4 || req.CommandCount != 4 {
		t.Fatalf("unexpected channel mission-control parsing: %#v", req)
	}
	if req.NoteSource != "trailing-note" || req.Options.Note != "Keep the tool lane visible." || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMissionID {
		t.Fatalf("unexpected channel mission-control defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.MissionIDHash == "" || req.LaneSHA == "" || req.NoteSHA == "" || req.LoopSHA == "" || req.NotificationBodySHA == "" || req.NotificationBytes == 0 || req.NotificationLines == 0 {
		t.Fatalf("expected mission-control hashes and notification metadata: %#v", req)
	}
	if !IsChannelMissionControlActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel flight-plan alias to be recognized")
	}
}
