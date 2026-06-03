package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelMergeCreatesIssueAndNotifiesTargetWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-merge-target-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 271,
			"title": "GitClaw telegram thread chat-merge-target-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-merge-target-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 27101,
			"body": "@gitclaw /channels merge --merge-id join-one --from-thread branch-source-271 --message-id inbound-271 --notify-message-id notify-271\nMerge: Rejoin design branch\nNotes:\nVisible merge notes with CHANNEL_MERGE_NOTES_SECRET.",
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
			Number: 271,
			Title:  "GitClaw telegram thread chat-merge-target-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{271: {{
			ID: 27100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-merge-target-123",
				MessageID: "inbound-271",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_MERGE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{271: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel merge action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one merge issue: %#v", len(github.Issues), github.Issues)
	}
	merge := github.Issues[1]
	if !HasChannelMergeMarker(merge.Body) || !strings.Contains(merge.Body, `merge_id="join-one"`) {
		t.Fatalf("merge issue missing channel merge marker:\n%s", merge.Body)
	}
	for _, want := range []string{
		"GitClaw channel thread merge",
		"merge_id: join-one",
		"channel: telegram",
		"source_issue: #271",
		"source_thread_id_sha256_12:",
		"target_thread_id_sha256_12:",
		"source_message_id_sha256_12:",
		"merge_mode: github-issue-channel-thread-merge",
		"raw_source_thread_id_included: false",
		"raw_target_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Rejoin design branch",
		"Visible merge notes with CHANNEL_MERGE_NOTES_SECRET.",
	} {
		if !strings.Contains(merge.Body, want) {
			t.Fatalf("merge issue missing %q:\n%s", want, merge.Body)
		}
	}
	for _, leaked := range []string{"chat-merge-target-123", "branch-source-271", "inbound-271", "CHANNEL_MERGE_INGEST_SECRET"} {
		if strings.Contains(merge.Body, leaked) {
			t.Fatalf("merge issue leaked %q:\n%s", leaked, merge.Body)
		}
	}
	if !hasLabel(github.IssueLabels[merge.Number], "gitclaw") || !hasLabel(github.IssueLabels[merge.Number], "gitclaw:channel") {
		t.Fatalf("merge issue missing gitclaw/channel labels: %#v", github.IssueLabels[merge.Number])
	}

	targetComments := github.CommentsByIssue[271]
	if len(targetComments) != 3 {
		t.Fatalf("target comments = %d, want message + outbound + receipt: %#v", len(targetComments), targetComments)
	}
	outbound := targetComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-271"`,
		"GitClaw channel thread merge recorded.",
		"Merge: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Rejoin design branch",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("merge notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MERGE_NOTES_SECRET", "CHANNEL_MERGE_INGEST_SECRET", "branch-source-271", "inbound-271", "join-one"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("merge notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := targetComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Merge Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels merge`",
		"channel_merge_status: `recorded`",
		"merge_issue: `#101`",
		"merge_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#271`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"channel: `telegram`",
		"target_from_current_channel_issue: `true`",
		"raw_merge_id_included: `false`",
		"raw_source_thread_id_included: `false`",
		"raw_target_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_merge_title_included: `false`",
		"raw_merge_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_merge_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel merge receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MERGE_INGEST_SECRET", "CHANNEL_MERGE_NOTES_SECRET", "Rejoin design branch", "join-one", "chat-merge-target-123", "branch-source-271", "inbound-271", "notify-271"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel merge receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 271,
			"title": "GitClaw telegram thread chat-merge-target-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-merge-target-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 27102,
			"body": "@gitclaw /channels merge --merge-id join-one --from-thread branch-source-271 --message-id inbound-271 --notify-message-id notify-271\nMerge: Rejoin design branch\nNotes:\nDo not leak duplicate token CHANNEL_MERGE_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate merge created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[271]); got != 4 {
		t.Fatalf("duplicate merge posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[271])
	}
	duplicateReceipt := github.CommentsByIssue[271][3].Body
	for _, want := range []string{
		"channel_merge_status: `duplicate`",
		"merge_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate merge receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_MERGE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate merge receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelMergeActionRequestParsesExplicitTarget(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel merge"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel converge --channel telegram --from-thread source-thread --to-thread target-thread --merge-id Design.Merge --message-id source-1 --notify-message-id notify-1
Merge: Route convergence
Context:
Keep this convergence record handy.`,
		},
	}
	req, err := BuildChannelMergeActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelMergeActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "converge" || req.Options.Channel != "telegram" || req.Options.SourceThreadID != "source-thread" || req.Options.TargetThreadID != "target-thread" || req.Options.MergeID != "design-merge" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel merge parsing: %#v", req)
	}
	if req.Options.Title != "Route convergence" || !strings.Contains(req.Options.Notes, "Keep this convergence record handy.") {
		t.Fatalf("unexpected merge title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoMergeID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.NotesSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit channel merge hashes: %#v", req)
	}
}
