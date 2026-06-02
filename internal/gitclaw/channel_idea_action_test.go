package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelIdeaCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-idea-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-idea-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-idea-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48401,
			"body": "@gitclaw /channels idea --idea-id idea-1 --message-id inbound-484 --notify-message-id notify-484\nIdea: Build a channel-native idea incubator\nNotes:\nVisible idea note with CHANNEL_IDEA_NOTE_SECRET.",
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
			Number: 484,
			Title:  "GitClaw telegram thread chat-idea-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{484: {{
			ID: 48400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-idea-123",
				MessageID: "inbound-484",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_IDEA_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{484: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel idea action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one idea issue: %#v", len(github.Issues), github.Issues)
	}
	idea := github.Issues[1]
	if !HasChannelIdeaMarker(idea.Body) || !strings.Contains(idea.Body, `idea_id="idea-1"`) {
		t.Fatalf("idea issue missing channel-idea marker:\n%s", idea.Body)
	}
	for _, want := range []string{
		"GitClaw channel idea",
		"idea_id: idea-1",
		"source_channel: telegram",
		"source_issue: #484",
		"source_message_id_sha256_12:",
		"idea_mode: github-issue-idea",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Build a channel-native idea incubator",
		"Visible idea note with CHANNEL_IDEA_NOTE_SECRET.",
	} {
		if !strings.Contains(idea.Body, want) {
			t.Fatalf("idea issue missing %q:\n%s", want, idea.Body)
		}
	}
	if strings.Contains(idea.Body, "chat-idea-123") || strings.Contains(idea.Body, "inbound-484") || strings.Contains(idea.Body, "CHANNEL_IDEA_INGEST_SECRET") {
		t.Fatalf("idea issue leaked provider IDs or channel body:\n%s", idea.Body)
	}
	if !hasLabel(github.IssueLabels[idea.Number], "gitclaw") {
		t.Fatalf("idea issue missing gitclaw trigger label: %#v", github.IssueLabels[idea.Number])
	}

	sourceComments := github.CommentsByIssue[484]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-484"`,
		"GitClaw channel idea captured.",
		"Idea: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Build a channel-native idea incubator",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("idea notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_IDEA_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_IDEA_INGEST_SECRET") {
		t.Fatalf("idea notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Idea Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels idea`",
		"channel_idea_status: `captured`",
		"idea_issue: `#101`",
		"idea_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#484`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_idea_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_idea_title_included: `false`",
		"raw_idea_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_idea_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel idea receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_IDEA_INGEST_SECRET", "CHANNEL_IDEA_NOTE_SECRET", "Build a channel-native", "idea-1", "chat-idea-123", "inbound-484", "notify-484"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel idea receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-idea-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-idea-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48402,
			"body": "@gitclaw /channels idea --idea-id idea-1 --message-id inbound-484 --notify-message-id notify-484\nIdea: Build a channel-native idea incubator\nNotes:\nDo not leak duplicate token CHANNEL_IDEA_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate idea created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[484]); got != 4 {
		t.Fatalf("duplicate idea posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[484])
	}
	duplicateReceipt := github.CommentsByIssue[484][3].Body
	for _, want := range []string{
		"channel_idea_status: `duplicate`",
		"idea_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate idea receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_IDEA_DUPLICATE_SECRET") {
		t.Fatalf("duplicate idea receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelIdeaActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel idea"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel brainstorm --route team-demo --idea-id Roadmap.Spark --message-id source-1 --notify-message-id notify-1
Title: Make channel messages spawn GitHub-native idea labs
Context:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable shaping surface.`,
		},
	}
	req, err := BuildChannelIdeaActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelIdeaActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "brainstorm" || req.Options.Route != "team-demo" || req.Options.IdeaID != "roadmap-spark" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel idea parsing: %#v", req)
	}
	if req.Options.Title != "Make channel messages spawn GitHub-native idea labs" || !strings.Contains(req.Options.Notes, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoIdeaID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route idea hashes: %#v", req)
	}
}
