package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelDigestCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-digest-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-digest-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-digest-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels digest --digest-id digest-1 --message-id inbound-384 --notify-message-id notify-384\nSummary: Team settled the launch readiness plan\nHighlights:\nVisible digest highlight with CHANNEL_DIGEST_HIGHLIGHT_SECRET.",
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
			Title:  "GitClaw telegram thread chat-digest-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-digest-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_DIGEST_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel digest action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one digest issue: %#v", len(github.Issues), github.Issues)
	}
	digest := github.Issues[1]
	if !HasChannelDigestMarker(digest.Body) || !strings.Contains(digest.Body, `digest_id="digest-1"`) {
		t.Fatalf("digest issue missing channel-digest marker:\n%s", digest.Body)
	}
	for _, want := range []string{
		"GitClaw channel digest",
		"digest_id: digest-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"digest_mode: github-issue-digest",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Team settled the launch readiness plan",
		"Visible digest highlight with CHANNEL_DIGEST_HIGHLIGHT_SECRET.",
	} {
		if !strings.Contains(digest.Body, want) {
			t.Fatalf("digest issue missing %q:\n%s", want, digest.Body)
		}
	}
	if strings.Contains(digest.Body, "chat-digest-123") || strings.Contains(digest.Body, "inbound-384") || strings.Contains(digest.Body, "CHANNEL_DIGEST_INGEST_SECRET") {
		t.Fatalf("digest issue leaked provider IDs or channel body:\n%s", digest.Body)
	}
	if !hasLabel(github.IssueLabels[digest.Number], "gitclaw") {
		t.Fatalf("digest issue missing gitclaw trigger label: %#v", github.IssueLabels[digest.Number])
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
		"GitClaw channel digest recorded.",
		"Digest: #101",
		"https://github.com/owner/repo/issues/101",
		"Summary: Team settled the launch readiness plan",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("digest notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_DIGEST_HIGHLIGHT_SECRET") || strings.Contains(outbound, "CHANNEL_DIGEST_INGEST_SECRET") {
		t.Fatalf("digest notification leaked highlights or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Digest Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels digest`",
		"channel_digest_status: `recorded`",
		"digest_issue: `#101`",
		"digest_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_digest_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_digest_summary_included: `false`",
		"raw_digest_highlights_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_digest_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel digest receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_DIGEST_INGEST_SECRET", "CHANNEL_DIGEST_HIGHLIGHT_SECRET", "Team settled the launch", "digest-1", "chat-digest-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel digest receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-digest-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-digest-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels digest --digest-id digest-1 --message-id inbound-384 --notify-message-id notify-384\nSummary: Team settled the launch readiness plan\nHighlights:\nDo not leak duplicate token CHANNEL_DIGEST_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate digest created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate digest posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_digest_status: `duplicate`",
		"digest_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate digest receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_DIGEST_DUPLICATE_SECRET") {
		t.Fatalf("duplicate digest receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelDigestActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel digest"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel brief --route team-demo --digest-id Weekly.Brief --message-id source-1 --notify-message-id notify-1
Summary: The channel reached launch readiness
Highlights:
- Design is stable.
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelDigestActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelDigestActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "brief" || req.Options.Route != "team-demo" || req.Options.DigestID != "weekly-brief" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel digest parsing: %#v", req)
	}
	if req.Options.Summary != "The channel reached launch readiness" || !strings.Contains(req.Options.Highlights, "Design is stable") {
		t.Fatalf("unexpected summary/highlights: %#v", req)
	}
	if req.TargetFromIssue || req.AutoDigestID || req.AutoNotifyMessageID || req.SummarySHA == "" || req.HighlightsSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route digest hashes: %#v", req)
	}
}
