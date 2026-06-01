package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelTaskCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-task-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 262,
			"title": "GitClaw telegram thread chat-task-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-task-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26201,
			"body": "@gitclaw /channels task --task-id task-1 --message-id inbound-262 --notify-message-id notify-262\nTitle: Follow up on channel incident\nNotes:\nVisible task notes with CHANNEL_TASK_NOTES_SECRET.",
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
			Title:  "GitClaw telegram thread chat-task-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{262: {{
			ID: 26200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-task-123",
				MessageID: "inbound-262",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_TASK_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{262: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel task action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one task issue: %#v", len(github.Issues), github.Issues)
	}
	task := github.Issues[1]
	if !HasChannelTaskMarker(task.Body) || !strings.Contains(task.Body, `task_id="task-1"`) {
		t.Fatalf("task issue missing channel-task marker:\n%s", task.Body)
	}
	for _, want := range []string{
		"GitClaw channel task",
		"task_id: task-1",
		"source_channel: telegram",
		"source_issue: #262",
		"source_message_id_sha256_12:",
		"task_mode: github-issue-task",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Follow up on channel incident",
		"Visible task notes with CHANNEL_TASK_NOTES_SECRET.",
	} {
		if !strings.Contains(task.Body, want) {
			t.Fatalf("task issue missing %q:\n%s", want, task.Body)
		}
	}
	if strings.Contains(task.Body, "chat-task-123") || strings.Contains(task.Body, "inbound-262") || strings.Contains(task.Body, "CHANNEL_TASK_INGEST_SECRET") {
		t.Fatalf("task issue leaked provider IDs or channel body:\n%s", task.Body)
	}
	if !hasLabel(github.IssueLabels[task.Number], "gitclaw") {
		t.Fatalf("task issue missing gitclaw trigger label: %#v", github.IssueLabels[task.Number])
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
		"GitClaw channel task created.",
		"Task: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Follow up on channel incident",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("task notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_TASK_NOTES_SECRET") || strings.Contains(outbound, "CHANNEL_TASK_INGEST_SECRET") {
		t.Fatalf("task notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Task Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels task`",
		"channel_task_status: `created`",
		"task_issue: `#101`",
		"task_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#262`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_task_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_task_title_included: `false`",
		"raw_task_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_task_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel task receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TASK_INGEST_SECRET", "CHANNEL_TASK_NOTES_SECRET", "Follow up on channel incident", "task-1", "chat-task-123", "inbound-262", "notify-262"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel task receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 262,
			"title": "GitClaw telegram thread chat-task-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-task-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26202,
			"body": "@gitclaw /channels task --task-id task-1 --message-id inbound-262 --notify-message-id notify-262\nTitle: Follow up on channel incident\nNotes:\nDo not leak duplicate token CHANNEL_TASK_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate task created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[262]); got != 4 {
		t.Fatalf("duplicate task posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[262])
	}
	duplicateReceipt := github.CommentsByIssue[262][3].Body
	for _, want := range []string{
		"channel_task_status: `duplicate`",
		"task_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate task receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_TASK_DUPLICATE_SECRET") {
		t.Fatalf("duplicate task receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelTaskActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 29, Title: "Channel task"},
		Comment: &Comment{
			ID: 2901,
			Body: `@gitclaw /channel todo --route team-demo --task-id Design.Task --message-id source-1 --notify-message-id notify-1
Title: Route follow-up task
Notes:
Check the provider queue.`,
		},
	}
	req, err := BuildChannelTaskActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelTaskActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "todo" || req.Options.Route != "team-demo" || req.Options.TaskID != "design-task" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel task parsing: %#v", req)
	}
	if req.Options.Title != "Route follow-up task" || !strings.Contains(req.Options.Notes, "Check the provider queue.") {
		t.Fatalf("unexpected task title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoTaskID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route task hashes: %#v", req)
	}
}
