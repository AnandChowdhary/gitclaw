package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelForkCreatesThreadAndNotifiesSourceWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-fork-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 270,
			"title": "GitClaw telegram thread chat-fork-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-fork-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 27001,
			"body": "@gitclaw /channels fork --fork-id branch-one --new-thread-id child-thread-270 --message-id inbound-270 --notify-message-id notify-270\nFork: Follow-up design branch\nNotes:\nVisible fork notes with CHANNEL_FORK_NOTES_SECRET.",
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
			Number: 270,
			Title:  "GitClaw telegram thread chat-fork-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{270: {{
			ID: 27000,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-fork-123",
				MessageID: "inbound-270",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_FORK_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{270: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel fork action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one fork issue: %#v", len(github.Issues), github.Issues)
	}
	fork := github.Issues[1]
	if !HasChannelThreadMarker(fork.Body) || !HasChannelForkMarker(fork.Body) || !strings.Contains(fork.Body, `fork_id="branch-one"`) {
		t.Fatalf("fork issue missing channel thread/fork markers:\n%s", fork.Body)
	}
	for _, want := range []string{
		"GitClaw forked channel thread",
		`thread_id="child-thread-270"`,
		"thread_id: child-thread-270",
		"fork_id: branch-one",
		"channel: telegram",
		"source_issue: #270",
		"source_thread_id_sha256_12:",
		"source_message_id_sha256_12:",
		"fork_mode: github-issue-channel-thread-fork",
		"raw_source_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Follow-up design branch",
		"Visible fork notes with CHANNEL_FORK_NOTES_SECRET.",
	} {
		if !strings.Contains(fork.Body, want) {
			t.Fatalf("fork issue missing %q:\n%s", want, fork.Body)
		}
	}
	for _, leaked := range []string{"chat-fork-123", "inbound-270", "CHANNEL_FORK_INGEST_SECRET"} {
		if strings.Contains(fork.Body, leaked) {
			t.Fatalf("fork issue leaked %q:\n%s", leaked, fork.Body)
		}
	}
	if !hasLabel(github.IssueLabels[fork.Number], "gitclaw") || !hasLabel(github.IssueLabels[fork.Number], "gitclaw:channel") {
		t.Fatalf("fork issue missing gitclaw/channel labels: %#v", github.IssueLabels[fork.Number])
	}

	sourceComments := github.CommentsByIssue[270]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-270"`,
		"GitClaw channel thread forked.",
		"Fork: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Follow-up design branch",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("fork notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_FORK_NOTES_SECRET", "CHANNEL_FORK_INGEST_SECRET", "inbound-270", "branch-one"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("fork notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Fork Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels fork`",
		"channel_fork_status: `created`",
		"fork_issue: `#101`",
		"fork_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#270`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"channel: `telegram`",
		"target_from_current_channel_issue: `true`",
		"raw_fork_id_included: `false`",
		"raw_source_thread_id_included: `false`",
		"raw_target_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_fork_title_included: `false`",
		"raw_fork_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_fork_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel fork receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_FORK_INGEST_SECRET", "CHANNEL_FORK_NOTES_SECRET", "Follow-up design branch", "branch-one", "chat-fork-123", "child-thread-270", "inbound-270", "notify-270"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel fork receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 270,
			"title": "GitClaw telegram thread chat-fork-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-fork-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 27002,
			"body": "@gitclaw /channels fork --fork-id branch-one --new-thread-id child-thread-270 --message-id inbound-270 --notify-message-id notify-270\nFork: Follow-up design branch\nNotes:\nDo not leak duplicate token CHANNEL_FORK_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate fork created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[270]); got != 4 {
		t.Fatalf("duplicate fork posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[270])
	}
	duplicateReceipt := github.CommentsByIssue[270][3].Body
	for _, want := range []string{
		"channel_fork_status: `duplicate`",
		"fork_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate fork receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_FORK_DUPLICATE_SECRET") {
		t.Fatalf("duplicate fork receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelForkActionRequestParsesExplicitSource(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel fork"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel branch --channel telegram --from-thread source-thread --to-thread target-thread --fork-id Design.Fork --message-id source-1 --notify-message-id notify-1
Fork: Route branch
Context:
Keep this provider branch handy.`,
		},
	}
	req, err := BuildChannelForkActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelForkActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "branch" || req.Options.Channel != "telegram" || req.Options.SourceThreadID != "source-thread" || req.Options.TargetThreadID != "target-thread" || req.Options.ForkID != "design-fork" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel fork parsing: %#v", req)
	}
	if req.Options.Title != "Route branch" || !strings.Contains(req.Options.Notes, "Keep this provider branch handy.") {
		t.Fatalf("unexpected fork title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoForkID || req.AutoTargetThreadID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.NotesSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit channel fork hashes: %#v", req)
	}
}
