package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelRiddleQueuesDeterministicCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-riddle-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 902,
			"title": "GitClaw telegram thread chat-riddle-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-riddle-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90201,
			"body": "@gitclaw /channels riddle release --message-id riddle-inbound-902 --notify-message-id riddle-notify-902 --riddle-id riddle-secret-902\nNote: Make the launch card tiny.\nDo not include this command hidden token in the receipt: CHANNEL_RIDDLE_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-riddle-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{902: {{
			ID: 90200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-riddle-123",
				MessageID: "riddle-inbound-902",
				Author:    "telegram",
				Body:      "Original mirrored riddle command with CHANNEL_RIDDLE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{902: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}
	pick := buildChannelRiddlePick(ChannelRiddleOptions{
		Repo:            "owner/repo",
		Channel:         "telegram",
		ThreadID:        "chat-riddle-123",
		SourceMessageID: "riddle-inbound-902",
		NotifyMessageID: "riddle-notify-902",
		RiddleID:        "riddle-secret-902",
		Theme:           "release",
		Note:            "Make the launch card tiny.",
	})

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel riddle action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("riddle action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[902]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="riddle-notify-902"`,
		"GitClaw channel riddle.",
		"Theme: release",
		"Picked: #",
		"Riddle: " + pick.Entry.Question,
		"Hint: " + pick.Entry.Hint,
		"Answer: " + pick.Entry.Answer,
		"Question hash: ",
		"Hint hash: ",
		"Answer hash: ",
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
			t.Fatalf("riddle notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_RIDDLE_INGEST_MARKER", "CHANNEL_RIDDLE_COMMAND_MARKER", "riddle-secret-902"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("riddle notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Riddle Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels riddle`",
		"channel_riddle_status: `queued`",
		"riddle_mode: `deterministic-channel-riddle`",
		"notification_target_issue: `#902`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"riddle_id_sha256_12: `",
		"riddle_id_auto: `false`",
		"riddle_theme_sha256_12: `",
		"riddle_theme_bytes: `7`",
		"riddle_theme_terms: `1`",
		"riddle_theme_source: `positional`",
		"riddle_count: `5`",
		"riddle_index: `",
		"riddle_deck_sha256_12: `",
		"riddle_question_sha256_12: `",
		"riddle_hint_sha256_12: `",
		"riddle_answer_sha256_12: `",
		"riddle_seed_sha256_12: `",
		"riddle_note_sha256_12: `",
		"riddle_note_bytes: `26`",
		"riddle_note_lines: `1`",
		"riddle_note_source: `trailing-note`",
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
		"raw_riddle_id_included: `false`",
		"raw_riddle_theme_included: `false`",
		"raw_riddle_note_included: `false`",
		"raw_riddle_deck_included: `false`",
		"raw_riddle_question_included: `false`",
		"raw_riddle_hint_included: `false`",
		"raw_riddle_answer_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_riddle_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel riddle receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_RIDDLE_INGEST_MARKER", "CHANNEL_RIDDLE_COMMAND_MARKER", "chat-riddle-123", "riddle-inbound-902", "riddle-notify-902", "riddle-secret-902", "release", "Make the launch card tiny.", pick.Entry.Question, pick.Entry.Hint, pick.Entry.Answer} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel riddle receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 902,
			"title": "GitClaw telegram thread chat-riddle-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-riddle-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90202,
			"body": "@gitclaw /channels riddle release --message-id riddle-inbound-902 --notify-message-id riddle-notify-902 --riddle-id riddle-secret-902\nNote: Make the launch card tiny.\nDo not leak duplicate token CHANNEL_RIDDLE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate riddle created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[902]); got != 4 {
		t.Fatalf("duplicate riddle posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[902])
	}
	duplicateReceipt := github.CommentsByIssue[902][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels riddle`",
		"channel_riddle_status: `duplicate`",
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
			t.Fatalf("duplicate riddle receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_RIDDLE_DUPLICATE_MARKER", "chat-riddle-123", "riddle-inbound-902", "riddle-notify-902", "riddle-secret-902", "release", "Make the launch card tiny.", pick.Entry.Question, pick.Entry.Hint, pick.Entry.Answer} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate riddle receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelRiddleActionRequestParsesRouteAliasAndTheme(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel riddle"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel brain-teaser team-demo debug --message-id source-1 --notify-message-id notify-1 --riddle-id Riddle.One
Note: Debug gently.`,
		},
	}
	req, err := BuildChannelRiddleActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRiddleActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "brain-teaser" || req.Options.Route != "team-demo" || req.Options.Theme != "debug" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.RiddleID != "riddle-one" || req.RiddleCount != 5 || req.RiddleIndex < 1 {
		t.Fatalf("unexpected channel riddle parsing: %#v", req)
	}
	if req.Options.Note != "Debug gently." || req.NoteSource != "trailing-note" || req.ThemeSource != "positional" {
		t.Fatalf("unexpected channel riddle note/theme parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoRiddleID || req.RequestedRouteHash == "" || req.RiddleIDHash == "" || req.ThemeSHA == "" || req.DeckSHA == "" || req.QuestionSHA == "" || req.HintSHA == "" || req.AnswerSHA == "" || req.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route riddle hashes: %#v", req)
	}
}
