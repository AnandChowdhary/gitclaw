package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelWatchCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-watch-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 262,
			"title": "GitClaw telegram thread chat-watch-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-watch-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26201,
			"body": "@gitclaw /channels watch --watch-id watch-1 --cadence hourly --message-id inbound-262 --notify-message-id notify-262\nSubject: Follow up on channel incident\nNotes:\nVisible watch notes with CHANNEL_WATCH_NOTES_SECRET.",
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
			Number: 262,
			Title:  "GitClaw telegram thread chat-watch-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{262: {{
			ID: 26200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-watch-123",
				MessageID: "inbound-262",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_WATCH_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{262: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel watch action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one watch issue: %#v", len(github.Issues), github.Issues)
	}
	watch := github.Issues[1]
	if !HasChannelWatchMarker(watch.Body) || !strings.Contains(watch.Body, `watch_id="watch-1"`) {
		t.Fatalf("watch issue missing channel-watch marker:\n%s", watch.Body)
	}
	for _, want := range []string{
		"GitClaw channel watch",
		"watch_id: watch-1",
		"source_channel: telegram",
		"source_issue: #262",
		"source_message_id_sha256_12:",
		"cadence: hourly",
		"watch_mode: github-issue-watch",
		"proactive_schedule_ready: true",
		"scheduler: github-actions-scheduled-workflow",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Follow up on channel incident",
		"Visible watch notes with CHANNEL_WATCH_NOTES_SECRET.",
	} {
		if !strings.Contains(watch.Body, want) {
			t.Fatalf("watch issue missing %q:\n%s", want, watch.Body)
		}
	}
	if strings.Contains(watch.Body, "chat-watch-123") || strings.Contains(watch.Body, "inbound-262") || strings.Contains(watch.Body, "CHANNEL_WATCH_INGEST_SECRET") {
		t.Fatalf("watch issue leaked provider IDs or channel body:\n%s", watch.Body)
	}
	if !hasLabel(github.IssueLabels[watch.Number], "gitclaw") {
		t.Fatalf("watch issue missing gitclaw trigger label: %#v", github.IssueLabels[watch.Number])
	}

	sourceComments := github.CommentsByIssue[262]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-262"`,
		"GitClaw channel watch created.",
		"Watch: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Follow up on channel incident",
		"Cadence: hourly",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("watch notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_WATCH_NOTES_SECRET") || strings.Contains(outbound, "CHANNEL_WATCH_INGEST_SECRET") {
		t.Fatalf("watch notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Watch Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels watch`",
		"channel_watch_status: `created`",
		"watch_issue: `#101`",
		"watch_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#262`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_watch_id_included: `false`",
		"raw_watch_cadence_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_watch_title_included: `false`",
		"raw_watch_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"proactive_watch_issue: `true`",
		"watch_scheduler: `github-actions-scheduled-workflow`",
		"llm_e2e_required_after_channel_watch_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel watch receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_WATCH_INGEST_SECRET", "CHANNEL_WATCH_NOTES_SECRET", "Follow up on channel incident", "watch-1", "chat-watch-123", "inbound-262", "notify-262", "hourly"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel watch receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 262,
			"title": "GitClaw telegram thread chat-watch-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-watch-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26202,
			"body": "@gitclaw /channels watch --watch-id watch-1 --cadence hourly --message-id inbound-262 --notify-message-id notify-262\nSubject: Follow up on channel incident\nNotes:\nDo not leak duplicate token CHANNEL_WATCH_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate watch created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[262]); got != 4 {
		t.Fatalf("duplicate watch posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[262])
	}
	duplicateReceipt := github.CommentsByIssue[262][3].Body
	for _, want := range []string{
		"channel_watch_status: `duplicate`",
		"watch_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate watch receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_WATCH_DUPLICATE_SECRET", "hourly"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate watch receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelWatchActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 29, Title: "Channel watch"},
		Comment: &Comment{
			ID: 2901,
			Body: `@gitclaw /channel monitor --route team-demo --watch-id Design.Watch --cadence daily --message-id source-1 --notify-message-id notify-1
Watch: Route follow-up watch
Notes:
Check the provider queue.`,
		},
	}
	req, err := BuildChannelWatchActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelWatchActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "monitor" || req.Options.Route != "team-demo" || req.Options.WatchID != "design-watch" || req.Options.Cadence != "daily" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel watch parsing: %#v", req)
	}
	if req.Options.Title != "Route follow-up watch" || !strings.Contains(req.Options.Notes, "Check the provider queue.") {
		t.Fatalf("unexpected watch title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoWatchID || req.AutoNotifyMessageID || req.CadenceSHA == "" || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route watch hashes: %#v", req)
	}
}
