package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSoundtrackQueuesMixWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-soundtrack-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-soundtrack-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soundtrack-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels soundtrack launch --message-id soundtrack-inbound-901 --notify-message-id soundtrack-notify-901 --soundtrack-id soundtrack-secret-901\nNote: Release handoff is steady.\nDo not include this command hidden token in the receipt: CHANNEL_SOUNDTRACK_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-soundtrack-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-soundtrack-123",
				MessageID: "soundtrack-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored soundtrack command with CHANNEL_SOUNDTRACK_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soundtrack action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("soundtrack action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="soundtrack-notify-901"`,
		"GitClaw channel soundtrack.",
		"Theme: launch",
		"Tracks:",
		"Soundtrack hash: ",
		"Seed hash: ",
		"Note: Release handoff is steady.",
		"Note hash: ",
		"Mix source: deterministic GitHub channel action seed.",
		"Soundtrack deck: bounded static GitClaw track deck.",
		"Model call: not performed by this action.",
		"External randomness: not used.",
		"Media generation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("soundtrack notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUNDTRACK_INGEST_MARKER", "CHANNEL_SOUNDTRACK_COMMAND_MARKER", "soundtrack-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("soundtrack notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soundtrack Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels soundtrack`",
		"channel_soundtrack_status: `queued`",
		"soundtrack_mode: `deterministic-channel-mix`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"soundtrack_id_sha256_12: `",
		"soundtrack_id_auto: `false`",
		"soundtrack_theme_sha256_12: `",
		"soundtrack_theme_bytes: `6`",
		"soundtrack_note_sha256_12: `",
		"soundtrack_note_bytes: `26`",
		"soundtrack_note_lines: `1`",
		"soundtrack_note_source: `trailing-note`",
		"soundtrack_seed_sha256_12: `",
		"soundtrack_sha256_12: `",
		"soundtrack_track_count: `3`",
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
		"raw_soundtrack_id_included: `false`",
		"raw_soundtrack_theme_included: `false`",
		"raw_soundtrack_note_included: `false`",
		"raw_soundtrack_tracks_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_soundtrack_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soundtrack receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUNDTRACK_INGEST_MARKER", "CHANNEL_SOUNDTRACK_COMMAND_MARKER", "chat-soundtrack-123", "soundtrack-inbound-901", "soundtrack-notify-901", "soundtrack-secret-901", "launch", "Release handoff is steady.", "Green Check Walkout", "Changelog Lights", "Tag Door Opens"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soundtrack receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-soundtrack-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soundtrack-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels mix launch --message-id soundtrack-inbound-901 --notify-message-id soundtrack-notify-901 --soundtrack-id soundtrack-secret-901\nNote: Release handoff is steady.\nDo not leak duplicate token CHANNEL_SOUNDTRACK_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate soundtrack created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate soundtrack posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels mix`",
		"channel_soundtrack_status: `duplicate`",
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
			t.Fatalf("duplicate soundtrack receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUNDTRACK_DUPLICATE_MARKER", "chat-soundtrack-123", "soundtrack-inbound-901", "soundtrack-notify-901", "soundtrack-secret-901", "launch", "Release handoff is steady."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate soundtrack receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSoundtrackActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel soundtrack"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel playlist --route team-demo --message-id source-1 --notify-message-id notify-1 --playlist-id Soundtrack.One
Note: Tiny signal.`,
		},
	}
	req, err := BuildChannelSoundtrackActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelSoundtrackActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "playlist" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SoundtrackID != "soundtrack-one" || req.Options.Theme != "fun" {
		t.Fatalf("unexpected channel soundtrack parsing: %#v", req)
	}
	if req.Options.Note != "Tiny signal." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel soundtrack note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSoundtrackID || req.RequestedRouteHash == "" || req.SoundtrackIDHash == "" || req.ThemeSHA == "" || req.NoteSHA == "" || req.SoundtrackSHA == "" || req.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route soundtrack hashes: %#v", req)
	}
}

func TestBuildChannelSoundtrackActionRequestParsesPositionalRouteAndTheme(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels soundtrack team-demo launch --message-id source-2 --notify-message-id notify-2 --soundtrack-id Soundtrack.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelSoundtrackActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelSoundtrackActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Theme != "launch" || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel soundtrack parsing: %#v", req)
	}
}
