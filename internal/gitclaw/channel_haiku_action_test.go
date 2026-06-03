package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelHaikuQueuesPoemWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-haiku-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-haiku-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-haiku-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels haiku launch --message-id haiku-inbound-901 --notify-message-id haiku-notify-901 --haiku-id haiku-secret-901\nNote: Release handoff is steady.\nDo not include this command hidden token in the receipt: CHANNEL_HAIKU_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-haiku-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-haiku-123",
				MessageID: "haiku-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored haiku command with CHANNEL_HAIKU_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel haiku action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("haiku action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="haiku-notify-901"`,
		"GitClaw channel haiku.",
		"Theme: launch",
		"Haiku:",
		"Haiku hash: ",
		"Seed hash: ",
		"Note: Release handoff is steady.",
		"Note hash: ",
		"Poem source: deterministic GitHub channel action seed.",
		"Haiku deck: bounded static GitClaw line deck.",
		"Model call: not performed by this action.",
		"External randomness: not used.",
		"Media generation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("haiku notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_HAIKU_INGEST_MARKER", "CHANNEL_HAIKU_COMMAND_MARKER", "haiku-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("haiku notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Haiku Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels haiku`",
		"channel_haiku_status: `queued`",
		"haiku_mode: `deterministic-channel-poem`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"haiku_id_sha256_12: `",
		"haiku_id_auto: `false`",
		"haiku_theme_sha256_12: `",
		"haiku_theme_bytes: `6`",
		"haiku_note_sha256_12: `",
		"haiku_note_bytes: `26`",
		"haiku_note_lines: `1`",
		"haiku_note_source: `trailing-note`",
		"haiku_seed_sha256_12: `",
		"haiku_sha256_12: `",
		"haiku_lines: `3`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"external_randomness_used: `false`",
		"media_generation_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_haiku_id_included: `false`",
		"raw_haiku_theme_included: `false`",
		"raw_haiku_note_included: `false`",
		"raw_haiku_lines_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_haiku_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel haiku receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_HAIKU_INGEST_MARKER", "CHANNEL_HAIKU_COMMAND_MARKER", "chat-haiku-123", "haiku-inbound-901", "haiku-notify-901", "haiku-secret-901", "launch", "Release handoff is steady.", "green checks in the branch", "release notes fold into light", "ship with one clear path"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel haiku receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-haiku-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-haiku-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels poem launch --message-id haiku-inbound-901 --notify-message-id haiku-notify-901 --haiku-id haiku-secret-901\nNote: Release handoff is steady.\nDo not leak duplicate token CHANNEL_HAIKU_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate haiku created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate haiku posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels poem`",
		"channel_haiku_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"external_randomness_used: `false`",
		"media_generation_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate haiku receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_HAIKU_DUPLICATE_MARKER", "chat-haiku-123", "haiku-inbound-901", "haiku-notify-901", "haiku-secret-901", "launch", "Release handoff is steady."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate haiku receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelHaikuActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel haiku"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel verse --route team-demo --message-id source-1 --notify-message-id notify-1 --haiku-id Haiku.One
Note: Tiny signal.`,
		},
	}
	req, err := BuildChannelHaikuActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelHaikuActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "verse" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.HaikuID != "haiku-one" || req.Options.Theme != "fun" {
		t.Fatalf("unexpected channel haiku parsing: %#v", req)
	}
	if req.Options.Note != "Tiny signal." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel haiku note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoHaikuID || req.RequestedRouteHash == "" || req.HaikuIDHash == "" || req.ThemeSHA == "" || req.NoteSHA == "" || req.HaikuSHA == "" || req.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route haiku hashes: %#v", req)
	}
}

func TestBuildChannelHaikuActionRequestParsesPositionalRouteAndTheme(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels haiku team-demo launch --message-id source-2 --notify-message-id notify-2 --haiku-id Haiku.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelHaikuActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelHaikuActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Theme != "launch" || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel haiku parsing: %#v", req)
	}
}
