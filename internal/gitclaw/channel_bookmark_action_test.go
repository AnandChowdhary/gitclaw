package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelBookmarkCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-bookmark-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-bookmark-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-bookmark-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26301,
			"body": "@gitclaw /channels bookmark-message --bookmark-id bookmark-1 --message-id inbound-263 --notify-message-id notify-263\nBookmark: Save this channel turning point\nReason:\nVisible bookmark reason with CHANNEL_BOOKMARK_REASON_SECRET.",
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
			Number: 263,
			Title:  "GitClaw telegram thread chat-bookmark-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{263: {{
			ID: 26300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-bookmark-123",
				MessageID: "inbound-263",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_BOOKMARK_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{263: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel bookmark action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one bookmark issue: %#v", len(github.Issues), github.Issues)
	}
	bookmark := github.Issues[1]
	if !HasChannelBookmarkMarker(bookmark.Body) || !strings.Contains(bookmark.Body, `bookmark_id="bookmark-1"`) {
		t.Fatalf("bookmark issue missing channel-bookmark marker:\n%s", bookmark.Body)
	}
	for _, want := range []string{
		"GitClaw channel bookmark",
		"bookmark_id: bookmark-1",
		"source_channel: telegram",
		"source_issue: #263",
		"source_message_id_sha256_12:",
		"reference_url_sha256_12: none",
		"reference_url_present: false",
		"bookmark_mode: github-issue-message-bookmark",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"raw_reference_url_included: false",
		"Save this channel turning point",
		"Visible bookmark reason with CHANNEL_BOOKMARK_REASON_SECRET.",
	} {
		if !strings.Contains(bookmark.Body, want) {
			t.Fatalf("bookmark issue missing %q:\n%s", want, bookmark.Body)
		}
	}
	if strings.Contains(bookmark.Body, "chat-bookmark-123") || strings.Contains(bookmark.Body, "inbound-263") || strings.Contains(bookmark.Body, "CHANNEL_BOOKMARK_INGEST_SECRET") {
		t.Fatalf("bookmark issue leaked provider IDs or channel body:\n%s", bookmark.Body)
	}
	if !hasLabel(github.IssueLabels[bookmark.Number], "gitclaw") {
		t.Fatalf("bookmark issue missing gitclaw trigger label: %#v", github.IssueLabels[bookmark.Number])
	}

	sourceComments := github.CommentsByIssue[263]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-263"`,
		"GitClaw channel bookmark saved.",
		"Bookmark: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Save this channel turning point",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("bookmark notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_BOOKMARK_REASON_SECRET") || strings.Contains(outbound, "CHANNEL_BOOKMARK_INGEST_SECRET") {
		t.Fatalf("bookmark notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Bookmark Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels bookmark-message`",
		"channel_bookmark_status: `saved`",
		"bookmark_issue: `#101`",
		"bookmark_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#263`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_bookmark_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_bookmark_title_included: `false`",
		"reference_url_present: `false`",
		"raw_reference_url_included: `false`",
		"raw_bookmark_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_bookmark_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel bookmark receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BOOKMARK_INGEST_SECRET", "CHANNEL_BOOKMARK_REASON_SECRET", "Save this channel turning point", "bookmark-1", "chat-bookmark-123", "inbound-263", "notify-263"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel bookmark receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-bookmark-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-bookmark-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26302,
			"body": "@gitclaw /channels bookmark-message --bookmark-id bookmark-1 --message-id inbound-263 --notify-message-id notify-263\nBookmark: Save this channel turning point\nReason:\nDo not leak duplicate token CHANNEL_BOOKMARK_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate bookmark created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[263]); got != 4 {
		t.Fatalf("duplicate bookmark posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[263])
	}
	duplicateReceipt := github.CommentsByIssue[263][3].Body
	for _, want := range []string{
		"channel_bookmark_status: `duplicate`",
		"bookmark_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate bookmark receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_BOOKMARK_DUPLICATE_SECRET") {
		t.Fatalf("duplicate bookmark receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelBookmarkActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel bookmark"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel preserve --route team-demo --bookmark-id Design.Bookmark --reference-url https://bookmarks.example.invalid/design-secret --message-id source-1 --notify-message-id notify-1
Bookmark: Route saved context
Context:
Keep this provider thread handy.`,
		},
	}
	req, err := BuildChannelBookmarkActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBookmarkActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "preserve" || req.Options.Route != "team-demo" || req.Options.BookmarkID != "design-bookmark" || req.Options.ReferenceURL != "https://bookmarks.example.invalid/design-secret" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel bookmark parsing: %#v", req)
	}
	if req.Options.Title != "Route saved context" || !strings.Contains(req.Options.Notes, "Keep this provider thread handy.") {
		t.Fatalf("unexpected bookmark title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoBookmarkID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.ReferenceURLSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route bookmark hashes: %#v", req)
	}
}
