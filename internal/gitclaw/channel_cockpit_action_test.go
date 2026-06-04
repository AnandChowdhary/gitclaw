package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelCockpitQueuesStatusBoardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo with CHANNEL_COCKPIT_DOC_SECRET\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-cockpit-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 914,
			"title": "GitClaw telegram thread chat-cockpit-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-cockpit-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91401,
			"body": "@gitclaw /channels cockpit research --message-id cockpit-inbound-914 --notify-message-id cockpit-notify-914 --cockpit-id Cockpit.Secret.914\nNote: Keep the board skimmable\nDo not include this command hidden token in the receipt: CHANNEL_COCKPIT_COMMAND_MARKER.",
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
			Number: 914,
			Title:  "GitClaw telegram thread chat-cockpit-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{914: {{
			ID: 91400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-cockpit-123",
				MessageID: "cockpit-inbound-914",
				Author:    "telegram",
				Body:      "Original mirrored cockpit command with CHANNEL_COCKPIT_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{914: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel cockpit action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("cockpit action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[914]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="cockpit-notify-914"`,
		"GitClaw channel cockpit.",
		"Lane: research",
		"Board: OpenClaw/Hermes patterns are ready for review-first GitClaw translation.",
		"Gauges:",
		"Evidence [ready] - reviewed OpenClaw/Hermes source catalog is the first stop",
		"Next: `@gitclaw /research catalog`",
		"Pattern [ready] - draw one source or pattern without live browsing",
		"Next: `@gitclaw /channels research-spotlight openclaw --spotlight-id <id> --message-id <id> --notify-message-id <id>`",
		"Path [ready] - translate the selected pattern into safe GitClaw commands",
		"Next: `@gitclaw /channels research-map openclaw --map-id <id> --message-id <id> --notify-message-id <id>`",
		"Loop [watch] - keep the next move review-first and bounded",
		"Next: `@gitclaw /channels mission-control research --mission-id <id> --message-id <id> --notify-message-id <id>`",
		"Note: Keep the board skimmable",
		"Cockpit hash: ",
		"Note hash: ",
		"Cockpit persistence: advisory only; no durable channel state changed.",
		"Cockpit source: bounded GitHub channel action deck.",
		"Model call: not performed by this action.",
		"Dynamic cockpit generation: not performed by this action.",
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
			t.Fatalf("cockpit notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_COCKPIT_INGEST_MARKER", "CHANNEL_COCKPIT_COMMAND_MARKER", "CHANNEL_COCKPIT_DOC_SECRET", "Cockpit.Secret.914"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("cockpit notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Cockpit Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels cockpit`",
		"channel_cockpit_status: `queued`",
		"cockpit_card_mode: `bounded-provider-status-board`",
		"notification_target_issue: `#914`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"cockpit_id_sha256_12: `",
		"cockpit_id_auto: `false`",
		"cockpit_lane_sha256_12: `",
		"cockpit_lane_bytes: `8`",
		"cockpit_gauge_count: `4`",
		"cockpit_command_count: `4`",
		"cockpit_gauge_sha256_12: `",
		"cockpit_note_sha256_12: `",
		"cockpit_note_bytes: `24`",
		"cockpit_note_lines: `1`",
		"cockpit_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"dynamic_cockpit_generation_performed: `false`",
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
		"raw_cockpit_id_included: `false`",
		"raw_cockpit_lane_included: `false`",
		"raw_cockpit_note_included: `false`",
		"raw_cockpit_gauges_included: `false`",
		"raw_cockpit_commands_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_cockpit_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel cockpit receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"research", "openclaw", "hermes", "CHANNEL_COCKPIT_INGEST_MARKER", "CHANNEL_COCKPIT_COMMAND_MARKER", "CHANNEL_COCKPIT_DOC_SECRET", "chat-cockpit-123", "cockpit-inbound-914", "cockpit-notify-914", "Cockpit.Secret.914", "Keep the board skimmable", "Evidence [ready]"} {
		if strings.Contains(strings.ToLower(receipt), strings.ToLower(leaked)) {
			t.Fatalf("channel cockpit receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 914,
			"title": "GitClaw telegram thread chat-cockpit-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-cockpit-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91402,
			"body": "@gitclaw /channels dashboard research --message-id cockpit-inbound-914 --notify-message-id cockpit-notify-914 --cockpit-id Cockpit.Secret.914\nNote: Keep the board skimmable\nDo not leak duplicate token CHANNEL_COCKPIT_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate cockpit created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[914]); got != 4 {
		t.Fatalf("duplicate cockpit posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[914])
	}
	duplicateReceipt := github.CommentsByIssue[914][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels dashboard`",
		"channel_cockpit_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"dynamic_cockpit_generation_performed: `false`",
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
			t.Fatalf("duplicate cockpit receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"research", "openclaw", "hermes", "CHANNEL_COCKPIT_DUPLICATE_MARKER", "chat-cockpit-123", "cockpit-inbound-914", "cockpit-notify-914", "Cockpit.Secret.914", "Keep the board skimmable", "Evidence [ready]"} {
		if strings.Contains(strings.ToLower(duplicateReceipt), strings.ToLower(leaked)) {
			t.Fatalf("duplicate cockpit receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelCockpitActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel cockpit"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel control-room --route team-demo --lane tools --message-id source-1 --notify-message-id notify-1 --cockpit-id Cockpit.One
Note: Keep the tool lane visible.`,
		},
	}
	req, err := BuildChannelCockpitActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelCockpitActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "control-room" || req.Options.Route != "team-demo" || req.Options.Lane != "tools" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.CockpitID != "cockpit-one" || req.GaugeCount != 4 || req.CommandCount != 4 {
		t.Fatalf("unexpected channel cockpit parsing: %#v", req)
	}
	if req.NoteSource != "trailing-note" || req.Options.Note != "Keep the tool lane visible." || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoCockpitID {
		t.Fatalf("unexpected channel cockpit defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.CockpitIDHash == "" || req.LaneSHA == "" || req.NoteSHA == "" || req.GaugeSHA == "" || req.NotificationBodySHA == "" || req.NotificationBytes == 0 || req.NotificationLines == 0 {
		t.Fatalf("expected cockpit hashes and notification metadata: %#v", req)
	}
	if !IsChannelCockpitActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel control-room alias to be recognized")
	}
}
