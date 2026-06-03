package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelStoryDiceQueuesPromptDiceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-story-dice-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 906,
			"title": "GitClaw telegram thread chat-story-dice-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-story-dice-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90601,
			"body": "@gitclaw /channels story-dice fun --message-id story-dice-inbound-906 --notify-message-id story-dice-notify-906 --story-dice-id story-dice-secret-906\nNote: Give us a tiny playful thread move.\nDo not include this command hidden token in the receipt: CHANNEL_STORY_DICE_COMMAND_MARKER.",
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
			Number: 906,
			Title:  "GitClaw telegram thread chat-story-dice-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{906: {{
			ID: 90600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-story-dice-123",
				MessageID: "story-dice-inbound-906",
				Author:    "telegram",
				Body:      "Original mirrored story dice command with CHANNEL_STORY_DICE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{906: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel story dice action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("story dice action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[906]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="story-dice-notify-906"`,
		"GitClaw channel story dice.",
		"Theme: fun",
		"Dice:",
		"Opening:",
		"Constraint:",
		"Move:",
		"Button:",
		"Story dice hash: ",
		"Seed hash: ",
		"Note: Give us a tiny playful thread move.",
		"Note hash: ",
		"Story dice source: deterministic GitHub channel action seed.",
		"Story dice deck: bounded static GitClaw prompt deck.",
		"Model call: not performed by this action.",
		"External randomness: not used.",
		"Media generation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("story dice notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_STORY_DICE_INGEST_MARKER", "CHANNEL_STORY_DICE_COMMAND_MARKER", "story-dice-secret-906"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("story dice notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Story Dice Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels story-dice`",
		"channel_story_dice_status: `queued`",
		"story_dice_mode: `deterministic-channel-prompt-dice`",
		"notification_target_issue: `#906`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"story_dice_id_sha256_12: `",
		"story_dice_id_auto: `false`",
		"story_dice_theme_sha256_12: `",
		"story_dice_theme_bytes: `3`",
		"story_dice_note_sha256_12: `",
		"story_dice_note_bytes: `35`",
		"story_dice_note_lines: `1`",
		"story_dice_note_source: `trailing-note`",
		"story_dice_seed_sha256_12: `",
		"story_dice_roll_sha256_12: `",
		"story_dice_count: `4`",
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
		"raw_story_dice_id_included: `false`",
		"raw_story_dice_theme_included: `false`",
		"raw_story_dice_note_included: `false`",
		"raw_story_dice_roll_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_story_dice_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel story dice receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_STORY_DICE_INGEST_MARKER", "CHANNEL_STORY_DICE_COMMAND_MARKER", "chat-story-dice-123", "story-dice-inbound-906", "story-dice-notify-906", "story-dice-secret-906", "fun", "Give us a tiny playful thread move.", "a tiny side quest appears", "no magic, only hashes", "pick the surprising next command", "fun is better when it is inspectable"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel story dice receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 906,
			"title": "GitClaw telegram thread chat-story-dice-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-story-dice-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90602,
			"body": "@gitclaw /channels plot-dice fun --message-id story-dice-inbound-906 --notify-message-id story-dice-notify-906 --story-dice-id story-dice-secret-906\nNote: Give us a tiny playful thread move.\nDo not leak duplicate token CHANNEL_STORY_DICE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate story dice created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[906]); got != 4 {
		t.Fatalf("duplicate story dice posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[906])
	}
	duplicateReceipt := github.CommentsByIssue[906][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels plot-dice`",
		"channel_story_dice_status: `duplicate`",
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
			t.Fatalf("duplicate story dice receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_STORY_DICE_DUPLICATE_MARKER", "chat-story-dice-123", "story-dice-inbound-906", "story-dice-notify-906", "story-dice-secret-906", "fun", "Give us a tiny playful thread move."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate story dice receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelStoryDiceActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel story dice"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel riff --route team-demo --message-id source-1 --notify-message-id notify-1 --riff-id Story.Dice.One
Note: Tiny prompt.`,
		},
	}
	req, err := BuildChannelStoryDiceActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelStoryDiceActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "riff" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StoryDiceID != "story-dice-one" || req.Options.Theme != "fun" {
		t.Fatalf("unexpected channel story dice parsing: %#v", req)
	}
	if req.Options.Note != "Tiny prompt." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel story dice note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStoryDiceID || req.RequestedRouteHash == "" || req.StoryDiceIDHash == "" || req.ThemeSHA == "" || req.NoteSHA == "" || req.RollSHA == "" || req.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route story dice hashes: %#v", req)
	}
}

func TestBuildChannelStoryDiceActionRequestParsesPositionalRouteAndTheme(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels story-dice team-demo launch --message-id source-2 --notify-message-id notify-2 --story-dice-id Story.Dice.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelStoryDiceActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelStoryDiceActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Theme != "launch" || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel story dice parsing: %#v", req)
	}
}
