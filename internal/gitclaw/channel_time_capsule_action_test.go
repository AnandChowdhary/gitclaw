package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelTimeCapsuleCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-time-capsule-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-time-capsule-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-time-capsule-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels time-capsule --capsule-id capsule-1 --open-after 2030-01-01 --message-id inbound-384 --notify-message-id notify-384\nTitle: Open when the roadmap feels stale\nMessage:\nVisible time capsule message with CHANNEL_TIME_CAPSULE_MESSAGE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 384,
			Title:  "GitClaw telegram thread chat-time-capsule-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-time-capsule-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_TIME_CAPSULE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel time capsule action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one capsule issue: %#v", len(github.Issues), github.Issues)
	}
	capsule := github.Issues[1]
	if !HasChannelTimeCapsuleMarker(capsule.Body) || !strings.Contains(capsule.Body, `capsule_id="capsule-1"`) {
		t.Fatalf("capsule issue missing channel-time-capsule marker:\n%s", capsule.Body)
	}
	for _, want := range []string{
		"GitClaw channel time capsule",
		"capsule_id: capsule-1",
		"open_after: 2030-01-01",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"time_capsule_mode: github-issue-time-capsule",
		"scheduled_delivery_created: false",
		"reminder_created: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Open After",
		"2030-01-01",
		"Open when the roadmap feels stale",
		"## Message",
		"Visible time capsule message with CHANNEL_TIME_CAPSULE_MESSAGE_SECRET.",
	} {
		if !strings.Contains(capsule.Body, want) {
			t.Fatalf("capsule issue missing %q:\n%s", want, capsule.Body)
		}
	}
	if strings.Contains(capsule.Body, "chat-time-capsule-123") || strings.Contains(capsule.Body, "inbound-384") || strings.Contains(capsule.Body, "CHANNEL_TIME_CAPSULE_INGEST_SECRET") {
		t.Fatalf("capsule issue leaked provider IDs or channel body:\n%s", capsule.Body)
	}
	if !hasLabel(github.IssueLabels[capsule.Number], "gitclaw") {
		t.Fatalf("capsule issue missing gitclaw trigger label: %#v", github.IssueLabels[capsule.Number])
	}

	sourceComments := github.CommentsByIssue[384]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-384"`,
		"GitClaw channel time capsule recorded.",
		"Time capsule: #101",
		"https://github.com/owner/repo/issues/101",
		"Open after: 2030-01-01",
		"Title: Open when the roadmap feels stale",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("time capsule notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_TIME_CAPSULE_MESSAGE_SECRET") || strings.Contains(outbound, "CHANNEL_TIME_CAPSULE_INGEST_SECRET") {
		t.Fatalf("time capsule notification leaked message details or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Time Capsule Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels time-capsule`",
		"channel_time_capsule_status: `recorded`",
		"capsule_issue: `#101`",
		"capsule_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_capsule_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"open_after_sha256_12:",
		"open_after_bytes: `10`",
		"open_after_lines: `1`",
		"raw_open_after_included: `false`",
		"raw_capsule_title_included: `false`",
		"raw_capsule_message_included: `false`",
		"raw_channel_message_body_included: `false`",
		"scheduled_delivery_created: `false`",
		"reminder_created: `false`",
		"workflow_created: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_time_capsule_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel time capsule receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TIME_CAPSULE_INGEST_SECRET", "CHANNEL_TIME_CAPSULE_MESSAGE_SECRET", "Open when the roadmap", "2030-01-01", "capsule-1", "chat-time-capsule-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel time capsule receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-time-capsule-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-time-capsule-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels time-capsule --capsule-id capsule-1 --open-after 2030-01-01 --message-id inbound-384 --notify-message-id notify-384\nTitle: Open when the roadmap feels stale\nMessage:\nDo not leak duplicate token CHANNEL_TIME_CAPSULE_DUPLICATE_SECRET.",
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
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate time capsule created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate time capsule posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_time_capsule_status: `duplicate`",
		"capsule_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate time capsule receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_TIME_CAPSULE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate time capsule receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelTimeCapsuleActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel time capsule"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel future-note --route team-demo --capsule-id Future.Self --open-after 2030-01-01 --message-id source-1 --notify-message-id notify-1
Title: Open when the channel roadmap feels stale
Message:
- Re-read the old constraints.
- Turn the useful part into a GitHub issue.`,
		},
	}
	req, err := BuildChannelTimeCapsuleActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelTimeCapsuleActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "future-note" || req.Options.Route != "team-demo" || req.Options.CapsuleID != "future-self" || req.Options.OpenAfter != "2030-01-01" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel time capsule parsing: %#v", req)
	}
	if req.Options.Title != "Open when the channel roadmap feels stale" || !strings.Contains(req.Options.Message, "Re-read the old constraints") {
		t.Fatalf("unexpected title/message: %#v", req)
	}
	if req.TargetFromIssue || req.AutoCapsuleID || req.AutoNotifyMessageID || req.OpenAfterSHA == "" || req.TitleSHA == "" || req.MessageSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route time capsule hashes: %#v", req)
	}
}
