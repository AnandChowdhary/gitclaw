package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSoulNoteCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-soul-note-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 486,
			"title": "GitClaw telegram thread chat-soul-note-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-note-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48601,
			"body": "@gitclaw /channels soul-note --note-id note-1 --area operating-principles --message-id inbound-486 --notify-message-id notify-486\nTitle: Prefer GitHub review before SOUL writes\nNote:\nVisible soul note with CHANNEL_SOUL_NOTE_SECRET.",
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
			Title:  "GitClaw telegram thread chat-soul-note-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{486: {{
			ID: 48600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-soul-note-123",
				MessageID: "inbound-486",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_SOUL_NOTE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{486: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soul note action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one soul note issue: %#v", len(github.Issues), github.Issues)
	}
	note := github.Issues[1]
	if !HasChannelSoulNoteMarker(note.Body) || !strings.Contains(note.Body, `note_id="note-1"`) {
		t.Fatalf("soul note issue missing channel-soul-note marker:\n%s", note.Body)
	}
	for _, want := range []string{
		"GitClaw channel soul note",
		"note_id: note-1",
		"source_channel: telegram",
		"source_issue: #486",
		"source_message_id_sha256_12:",
		"soul_note_mode: github-issue-soul-note",
		"soul_mutation_performed: false",
		"memory_mutation_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"operating-principles",
		"Prefer GitHub review before SOUL writes",
		"Visible soul note with CHANNEL_SOUL_NOTE_SECRET.",
	} {
		if !strings.Contains(note.Body, want) {
			t.Fatalf("soul note issue missing %q:\n%s", want, note.Body)
		}
	}
	if strings.Contains(note.Body, "chat-soul-note-123") || strings.Contains(note.Body, "inbound-486") || strings.Contains(note.Body, "CHANNEL_SOUL_NOTE_INGEST_SECRET") {
		t.Fatalf("soul note issue leaked provider IDs or channel body:\n%s", note.Body)
	}
	if !hasLabel(github.IssueLabels[note.Number], "gitclaw") {
		t.Fatalf("soul note issue missing gitclaw trigger label: %#v", github.IssueLabels[note.Number])
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
		"GitClaw channel soul note captured.",
		"Soul note: #101",
		"https://github.com/owner/repo/issues/101",
		"Area: operating-principles",
		"Title: Prefer GitHub review before SOUL writes",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("soul note notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_SOUL_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_SOUL_NOTE_INGEST_SECRET") {
		t.Fatalf("soul note notification leaked note or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soul Note Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels soul-note`",
		"channel_soul_note_status: `captured`",
		"soul_note_issue: `#101`",
		"soul_note_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#486`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"soul_area_auto: `false`",
		"raw_soul_note_id_included: `false`",
		"raw_soul_area_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_soul_note_title_included: `false`",
		"raw_soul_note_text_included: `false`",
		"raw_channel_message_body_included: `false`",
		"soul_mutation_performed: `false`",
		"memory_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_soul_note_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soul note receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_NOTE_INGEST_SECRET", "CHANNEL_SOUL_NOTE_SECRET", "Prefer GitHub review before SOUL writes", "operating-principles", "note-1", "chat-soul-note-123", "inbound-486", "notify-486"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soul note receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 486,
			"title": "GitClaw telegram thread chat-soul-note-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-note-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48602,
			"body": "@gitclaw /channels soul-note --note-id note-1 --area operating-principles --message-id inbound-486 --notify-message-id notify-486\nTitle: Prefer GitHub review before SOUL writes\nNote:\nDo not leak duplicate token CHANNEL_SOUL_NOTE_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate soul note created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[486]); got != 4 {
		t.Fatalf("duplicate soul note posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[486])
	}
	duplicateReceipt := github.CommentsByIssue[486][3].Body
	for _, want := range []string{
		"channel_soul_note_status: `duplicate`",
		"soul_note_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate soul note receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_SOUL_NOTE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate soul note receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelSoulNoteActionRequestParsesRouteAliasAndBodyArea(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 33, Title: "Channel soul note"},
		Comment: &Comment{
			ID: 3301,
			Body: `@gitclaw /channel context-note --route team-demo --note-id Roadmap.SoulNote --message-id source-1 --notify-message-id notify-1
Area: operating-principles
Title: Prefer GitHub review before SOUL writes
Note:
- Preserve the idea in GitHub first.
- Keep soul mutation as a reviewed follow-up.`,
		},
	}
	req, err := BuildChannelSoulNoteActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelSoulNoteActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "context-note" || req.Options.Route != "team-demo" || req.Options.NoteID != "roadmap-soulnote" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel soul note parsing: %#v", req)
	}
	if req.Options.SoulArea != "operating-principles" || req.Options.Title != "Prefer GitHub review before SOUL writes" || !strings.Contains(req.Options.Note, "Preserve the idea in GitHub first") {
		t.Fatalf("unexpected area/title/note: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNoteID || req.AutoSoulArea || req.AutoNotifyMessageID || req.SoulAreaSHA == "" || req.TitleSHA == "" || req.NoteSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route soul note hashes: %#v", req)
	}
}
