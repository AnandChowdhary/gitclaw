package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelCompassQueuesCompassWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-compass-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-compass-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-compass-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels compass fun --message-id compass-inbound-901 --notify-message-id compass-notify-901 --compass-id compass-secret-901\nNote: Use this tiny launcher.\nDo not include this command hidden token in the receipt: CHANNEL_COMPASS_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-compass-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-compass-123",
				MessageID: "compass-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored compass command with CHANNEL_COMPASS_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel compass action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("compass action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="compass-notify-901"`,
		"GitClaw channel compass.",
		"Focus: fun",
		"Next safe steps:",
		"/channels roll --dice 2d6 --message-id <id> --notify-message-id <id>",
		"/channels choose --message-id <id> --notify-message-id <id>",
		"/channels mood <mood> --message-id <id> --notify-message-id <id>",
		"/channels sticker <sticker> --sticker-id <id> --message-id <id> --notify-message-id <id>",
		"/channels nudge <target> --nudge-id <id> --message-id <id> --notify-message-id <id>",
		"Note: Use this tiny launcher.",
		"Compass hash: ",
		"Note hash: ",
		"Compass source: GitHub channel action.",
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
			t.Fatalf("compass notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_COMPASS_INGEST_MARKER", "CHANNEL_COMPASS_COMMAND_MARKER", "compass-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("compass notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Compass Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels compass`",
		"channel_compass_status: `queued`",
		"compass_mode: `structured-channel-compass`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"compass_id_sha256_12: `",
		"compass_id_auto: `false`",
		"compass_focus_sha256_12: `",
		"compass_focus_bytes: `3`",
		"compass_step_count: `5`",
		"compass_note_sha256_12: `",
		"compass_note_bytes: `23`",
		"compass_note_lines: `1`",
		"compass_note_source: `trailing-note`",
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
		"raw_compass_id_included: `false`",
		"raw_compass_focus_included: `false`",
		"raw_compass_note_included: `false`",
		"raw_compass_steps_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_compass_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel compass receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_COMPASS_INGEST_MARKER", "CHANNEL_COMPASS_COMMAND_MARKER", "chat-compass-123", "compass-inbound-901", "compass-notify-901", "compass-secret-901", "Use this tiny launcher.", "/channels roll --dice 2d6"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel compass receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-compass-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-compass-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels orient fun --message-id compass-inbound-901 --notify-message-id compass-notify-901 --compass-id compass-secret-901\nNote: Use this tiny launcher.\nDo not leak duplicate token CHANNEL_COMPASS_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate compass created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate compass posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels orient`",
		"channel_compass_status: `duplicate`",
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
			t.Fatalf("duplicate compass receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_COMPASS_DUPLICATE_MARKER", "chat-compass-123", "compass-inbound-901", "compass-notify-901", "compass-secret-901", "Use this tiny launcher.", "/channels roll --dice 2d6"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate compass receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelCompassActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel compass"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel orient --route team-demo --focus skill --message-id source-1 --notify-message-id notify-1 --compass-id Compass.One
Note: Almost there.`,
		},
	}
	req, err := BuildChannelCompassActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelCompassActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "orient" || req.Options.Route != "team-demo" || req.Options.Focus != "skills" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.CompassID != "compass-one" || req.StepCount != 4 {
		t.Fatalf("unexpected channel compass parsing: %#v", req)
	}
	if req.Options.Note != "Almost there." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel compass note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoCompassID || req.RequestedRouteHash == "" || req.CompassIDHash == "" || req.FocusSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route compass hashes: %#v", req)
	}
}

func TestBuildChannelCompassActionRequestParsesPositionalRouteAndFocus(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels navigator team-demo memory --message-id source-2 --notify-message-id notify-2 --compass-id Compass.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelCompassActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelCompassActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Focus != "memory" || req.StepCount != 4 || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel compass parsing: %#v", req)
	}
}
