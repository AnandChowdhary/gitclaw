package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHandleChannelSnippetCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-snippet-123",
	})
	commentBody := strings.Join([]string{
		"@gitclaw /channels snippet --snippet-id snippet-1 --language go-secret --message-id inbound-472 --notify-message-id notify-472",
		"Title: Preserve channel parser snippet",
		"Snippet:",
		"```go",
		"func channelSnippetSecret() string {",
		"\treturn \"CHANNEL_SNIPPET_CODE_SECRET\"",
		"}",
		"```",
		"Notes:",
		"Visible snippet notes with CHANNEL_SNIPPET_NOTES_SECRET.",
	}, "\n")
	ev, err := ParseEvent("issue_comment", []byte(fmt.Sprintf(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 472,
			"title": "GitClaw telegram thread chat-snippet-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-snippet-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 47201,
			"body": %q,
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`, commentBody)))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 472,
			Title:  "GitClaw telegram thread chat-snippet-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{472: {{
			ID: 47200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-snippet-123",
				MessageID: "inbound-472",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_SNIPPET_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{472: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel snippet action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one snippet issue: %#v", len(github.Issues), github.Issues)
	}
	snippet := github.Issues[1]
	if !HasChannelSnippetMarker(snippet.Body) || !strings.Contains(snippet.Body, `snippet_id="snippet-1"`) {
		t.Fatalf("snippet issue missing channel-snippet marker:\n%s", snippet.Body)
	}
	for _, want := range []string{
		"GitClaw channel snippet",
		"snippet_id: snippet-1",
		"source_channel: telegram",
		"source_issue: #472",
		"source_message_id_sha256_12:",
		"language: go-secret",
		"snippet_sha256_12:",
		"snippet_mode: github-issue-snippet",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Preserve channel parser snippet",
		"func channelSnippetSecret() string",
		"CHANNEL_SNIPPET_CODE_SECRET",
		"Visible snippet notes with CHANNEL_SNIPPET_NOTES_SECRET.",
	} {
		if !strings.Contains(snippet.Body, want) {
			t.Fatalf("snippet issue missing %q:\n%s", want, snippet.Body)
		}
	}
	if strings.Contains(snippet.Body, "chat-snippet-123") || strings.Contains(snippet.Body, "inbound-472") || strings.Contains(snippet.Body, "CHANNEL_SNIPPET_INGEST_SECRET") {
		t.Fatalf("snippet issue leaked provider IDs or channel body:\n%s", snippet.Body)
	}
	if !hasLabel(github.IssueLabels[snippet.Number], "gitclaw") {
		t.Fatalf("snippet issue missing gitclaw trigger label: %#v", github.IssueLabels[snippet.Number])
	}

	sourceComments := github.CommentsByIssue[472]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-472"`,
		"GitClaw channel snippet saved.",
		"Snippet: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Preserve channel parser snippet",
		"Language: go-secret",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("snippet notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_SNIPPET_CODE_SECRET") || strings.Contains(outbound, "CHANNEL_SNIPPET_NOTES_SECRET") || strings.Contains(outbound, "CHANNEL_SNIPPET_INGEST_SECRET") {
		t.Fatalf("snippet notification leaked body, notes, or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Snippet Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels snippet`",
		"channel_snippet_status: `saved`",
		"snippet_issue: `#101`",
		"snippet_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#472`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"snippet_language_sha256_12:",
		"snippet_body_sha256_12:",
		"snippet_body_bytes:",
		"snippet_body_lines:",
		"raw_snippet_id_included: `false`",
		"raw_snippet_language_included: `false`",
		"raw_snippet_body_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_snippet_title_included: `false`",
		"raw_snippet_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_snippet_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel snippet receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SNIPPET_INGEST_SECRET", "CHANNEL_SNIPPET_CODE_SECRET", "CHANNEL_SNIPPET_NOTES_SECRET", "Preserve channel parser snippet", "snippet-1", "go-secret", "chat-snippet-123", "inbound-472", "notify-472"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel snippet receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateBody := strings.Join([]string{
		"@gitclaw /channels snippet --snippet-id snippet-1 --language go-secret --message-id inbound-472 --notify-message-id notify-472",
		"Title: Preserve channel parser snippet",
		"Snippet:",
		"```go",
		"func duplicateSnippetSecret() string { return \"CHANNEL_SNIPPET_DUPLICATE_CODE_SECRET\" }",
		"```",
		"Notes:",
		"Do not leak duplicate token CHANNEL_SNIPPET_DUPLICATE_SECRET.",
	}, "\n")
	duplicateEv, err := ParseEvent("issue_comment", []byte(fmt.Sprintf(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 472,
			"title": "GitClaw telegram thread chat-snippet-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-snippet-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 47202,
			"body": %q,
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`, duplicateBody)))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, cfg, github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate snippet created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[472]); got != 4 {
		t.Fatalf("duplicate snippet posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[472])
	}
	duplicateReceipt := github.CommentsByIssue[472][3].Body
	for _, want := range []string{
		"channel_snippet_status: `duplicate`",
		"snippet_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate snippet receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SNIPPET_DUPLICATE_SECRET", "CHANNEL_SNIPPET_DUPLICATE_CODE_SECRET"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate snippet receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSnippetActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel snippet"},
		Comment: &Comment{
			ID: 3001,
			Body: strings.Join([]string{
				"@gitclaw /channel paste --route team-demo --snippet-id Design.Snippet --message-id source-1 --notify-message-id notify-1",
				"Title: Persist TypeScript bridge snippet",
				"Snippet:",
				"```ts",
				"export const bridgeSnippet = \"CHANNEL_SNIPPET_ROUTE_CODE\";",
				"```",
				"Notes:",
				"Keep this provider thread code handy.",
			}, "\n"),
		},
	}
	req, err := BuildChannelSnippetActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelSnippetActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "paste" || req.Options.Route != "team-demo" || req.Options.SnippetID != "design-snippet" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel snippet parsing: %#v", req)
	}
	if req.Options.Title != "Persist TypeScript bridge snippet" || req.Options.Language != "ts" || !strings.Contains(req.Options.Snippet, "CHANNEL_SNIPPET_ROUTE_CODE") || !strings.Contains(req.Options.Notes, "Keep this provider thread code handy.") {
		t.Fatalf("unexpected snippet fields: %#v", req)
	}
	if req.TargetFromIssue || req.AutoSnippetID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.SnippetSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route snippet hashes: %#v", req)
	}
}
