package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelFortuneCookieQueuesDeterministicCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-fortune-cookie-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 902,
			"title": "GitClaw telegram thread chat-fortune-cookie-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-fortune-cookie-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90201,
			"body": "@gitclaw /channels fortune-cookie release --message-id fortune-cookie-inbound-902 --notify-message-id fortune-cookie-notify-902 --cookie-id fortune-cookie-secret-902\nNote: Make the launch card tiny.\nDo not include this command hidden token in the receipt: CHANNEL_FORTUNE_COOKIE_COMMAND_MARKER.",
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
			Number: 902,
			Title:  "GitClaw telegram thread chat-fortune-cookie-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{902: {{
			ID: 90200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-fortune-cookie-123",
				MessageID: "fortune-cookie-inbound-902",
				Author:    "telegram",
				Body:      "Original mirrored fortune cookie command with CHANNEL_FORTUNE_COOKIE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{902: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}
	pick := buildChannelFortuneCookiePick(ChannelFortuneCookieOptions{
		Repo:            "owner/repo",
		Channel:         "telegram",
		ThreadID:        "chat-fortune-cookie-123",
		SourceMessageID: "fortune-cookie-inbound-902",
		NotifyMessageID: "fortune-cookie-notify-902",
		CookieID:        "fortune-cookie-secret-902",
		Theme:           "release",
		Note:            "Make the launch card tiny.",
	})

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel fortune cookie action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("fortune cookie action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[902]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="fortune-cookie-notify-902"`,
		"GitClaw channel fortune cookie.",
		"Theme: release",
		"Opened: #",
		"Fortune: " + pick.Entry.Fortune,
		"Next prompt: " + pick.Entry.Prompt,
		"Lucky number: ",
		"Fortune hash: ",
		"Prompt hash: ",
		"Deck hash: ",
		"Seed hash: ",
		"Note: Make the launch card tiny.",
		"Note hash: ",
		"Selection source: deterministic GitHub channel action seed.",
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
			t.Fatalf("fortune cookie notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_FORTUNE_COOKIE_INGEST_MARKER", "CHANNEL_FORTUNE_COOKIE_COMMAND_MARKER", "fortune-cookie-secret-902"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("fortune cookie notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Fortune Cookie Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels fortune-cookie`",
		"channel_fortune_cookie_status: `queued`",
		"fortune_cookie_mode: `deterministic-channel-fortune-cookie`",
		"notification_target_issue: `#902`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"fortune_cookie_id_sha256_12: `",
		"fortune_cookie_id_auto: `false`",
		"fortune_cookie_theme_sha256_12: `",
		"fortune_cookie_theme_bytes: `7`",
		"fortune_cookie_theme_terms: `1`",
		"fortune_cookie_theme_source: `positional`",
		"fortune_cookie_count: `5`",
		"fortune_cookie_index: `",
		"fortune_cookie_deck_sha256_12: `",
		"fortune_cookie_text_sha256_12: `",
		"fortune_cookie_prompt_sha256_12: `",
		"fortune_cookie_lucky_number: `",
		"fortune_cookie_lucky_number_sha256_12: `",
		"fortune_cookie_seed_sha256_12: `",
		"fortune_cookie_note_sha256_12: `",
		"fortune_cookie_note_bytes: `26`",
		"fortune_cookie_note_lines: `1`",
		"fortune_cookie_note_source: `trailing-note`",
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
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_fortune_cookie_id_included: `false`",
		"raw_fortune_cookie_theme_included: `false`",
		"raw_fortune_cookie_note_included: `false`",
		"raw_fortune_cookie_deck_included: `false`",
		"raw_fortune_cookie_text_included: `false`",
		"raw_fortune_cookie_prompt_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_fortune_cookie_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel fortune cookie receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_FORTUNE_COOKIE_INGEST_MARKER", "CHANNEL_FORTUNE_COOKIE_COMMAND_MARKER", "chat-fortune-cookie-123", "fortune-cookie-inbound-902", "fortune-cookie-notify-902", "fortune-cookie-secret-902", "release", "Make the launch card tiny.", pick.Entry.Fortune, pick.Entry.Prompt} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel fortune cookie receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 902,
			"title": "GitClaw telegram thread chat-fortune-cookie-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-fortune-cookie-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90202,
			"body": "@gitclaw /channels cookie release --message-id fortune-cookie-inbound-902 --notify-message-id fortune-cookie-notify-902 --cookie-id fortune-cookie-secret-902\nNote: Make the launch card tiny.\nDo not leak duplicate token CHANNEL_FORTUNE_COOKIE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate fortune cookie created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[902]); got != 4 {
		t.Fatalf("duplicate fortune cookie posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[902])
	}
	duplicateReceipt := github.CommentsByIssue[902][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels cookie`",
		"channel_fortune_cookie_status: `duplicate`",
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
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate fortune cookie receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_FORTUNE_COOKIE_DUPLICATE_MARKER", "chat-fortune-cookie-123", "fortune-cookie-inbound-902", "fortune-cookie-notify-902", "fortune-cookie-secret-902", "release", "Make the launch card tiny.", pick.Entry.Fortune, pick.Entry.Prompt} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate fortune cookie receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelFortuneCookieActionRequestParsesRouteAliasAndTheme(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel fortune cookie"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel lucky-cookie team-demo debug --message-id source-1 --notify-message-id notify-1 --cookie-id Cookie.One
Note: Debug gently.`,
		},
	}
	req, err := BuildChannelFortuneCookieActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelFortuneCookieActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "lucky-cookie" || req.Options.Route != "team-demo" || req.Options.Theme != "debug" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.CookieID != "cookie-one" || req.FortuneCount != 5 || req.FortuneIndex < 1 || req.LuckyNumber < 1 || req.LuckyNumber > 99 {
		t.Fatalf("unexpected channel fortune cookie parsing: %#v", req)
	}
	if req.Options.Note != "Debug gently." || req.NoteSource != "trailing-note" || req.ThemeSource != "positional" {
		t.Fatalf("unexpected channel fortune cookie note/theme parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoCookieID || req.RequestedRouteHash == "" || req.CookieIDHash == "" || req.ThemeSHA == "" || req.DeckSHA == "" || req.FortuneSHA == "" || req.PromptSHA == "" || req.LuckyNumberSHA == "" || req.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route fortune cookie hashes: %#v", req)
	}
}
