package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelBackupNoteCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-backup-note-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 486,
			"title": "GitClaw telegram thread chat-backup-note-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-note-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48601,
			"body": "@gitclaw /channels backup-note --note-id note-1 --scope restore-readiness --message-id inbound-486 --notify-message-id notify-486\nTitle: Prefer GitHub review before backup restores\nNote:\nVisible backup note with CHANNEL_BACKUP_NOTE_SECRET.",
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
			Title:  "GitClaw telegram thread chat-backup-note-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{486: {{
			ID: 48600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-backup-note-123",
				MessageID: "inbound-486",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_BACKUP_NOTE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{486: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup note action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one backup note issue: %#v", len(github.Issues), github.Issues)
	}
	note := github.Issues[1]
	if !HasChannelBackupNoteMarker(note.Body) || !strings.Contains(note.Body, `note_id="note-1"`) {
		t.Fatalf("backup note issue missing channel-backup-note marker:\n%s", note.Body)
	}
	for _, want := range []string{
		"GitClaw channel backup note",
		"note_id: note-1",
		"source_channel: telegram",
		"source_issue: #486",
		"source_message_id_sha256_12:",
		"backup_note_mode: github-issue-backup-note",
		"backup_fetch_performed: false",
		"backup_restore_performed: false",
		"backup_payload_read_performed: false",
		"memory_mutation_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"restore-readiness",
		"Prefer GitHub review before backup restores",
		"Visible backup note with CHANNEL_BACKUP_NOTE_SECRET.",
	} {
		if !strings.Contains(note.Body, want) {
			t.Fatalf("backup note issue missing %q:\n%s", want, note.Body)
		}
	}
	if strings.Contains(note.Body, "chat-backup-note-123") || strings.Contains(note.Body, "inbound-486") || strings.Contains(note.Body, "CHANNEL_BACKUP_NOTE_INGEST_SECRET") {
		t.Fatalf("backup note issue leaked provider IDs or channel body:\n%s", note.Body)
	}
	if !hasLabel(github.IssueLabels[note.Number], "gitclaw") {
		t.Fatalf("backup note issue missing gitclaw trigger label: %#v", github.IssueLabels[note.Number])
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
		"GitClaw channel backup note captured.",
		"Backup note: #101",
		"https://github.com/owner/repo/issues/101",
		"Scope: restore-readiness",
		"Title: Prefer GitHub review before backup restores",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("backup note notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_BACKUP_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_BACKUP_NOTE_INGEST_SECRET") {
		t.Fatalf("backup note notification leaked note or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Note Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels backup-note`",
		"channel_backup_note_status: `captured`",
		"backup_note_issue: `#101`",
		"backup_note_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#486`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"backup_scope_auto: `false`",
		"raw_backup_note_id_included: `false`",
		"raw_backup_scope_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_backup_note_title_included: `false`",
		"raw_backup_note_text_included: `false`",
		"raw_channel_message_body_included: `false`",
		"backup_fetch_performed: `false`",
		"backup_restore_performed: `false`",
		"backup_payload_read_performed: `false`",
		"memory_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_backup_note_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup note receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_NOTE_INGEST_SECRET", "CHANNEL_BACKUP_NOTE_SECRET", "Prefer GitHub review before backup restores", "restore-readiness", "note-1", "chat-backup-note-123", "inbound-486", "notify-486"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup note receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 486,
			"title": "GitClaw telegram thread chat-backup-note-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-note-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48602,
			"body": "@gitclaw /channels backup-note --note-id note-1 --scope restore-readiness --message-id inbound-486 --notify-message-id notify-486\nTitle: Prefer GitHub review before backup restores\nNote:\nDo not leak duplicate token CHANNEL_BACKUP_NOTE_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate backup note created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[486]); got != 4 {
		t.Fatalf("duplicate backup note posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[486])
	}
	duplicateReceipt := github.CommentsByIssue[486][3].Body
	for _, want := range []string{
		"channel_backup_note_status: `duplicate`",
		"backup_note_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup note receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_BACKUP_NOTE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate backup note receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelBackupNoteActionRequestParsesRouteAliasAndBodyScope(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 33, Title: "Channel backup note"},
		Comment: &Comment{
			ID: 3301,
			Body: `@gitclaw /channel recovery-note --route team-demo --note-id Roadmap.BackupNote --message-id source-1 --notify-message-id notify-1
Backup scope: restore-readiness
Title: Prefer GitHub review before backup restores
Note:
- Preserve the idea in GitHub first.
- Keep backup restore as a reviewed follow-up.`,
		},
	}
	req, err := BuildChannelBackupNoteActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupNoteActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "recovery-note" || req.Options.Route != "team-demo" || req.Options.NoteID != "roadmap-backupnote" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel backup note parsing: %#v", req)
	}
	if req.Options.BackupScope != "restore-readiness" || req.Options.Title != "Prefer GitHub review before backup restores" || !strings.Contains(req.Options.Note, "Preserve the idea in GitHub first") {
		t.Fatalf("unexpected scope/title/note: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNoteID || req.AutoBackupScope || req.AutoNotifyMessageID || req.BackupScopeSHA == "" || req.TitleSHA == "" || req.NoteSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route backup note hashes: %#v", req)
	}
}
