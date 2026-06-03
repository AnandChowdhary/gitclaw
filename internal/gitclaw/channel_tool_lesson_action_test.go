package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolLessonCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-tool-lesson-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 486,
			"title": "GitClaw telegram thread chat-tool-lesson-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-lesson-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48601,
			"body": "@gitclaw /channels tool-lesson --note-id note-1 --tool gitclaw.search_files --message-id inbound-486 --notify-message-id notify-486\nTitle: Prefer search_files for precise repo recall\nLesson:\nVisible tool lesson with CHANNEL_TOOL_LESSON_SECRET.",
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
			Number: 486,
			Title:  "GitClaw telegram thread chat-tool-lesson-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{486: {{
			ID: 48600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-tool-lesson-123",
				MessageID: "inbound-486",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_TOOL_LESSON_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{486: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool lesson action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one tool lesson issue: %#v", len(github.Issues), github.Issues)
	}
	note := github.Issues[1]
	if !HasChannelToolLessonMarker(note.Body) || !strings.Contains(note.Body, `note_id="note-1"`) {
		t.Fatalf("tool lesson issue missing channel-tool-lesson marker:\n%s", note.Body)
	}
	for _, want := range []string{
		"GitClaw channel tool lesson",
		"note_id: note-1",
		"source_channel: telegram",
		"source_issue: #486",
		"source_message_id_sha256_12:",
		"tool_lesson_mode: github-issue-tool-lesson",
		"tool_execution_performed: false",
		"tool_install_performed: false",
		"tool_policy_mutation_performed: false",
		"memory_mutation_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"gitclaw.search_files",
		"Prefer search_files for precise repo recall",
		"Visible tool lesson with CHANNEL_TOOL_LESSON_SECRET.",
	} {
		if !strings.Contains(note.Body, want) {
			t.Fatalf("tool lesson issue missing %q:\n%s", want, note.Body)
		}
	}
	if strings.Contains(note.Body, "chat-tool-lesson-123") || strings.Contains(note.Body, "inbound-486") || strings.Contains(note.Body, "CHANNEL_TOOL_LESSON_INGEST_SECRET") {
		t.Fatalf("tool lesson issue leaked provider IDs or channel body:\n%s", note.Body)
	}
	if !hasLabel(github.IssueLabels[note.Number], "gitclaw") {
		t.Fatalf("tool lesson issue missing gitclaw trigger label: %#v", github.IssueLabels[note.Number])
	}

	sourceComments := github.CommentsByIssue[486]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-486"`,
		"GitClaw channel tool lesson captured.",
		"Tool lesson: #101",
		"https://github.com/owner/repo/issues/101",
		"Tool: gitclaw.search_files",
		"Title: Prefer search_files for precise repo recall",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("tool lesson notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_TOOL_LESSON_SECRET") || strings.Contains(outbound, "CHANNEL_TOOL_LESSON_INGEST_SECRET") {
		t.Fatalf("tool lesson notification leaked lesson or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Lesson Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels tool-lesson`",
		"channel_tool_lesson_status: `captured`",
		"tool_lesson_issue: `#101`",
		"tool_lesson_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#486`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"tool_name_auto: `false`",
		"raw_tool_lesson_id_included: `false`",
		"raw_tool_name_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_tool_lesson_title_included: `false`",
		"raw_tool_lesson_text_included: `false`",
		"raw_channel_message_body_included: `false`",
		"tool_execution_performed: `false`",
		"tool_install_performed: `false`",
		"tool_policy_mutation_performed: `false`",
		"memory_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_tool_lesson_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool lesson receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_LESSON_INGEST_SECRET", "CHANNEL_TOOL_LESSON_SECRET", "Prefer search_files for precise repo recall", "gitclaw.search_files", "note-1", "chat-tool-lesson-123", "inbound-486", "notify-486"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool lesson receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 486,
			"title": "GitClaw telegram thread chat-tool-lesson-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-lesson-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48602,
			"body": "@gitclaw /channels tool-lesson --note-id note-1 --tool gitclaw.search_files --message-id inbound-486 --notify-message-id notify-486\nTitle: Prefer search_files for precise repo recall\nLesson:\nDo not leak duplicate token CHANNEL_TOOL_LESSON_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate tool lesson created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[486]); got != 4 {
		t.Fatalf("duplicate tool lesson posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[486])
	}
	duplicateReceipt := github.CommentsByIssue[486][3].Body
	for _, want := range []string{
		"channel_tool_lesson_status: `duplicate`",
		"tool_lesson_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool lesson receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_TOOL_LESSON_DUPLICATE_SECRET") {
		t.Fatalf("duplicate tool lesson receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelToolLessonActionRequestParsesRouteAliasAndBody(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 33, Title: "Channel tool lesson"},
		Comment: &Comment{
			ID: 3301,
			Body: `@gitclaw /channel tool-guidance --route team-demo --lesson-id Roadmap.ToolLesson --message-id source-1 --notify-message-id notify-1
Tool: gitclaw.search_files
Title: Prefer repo-scoped tools for channel lessons
Lesson:
- Preserve the idea in GitHub first.
- Keep tool installation as a reviewed follow-up.`,
		},
	}
	req, err := BuildChannelToolLessonActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelToolLessonActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-guidance" || req.Options.Route != "team-demo" || req.Options.NoteID != "roadmap-toollesson" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel tool lesson parsing: %#v", req)
	}
	if req.Options.ToolName != "gitclaw.search_files" || req.Options.Title != "Prefer repo-scoped tools for channel lessons" || !strings.Contains(req.Options.Lesson, "Preserve the idea in GitHub first") {
		t.Fatalf("unexpected tool/title/lesson: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNoteID || req.AutoToolName || req.AutoNotifyMessageID || req.ToolNameSHA == "" || req.TitleSHA == "" || req.LessonSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route tool lesson hashes: %#v", req)
	}
}
