package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelConstellationQueuesStarMapWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo with CHANNEL_CONSTELLATION_DOC_SECRET\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-constellation-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 912,
			"title": "GitClaw telegram thread chat-constellation-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-constellation-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91201,
			"body": "@gitclaw /channels constellation research --message-id constellation-inbound-912 --notify-message-id constellation-notify-912 --constellation-id Constellation.Secret.912\nNote: Keep the star map playful\nDo not include this command hidden token in the receipt: CHANNEL_CONSTELLATION_COMMAND_MARKER.",
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
			Number: 912,
			Title:  "GitClaw telegram thread chat-constellation-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{912: {{
			ID: 91200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-constellation-123",
				MessageID: "constellation-inbound-912",
				Author:    "telegram",
				Body:      "Original mirrored constellation command with CHANNEL_CONSTELLATION_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{912: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel constellation action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("constellation action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[912]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="constellation-notify-912"`,
		"GitClaw channel constellation.",
		"Lane: research",
		"North star: Turn OpenClaw and Hermes patterns into small audited GitClaw moves.",
		"Stars:",
		"Research catalog [research] - review the OpenClaw and Hermes landscape without source fetches",
		"Next: `@gitclaw /research catalog`",
		"Research spotlight [research] - draw one source or pattern card",
		"Next: `@gitclaw /channels research-spotlight openclaw --spotlight-id <id> --message-id <id> --notify-message-id <id>`",
		"Research map [research] - turn the selected pattern into a safe GitClaw path",
		"Next: `@gitclaw /channels research-map openclaw --map-id <id> --message-id <id> --notify-message-id <id>`",
		"Note: Keep the star map playful",
		"Constellation hash: ",
		"Note hash: ",
		"Constellation persistence: advisory only; no durable channel state changed.",
		"Constellation source: bounded GitHub channel action deck.",
		"Model call: not performed by this action.",
		"Dynamic star generation: not performed by this action.",
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
			t.Fatalf("constellation notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_CONSTELLATION_INGEST_MARKER", "CHANNEL_CONSTELLATION_COMMAND_MARKER", "CHANNEL_CONSTELLATION_DOC_SECRET", "Constellation.Secret.912"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("constellation notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Constellation Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels constellation`",
		"channel_constellation_status: `queued`",
		"constellation_card_mode: `bounded-capability-star-map`",
		"notification_target_issue: `#912`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"constellation_id_sha256_12: `",
		"constellation_id_auto: `false`",
		"constellation_lane_sha256_12: `",
		"constellation_lane_bytes: `8`",
		"constellation_star_count: `3`",
		"constellation_command_count: `3`",
		"constellation_star_sha256_12: `",
		"constellation_note_sha256_12: `",
		"constellation_note_bytes: `25`",
		"constellation_note_lines: `1`",
		"constellation_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"dynamic_star_generation_performed: `false`",
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
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_constellation_id_included: `false`",
		"raw_constellation_lane_included: `false`",
		"raw_constellation_note_included: `false`",
		"raw_constellation_stars_included: `false`",
		"raw_constellation_commands_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_constellation_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel constellation receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"research", "openclaw", "hermes", "CHANNEL_CONSTELLATION_INGEST_MARKER", "CHANNEL_CONSTELLATION_COMMAND_MARKER", "CHANNEL_CONSTELLATION_DOC_SECRET", "chat-constellation-123", "constellation-inbound-912", "constellation-notify-912", "Constellation.Secret.912", "Keep the star map playful", "Research catalog"} {
		if strings.Contains(strings.ToLower(receipt), strings.ToLower(leaked)) {
			t.Fatalf("channel constellation receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 912,
			"title": "GitClaw telegram thread chat-constellation-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-constellation-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91202,
			"body": "@gitclaw /channels star-map research --message-id constellation-inbound-912 --notify-message-id constellation-notify-912 --constellation-id Constellation.Secret.912\nNote: Keep the star map playful\nDo not leak duplicate token CHANNEL_CONSTELLATION_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate constellation created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[912]); got != 4 {
		t.Fatalf("duplicate constellation posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[912])
	}
	duplicateReceipt := github.CommentsByIssue[912][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels star-map`",
		"channel_constellation_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"dynamic_star_generation_performed: `false`",
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
			t.Fatalf("duplicate constellation receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"research", "openclaw", "hermes", "CHANNEL_CONSTELLATION_DUPLICATE_MARKER", "chat-constellation-123", "constellation-inbound-912", "constellation-notify-912", "Constellation.Secret.912", "Keep the star map playful", "Research catalog"} {
		if strings.Contains(strings.ToLower(duplicateReceipt), strings.ToLower(leaked)) {
			t.Fatalf("duplicate constellation receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelConstellationActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel constellation"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel north-star --route team-demo --lane tools --message-id source-1 --notify-message-id notify-1 --constellation-id Constellation.One
Note: Keep the tool lane visible.`,
		},
	}
	req, err := BuildChannelConstellationActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelConstellationActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "north-star" || req.Options.Route != "team-demo" || req.Options.Lane != "tools" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.ConstellationID != "constellation-one" || req.StarCount != 3 || req.CommandCount != 3 {
		t.Fatalf("unexpected channel constellation parsing: %#v", req)
	}
	if req.NoteSource != "trailing-note" || req.Options.Note != "Keep the tool lane visible." || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoConstellationID {
		t.Fatalf("unexpected channel constellation defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.ConstellationIDHash == "" || req.LaneSHA == "" || req.NoteSHA == "" || req.StarSHA == "" || req.NotificationBodySHA == "" || req.NotificationBodyBytes == 0 || req.NotificationBodyLineCnt == 0 {
		t.Fatalf("expected constellation hashes and notification metadata: %#v", req)
	}
	if !IsChannelConstellationActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel north-star alias to be recognized")
	}
}
