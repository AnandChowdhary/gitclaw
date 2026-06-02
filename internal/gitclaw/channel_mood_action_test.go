package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelMoodQueuesPresenceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-mood-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-mood-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-mood-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels mood focused --message-id mood-inbound-901 --notify-message-id mood-notify-901 --mood-id mood-secret-901 --intensity 4\nNote: Release handoff is steady.\nDo not include this command hidden token in the receipt: CHANNEL_MOOD_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-mood-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-mood-123",
				MessageID: "mood-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored mood command with CHANNEL_MOOD_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel mood action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("mood action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="mood-notify-901"`,
		"GitClaw channel mood.",
		"Mood: focused",
		"Intensity: 4/5",
		"Note: Release handoff is steady.",
		"Mood hash: ",
		"Note hash: ",
		"Presence source: GitHub channel action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("mood notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MOOD_INGEST_MARKER", "CHANNEL_MOOD_COMMAND_MARKER", "mood-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("mood notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Mood Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels mood`",
		"channel_mood_status: `queued`",
		"mood_mode: `structured-channel-presence`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"mood_id_sha256_12: `",
		"mood_id_auto: `false`",
		"mood_sha256_12: `",
		"mood_bytes: `7`",
		"mood_intensity_level: `4`",
		"mood_note_sha256_12: `",
		"mood_note_bytes: `26`",
		"mood_note_lines: `1`",
		"mood_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_mood_id_included: `false`",
		"raw_mood_included: `false`",
		"raw_mood_note_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_mood_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel mood receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MOOD_INGEST_MARKER", "CHANNEL_MOOD_COMMAND_MARKER", "chat-mood-123", "mood-inbound-901", "mood-notify-901", "mood-secret-901", "focused", "Release handoff is steady."} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel mood receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-mood-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-mood-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels vibe focused --message-id mood-inbound-901 --notify-message-id mood-notify-901 --mood-id mood-secret-901 --intensity 4\nNote: Release handoff is steady.\nDo not leak duplicate token CHANNEL_MOOD_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate mood created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate mood posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels vibe`",
		"channel_mood_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate mood receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MOOD_DUPLICATE_MARKER", "chat-mood-123", "mood-inbound-901", "mood-notify-901", "mood-secret-901", "focused", "Release handoff is steady."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate mood receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelMoodActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel mood"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel pulse --route team-demo --message-id source-1 --notify-message-id notify-1 --mood-id Mood.One --intensity 5
Note: Almost there.`,
		},
	}
	req, err := BuildChannelMoodActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelMoodActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "pulse" || req.Options.Route != "team-demo" || req.Options.Mood != "present" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MoodID != "mood-one" || req.Options.Intensity != 5 {
		t.Fatalf("unexpected channel mood parsing: %#v", req)
	}
	if req.Options.Note != "Almost there." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel mood note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMoodID || req.RequestedRouteHash == "" || req.MoodIDHash == "" || req.MoodSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route mood hashes: %#v", req)
	}
}

func TestBuildChannelMoodActionRequestParsesPositionalRouteAndMood(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels vibe team-demo blocked --message-id source-2 --notify-message-id notify-2 --mood-id Mood.Two --intensity 2",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelMoodActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelMoodActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Mood != "blocked" || req.Options.Intensity != 2 || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel mood parsing: %#v", req)
	}
}
