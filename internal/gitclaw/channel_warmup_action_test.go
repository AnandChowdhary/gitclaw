package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelWarmupQueuesWarmupWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-warmup-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-warmup-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-warmup-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels warmup tools --message-id warmup-inbound-901 --notify-message-id warmup-notify-901 --warmup-id warmup-secret-901\nNote: Use this tiny launcher.\nDo not include this command hidden token in the receipt: CHANNEL_WARMUP_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-warmup-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-warmup-123",
				MessageID: "warmup-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored warmup command with CHANNEL_WARMUP_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel warmup action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("warmup action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="warmup-notify-901"`,
		"GitClaw channel warmup.",
		"Theme: tools",
		"Frame: Turn tool energy into reviewed requests before execution.",
		"Conversation starters:",
		"What decision needs a reviewed tool run, and what evidence would make it safe?",
		"Which tool result would change the next step?",
		"What must stay human-reviewed before execution?",
		"Note: Use this tiny launcher.",
		"Warmup hash: ",
		"Note hash: ",
		"Warmup persistence: advisory only; no durable channel state changed.",
		"Warmup source: GitHub channel action.",
		"Model call: not performed by this action.",
		"Command execution: not performed by this action.",
		"Skill install: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Backup payload read: not performed by this action.",
		"Soul body read: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Policy mutation: not performed by this action.",
		"Schedule creation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("warmup notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_WARMUP_INGEST_MARKER", "CHANNEL_WARMUP_COMMAND_MARKER", "warmup-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("warmup notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Warmup Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels warmup`",
		"channel_warmup_status: `queued`",
		"warmup_card_mode: `structured-channel-warmup`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"warmup_id_sha256_12: `",
		"warmup_id_auto: `false`",
		"warmup_theme_sha256_12: `",
		"warmup_theme_bytes: `5`",
		"warmup_prompt_count: `3`",
		"warmup_note_sha256_12: `",
		"warmup_note_bytes: `23`",
		"warmup_note_lines: `1`",
		"warmup_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"warmup_persistence_performed: `false`",
		"workflow_mutation_performed: `false`",
		"policy_mutation_performed: `false`",
		"schedule_created: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_warmup_id_included: `false`",
		"raw_warmup_theme_included: `false`",
		"raw_warmup_note_included: `false`",
		"raw_warmup_prompts_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_warmup_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel warmup receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_WARMUP_INGEST_MARKER", "CHANNEL_WARMUP_COMMAND_MARKER", "chat-warmup-123", "warmup-inbound-901", "warmup-notify-901", "warmup-secret-901", "tools", "Use this tiny launcher.", "What decision needs"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel warmup receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-warmup-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-warmup-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels icebreaker tools --message-id warmup-inbound-901 --notify-message-id warmup-notify-901 --warmup-id warmup-secret-901\nNote: Use this tiny launcher.\nDo not leak duplicate token CHANNEL_WARMUP_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate warmup created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate warmup posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels icebreaker`",
		"channel_warmup_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"warmup_persistence_performed: `false`",
		"workflow_mutation_performed: `false`",
		"policy_mutation_performed: `false`",
		"schedule_created: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate warmup receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_WARMUP_DUPLICATE_MARKER", "chat-warmup-123", "warmup-inbound-901", "warmup-notify-901", "warmup-secret-901", "tools", "Use this tiny launcher.", "What decision needs"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate warmup receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelWarmupActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel warmup"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel starter --route team-demo --theme soul --message-id source-1 --notify-message-id notify-1 --warmup-id Warmup.One
Note: Almost there.`,
		},
	}
	req, err := BuildChannelWarmupActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelWarmupActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "starter" || req.Options.Route != "team-demo" || req.Options.Theme != "soul" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.WarmupID != "warmup-one" || req.PromptCount != 3 {
		t.Fatalf("unexpected channel warmup parsing: %#v", req)
	}
	if req.Options.Note != "Almost there." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel warmup note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoWarmupID || req.RequestedRouteHash == "" || req.WarmupIDHash == "" || req.ThemeSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route warmup hashes: %#v", req)
	}
}

func TestBuildChannelWarmupActionRequestParsesPositionalRouteAndTheme(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels thread-warmup team-demo backup --message-id source-2 --notify-message-id notify-2 --warmup-id Warmup.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelWarmupActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelWarmupActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Theme != "backups" || req.PromptCount != 3 || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel warmup parsing: %#v", req)
	}
}

func TestBuildChannelWarmupActionRequestParsesVibeCheckDefaultFun(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 44,
			Title:  "Channel vibe-check",
			Body:   RenderChannelThreadBody(ChannelIngestOptions{Channel: "telegram", ThreadID: "chat-vibe-44"}),
		},
		Comment: &Comment{
			ID: 4401,
			Body: `@gitclaw /channels vibe-check --message-id source-3 --notify-message-id notify-3 --vibe-id Vibe.One
Note: Keep it light.`,
		},
	}
	if IsChannelMoodActionRequest(ev, DefaultConfig()) {
		t.Fatal("/channels vibe-check must remain outside channel mood aliases")
	}
	if !IsChannelWarmupActionRequest(ev, DefaultConfig()) {
		t.Fatal("/channels vibe-check should route to channel warmup")
	}
	req, err := BuildChannelWarmupActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelWarmupActionRequest returned error: %v", err)
	}
	if req.Command != "/channels" || req.Subcommand != "vibe-check" || req.Options.Theme != "fun" || req.Options.WarmupID != "vibe-one" || req.Options.SourceMessageID != "source-3" || req.Options.NotifyMessageID != "notify-3" || req.PromptCount != 3 || !req.TargetFromIssue {
		t.Fatalf("unexpected vibe-check warmup parsing: %#v", req)
	}
	if req.Options.Note != "Keep it light." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected vibe-check note parsing: %#v", req)
	}
	if req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoWarmupID || req.WarmupIDHash == "" || req.ThemeSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit vibe-check hashes: %#v", req)
	}
}
