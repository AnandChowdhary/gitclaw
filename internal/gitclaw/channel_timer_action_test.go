package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelTimerQueuesTimeboxWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-timer-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 905,
			"title": "GitClaw telegram thread chat-timer-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-timer-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90501,
			"body": "@gitclaw /channels timer 25m --message-id timer-inbound-905 --notify-message-id timer-notify-905 --timer-id timer-secret-905 --mode focus\nLabel: focus-sprint\nNote: Pairing window is open.\nDo not include this command hidden token in the receipt: CHANNEL_TIMER_COMMAND_MARKER.",
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
			Number: 905,
			Title:  "GitClaw telegram thread chat-timer-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{905: {{
			ID: 90500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-timer-123",
				MessageID: "timer-inbound-905",
				Author:    "telegram",
				Body:      "Original mirrored timer command with CHANNEL_TIMER_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{905: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel timer action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("timer action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[905]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="timer-notify-905"`,
		"GitClaw channel timer.",
		"Timer: focus-sprint",
		"Duration: 25 minutes",
		"Mode: focus",
		"Note: Pairing window is open.",
		"Timer hash: ",
		"Duration seconds: 1500",
		"Note hash: ",
		"Timer source: GitHub channel action.",
		"Scheduled reminder: not created by this action.",
		"Provider timer: not started by this action.",
		"Model call: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("timer notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TIMER_INGEST_MARKER", "CHANNEL_TIMER_COMMAND_MARKER", "timer-secret-905"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("timer notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Timer Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels timer`",
		"channel_timer_status: `queued`",
		"timer_mode: `provider-facing-timebox-cue`",
		"notification_target_issue: `#905`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"timer_id_sha256_12: `",
		"timer_id_auto: `false`",
		"timer_duration_sha256_12: `",
		"timer_duration_seconds: `1500`",
		"timer_duration_source: `positional-duration`",
		"timer_label_sha256_12: `",
		"timer_label_bytes: `12`",
		"timer_label_lines: `1`",
		"timer_label_source: `trailing-label`",
		"timer_mode_sha256_12: `",
		"timer_mode_bytes: `5`",
		"timer_note_sha256_12: `",
		"timer_note_bytes: `23`",
		"timer_note_lines: `1`",
		"timer_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"reminder_issue_created: `false`",
		"scheduled_workflow_created: `false`",
		"provider_timer_started: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_timer_id_included: `false`",
		"raw_timer_duration_included: `false`",
		"raw_timer_label_included: `false`",
		"raw_timer_note_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_timer_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel timer receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TIMER_INGEST_MARKER", "CHANNEL_TIMER_COMMAND_MARKER", "chat-timer-123", "timer-inbound-905", "timer-notify-905", "timer-secret-905", "25m", "focus-sprint", "Pairing window is open."} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel timer receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 905,
			"title": "GitClaw telegram thread chat-timer-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-timer-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90502,
			"body": "@gitclaw /channels timebox 25m --message-id timer-inbound-905 --notify-message-id timer-notify-905 --timer-id timer-secret-905 --mode focus\nLabel: focus-sprint\nNote: Pairing window is open.\nDo not leak duplicate token CHANNEL_TIMER_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate timer created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[905]); got != 4 {
		t.Fatalf("duplicate timer posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[905])
	}
	duplicateReceipt := github.CommentsByIssue[905][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels timebox`",
		"channel_timer_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"reminder_issue_created: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel timer receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TIMER_DUPLICATE_MARKER", "chat-timer-123", "timer-inbound-905", "timer-notify-905", "timer-secret-905", "25m", "focus-sprint", "Pairing window is open."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel timer receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelTimerActionRequestParsesRouteAliasAndMinutes(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel timer"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel pomodoro --route team-demo --message-id source-1 --notify-message-id notify-1 --timer-id Timer.One --minutes 30 --label review-window --mode pair
Note: Pair before handoff.`,
		},
	}
	req, err := BuildChannelTimerActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelTimerActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "pomodoro" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.TimerID != "timer-one" || req.Options.Duration != "30m" || req.Options.DurationSeconds != 1800 || req.Options.Label != "review-window" || req.Options.Mode != "pair" || req.Options.Note != "Pair before handoff." {
		t.Fatalf("unexpected channel timer parsing: %#v", req)
	}
	if req.DurationSource != "flag" || req.LabelSource != "flag" || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel timer source parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoTimerID || req.RequestedRouteHash == "" || req.TimerIDHash == "" || req.DurationSHA == "" || req.LabelSHA == "" || req.ModeSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route timer hashes: %#v", req)
	}
}
