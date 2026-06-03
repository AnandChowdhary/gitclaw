package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelCoachQueuesRepoAwareAdviceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Stay body-safe in channel receipts.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Use reviewed tool requests for mutating work.\n")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
description: Read repository files when asked.
---
Use repo search and file reads for codebase questions.
`)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-coach-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-coach-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-coach-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels coach skills --message-id coach-inbound-901 --notify-message-id coach-notify-901 --coach-id coach-secret-901\nNote: Use live posture.\nDo not include this command hidden token in the receipt: CHANNEL_COACH_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-coach-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-coach-123",
				MessageID: "coach-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored coach command with CHANNEL_COACH_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel coach action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("coach action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="coach-notify-901"`,
		"GitClaw channel coach.",
		"Lane: skills",
		"Signals:",
		"- Skills: available=",
		"- Tools: enabled=",
		"- Soul: status=",
		"Recommended next moves:",
		"`/channels skills --message-id <id> --notify-message-id <id>`",
		"`/channels skill-search <query> --message-id <id> --notify-message-id <id>`",
		"`/channels skill-info <skill> --message-id <id> --notify-message-id <id>`",
		"`/channels propose-skill <name> --message-id <id>`",
		"Note: Use live posture.",
		"Coach hash: ",
		"Recommendation hash: ",
		"Coach source: current GitHub Actions checkout metadata.",
		"Model call: not performed by this action.",
		"Command execution: not performed by this action.",
		"Skill install: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Backup payload read: not performed by this action.",
		"Soul body read: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("coach notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_COACH_INGEST_MARKER", "CHANNEL_COACH_COMMAND_MARKER", "coach-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("coach notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Coach Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels coach`",
		"channel_coach_status: `queued`",
		"coach_mode: `repo-aware-channel-coach`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"coach_id_sha256_12: `",
		"coach_id_auto: `false`",
		"coach_lane_sha256_12: `",
		"coach_lane_bytes: `6`",
		"coach_note_sha256_12: `",
		"coach_note_bytes: `17`",
		"coach_note_lines: `1`",
		"coach_note_source: `trailing-note`",
		"coach_recommendation_count: `4`",
		"coach_advice_sha256_12: `",
		"coach_snapshot_sha256_12: `",
		"available_skills: `",
		"enabled_skills: `",
		"selected_skills: `",
		"skills_missing_requirements: `",
		"enabled_tools: `",
		"active_tool_outputs: `",
		"tool_validation_status: `",
		"tool_risk_status: `",
		"soul_status: `",
		"soul_validation_status: `",
		"soul_risk_status: `",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"repository_mutation_recommended: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_coach_id_included: `false`",
		"raw_coach_lane_included: `false`",
		"raw_coach_note_included: `false`",
		"raw_coach_recommendations_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_coach_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel coach receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_COACH_INGEST_MARKER", "CHANNEL_COACH_COMMAND_MARKER", "chat-coach-123", "coach-inbound-901", "coach-notify-901", "coach-secret-901", "Use live posture.", "/channels skill-search <query>"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel coach receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-coach-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-coach-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels advise skills --message-id coach-inbound-901 --notify-message-id coach-notify-901 --coach-id coach-secret-901\nNote: Use live posture.\nDo not leak duplicate token CHANNEL_COACH_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate coach created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate coach posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels advise`",
		"channel_coach_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate coach receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_COACH_DUPLICATE_MARKER", "chat-coach-123", "coach-inbound-901", "coach-notify-901", "coach-secret-901", "Use live posture.", "/channels skill-search <query>"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate coach receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelCoachActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel coach"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel mentor --route team-demo --lane tool --message-id source-1 --notify-message-id notify-1 --coach-id Coach.One
Note: Almost there.`,
		},
	}
	req, err := BuildChannelCoachActionRequest(ev, DefaultConfig(), RepoContext{})
	if err != nil {
		t.Fatalf("BuildChannelCoachActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "mentor" || req.Options.Route != "team-demo" || req.Options.Lane != "tools" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.CoachID != "coach-one" || req.RecommendationCount != 4 {
		t.Fatalf("unexpected channel coach parsing: %#v", req)
	}
	if req.Options.Note != "Almost there." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel coach note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoCoachID || req.RequestedRouteHash == "" || req.CoachIDHash == "" || req.LaneSHA == "" || req.NoteSHA == "" || req.AdviceSHA == "" || req.SnapshotSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route coach hashes: %#v", req)
	}
}

func TestBuildChannelCoachActionRequestParsesPositionalRouteAndLane(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels next-move team-demo soul --message-id source-2 --notify-message-id notify-2 --coach-id Coach.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelCoachActionRequest(ev, DefaultConfig(), RepoContext{})
	if err != nil {
		t.Fatalf("BuildChannelCoachActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Lane != "soul" || req.RecommendationCount != 4 || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel coach parsing: %#v", req)
	}
}
