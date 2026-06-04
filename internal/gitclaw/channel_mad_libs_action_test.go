package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelMadLibsQueuesFillInCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-mad-libs-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 908,
			"title": "GitClaw telegram thread chat-mad-libs-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-mad-libs-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90801,
			"body": "@gitclaw /channels mad-libs fun --message-id mad-libs-inbound-908 --notify-message-id mad-libs-notify-908 --mad-libs-id mad-libs-secret-908\nNote: Give the thread a tiny fill-in game.\nDo not include this command hidden token in the receipt: CHANNEL_MAD_LIBS_COMMAND_MARKER.",
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
			Number: 908,
			Title:  "GitClaw telegram thread chat-mad-libs-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{908: {{
			ID: 90800,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-mad-libs-123",
				MessageID: "mad-libs-inbound-908",
				Author:    "telegram",
				Body:      "Original mirrored mad libs command with CHANNEL_MAD_LIBS_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{908: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel mad libs action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("mad libs action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[908]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="mad-libs-notify-908"`,
		"GitClaw channel mad libs.",
		"Theme: fun",
		"Fill-in card:",
		"Template:",
		"Blanks:",
		"Prompt:",
		"Mad libs hash: ",
		"Seed hash: ",
		"Note: Give the thread a tiny fill-in game.",
		"Note hash: ",
		"Mad libs source: deterministic GitHub channel action seed.",
		"Mad libs deck: bounded static GitClaw fill-in deck.",
		"Model call: not performed by this action.",
		"Dynamic text generation: not performed by this action.",
		"External randomness: not used.",
		"Game state: not persisted by this action.",
		"Score tracking: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("mad libs notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MAD_LIBS_INGEST_MARKER", "CHANNEL_MAD_LIBS_COMMAND_MARKER", "mad-libs-secret-908"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("mad libs notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Mad Libs Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels mad-libs`",
		"channel_mad_libs_status: `queued`",
		"mad_libs_mode: `deterministic-channel-fill-in-card`",
		"notification_target_issue: `#908`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"mad_libs_id_sha256_12: `",
		"mad_libs_id_auto: `false`",
		"mad_libs_theme_sha256_12: `",
		"mad_libs_theme_bytes: `3`",
		"mad_libs_theme_terms: `1`",
		"mad_libs_note_sha256_12: `",
		"mad_libs_note_bytes: `36`",
		"mad_libs_note_lines: `1`",
		"mad_libs_note_source: `trailing-note`",
		"mad_libs_template_sha256_12: `",
		"mad_libs_blank_bank_sha256_12: `",
		"mad_libs_prompt_sha256_12: `",
		"mad_libs_seed_sha256_12: `",
		"mad_libs_blank_count: `",
		"mad_libs_deck_size: `3`",
		"selected_card_index: `",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"dynamic_text_generation_performed: `false`",
		"external_randomness_used: `false`",
		"game_state_persisted: `false`",
		"score_tracking_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_mad_libs_id_included: `false`",
		"raw_mad_libs_theme_included: `false`",
		"raw_mad_libs_note_included: `false`",
		"raw_mad_libs_template_included: `false`",
		"raw_mad_libs_blank_bank_included: `false`",
		"raw_mad_libs_prompt_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_mad_libs_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel mad libs receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MAD_LIBS_INGEST_MARKER", "CHANNEL_MAD_LIBS_COMMAND_MARKER", "chat-mad-libs-123", "mad-libs-inbound-908", "mad-libs-notify-908", "mad-libs-secret-908", "fun", "Give the thread a tiny fill-in game.", "Today the channel found", "Reply with adjective"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel mad libs receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 908,
			"title": "GitClaw telegram thread chat-mad-libs-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-mad-libs-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90802,
			"body": "@gitclaw /channels fill-in fun --message-id mad-libs-inbound-908 --notify-message-id mad-libs-notify-908 --fill-in-id mad-libs-secret-908\nNote: Give the thread a tiny fill-in game.\nDo not leak duplicate token CHANNEL_MAD_LIBS_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate mad libs created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[908]); got != 4 {
		t.Fatalf("duplicate mad libs posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[908])
	}
	duplicateReceipt := github.CommentsByIssue[908][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels fill-in`",
		"channel_mad_libs_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"dynamic_text_generation_performed: `false`",
		"external_randomness_used: `false`",
		"game_state_persisted: `false`",
		"score_tracking_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate mad libs receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MAD_LIBS_DUPLICATE_MARKER", "chat-mad-libs-123", "mad-libs-inbound-908", "mad-libs-notify-908", "mad-libs-secret-908", "fun", "Give the thread a tiny fill-in game."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate mad libs receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelMadLibsActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel mad libs"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel fill-blanks --route team-demo --message-id source-1 --notify-message-id notify-1 --blank-id Mad.Libs.One
Note: Tiny blanks.`,
		},
	}
	req, err := BuildChannelMadLibsActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelMadLibsActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "fill-blanks" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MadLibsID != "mad-libs-one" || req.Options.Theme != "fun" {
		t.Fatalf("unexpected channel mad libs parsing: %#v", req)
	}
	if req.Options.Note != "Tiny blanks." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel mad libs note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMadLibsID || req.RequestedRouteHash == "" || req.MadLibsIDHash == "" || req.ThemeSHA == "" || req.NoteSHA == "" || req.TemplateSHA == "" || req.BlankSHA == "" || req.PromptSHA == "" || req.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route mad libs hashes: %#v", req)
	}
}

func TestBuildChannelMadLibsActionRequestParsesPositionalRouteAndTheme(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels mad-libs team-demo launch --message-id source-2 --notify-message-id notify-2 --mad-libs-id Mad.Libs.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelMadLibsActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelMadLibsActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Theme != "launch" || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel mad libs parsing: %#v", req)
	}
}
