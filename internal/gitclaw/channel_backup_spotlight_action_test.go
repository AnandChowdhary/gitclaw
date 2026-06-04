package gitclaw

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleChannelBackupSpotlightQueuesDeterministicCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(root, defaultBackupRoot)
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number:            7,
			Title:             "@gitclaw backup spotlight title CHANNEL_BACKUP_SPOTLIGHT_TITLE_SECRET",
			Body:              "Backup body has rescuecode and CHANNEL_BACKUP_SPOTLIGHT_BODY_SECRET.",
			Author:            "alice",
			AuthorAssociation: "OWNER",
			Labels:            []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "retrieval transcript token CHANNEL_BACKUP_SPOTLIGHT_TRANSCRIPT_SECRET", Actor: "alice", AuthorAssociation: "OWNER", Trusted: true},
			{Role: "assistant", Body: "assistant backup spotlight reply CHANNEL_BACKUP_SPOTLIGHT_ASSISTANT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nCHANNEL_BACKUP_SPOTLIGHT_COMMENT_SECRET", Author: "github-actions[bot]", AuthorAssociation: "NONE"}},
	})
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 8, Title: "@gitclaw unrelated", Body: "OTHER_CHANNEL_BACKUP_SPOTLIGHT_SECRET"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "other body"}},
	})
	if _, err := WriteBackupIndex(backupRoot, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-backup-spotlight-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 914,
			"title": "GitClaw telegram thread chat-backup-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91401,
			"body": "@gitclaw /channels backup-spotlight rescuecode CHANNEL_BACKUP_SPOTLIGHT_QUERY_SECRET --message-id backup-spotlight-inbound-914 --notify-message-id backup-spotlight-notify-914 --spotlight-id Backup.Spotlight.Secret.914\nDo not include this command hidden token in the receipt: CHANNEL_BACKUP_SPOTLIGHT_COMMAND_MARKER.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 914,
			Title:  "GitClaw telegram thread chat-backup-spotlight-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{914: {{
			ID: 91400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-backup-spotlight-123",
				MessageID: "backup-spotlight-inbound-914",
				Author:    "telegram",
				Body:      "Original mirrored backup spotlight command with CHANNEL_BACKUP_SPOTLIGHT_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{914: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup spotlight action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("backup spotlight action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[914]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="backup-spotlight-notify-914"`,
		"GitClaw channel backup spotlight",
		"Spotlight status: ok",
		"Backup verify status: ok",
		"Backup branch: gitclaw-backups",
		"Backup fetch status: local",
		"Focus hash: ",
		"Focus terms: 2",
		"Backup schema version: 1",
		"Issue count: 2",
		"Search status: ok",
		"Matched issues: 1",
		"Matched lines: 1",
		"Candidate backups: 1",
		"Selected index: 0",
		"Selection seed hash: ",
		"Selection hash: ",
		"Backup spotlight id hash: ",
		"Spotlight:",
		"issue=#7 path=issues/000007.json",
		"source=issue.body",
		"source_kind=search",
		"role=user",
		"trusted=true",
		"line=1",
		"matched_terms=1",
		"body_sha256_12=",
		"line_sha256_12=",
		"payload_sha256_12=",
		"comments=1",
		"transcript_messages=2",
		"title_sha256_12=",
		"Try next:",
		"@gitclaw /channels backup-info #7",
		"@gitclaw /channels backup-search issue.body",
		"Raw backup payloads, backup issue titles, channel bodies, issue bodies, comment bodies, transcript messages, prompts, tool outputs, raw focus text, raw notes, and raw spotlight ids are not included in the source receipt.",
		"Model call: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Backup branch write: not performed by this action.",
		"Restore mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("backup spotlight notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_SPOTLIGHT_TITLE_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_BODY_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_TRANSCRIPT_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_ASSISTANT_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_COMMENT_SECRET", "OTHER_CHANNEL_BACKUP_SPOTLIGHT_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_QUERY_SECRET", "rescuecode CHANNEL_BACKUP_SPOTLIGHT_QUERY_SECRET", "Backup.Spotlight.Secret.914"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("backup spotlight notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Spotlight Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels backup-spotlight`",
		"channel_backup_spotlight_status: `queued`",
		"backup_spotlight_status: `ok`",
		"backup_verify_status: `ok`",
		"backup_fetch_status: `local`",
		"backup_branch: `gitclaw-backups`",
		"spotlight_mode: `gitclaw-backups-deterministic-recovery-draw`",
		"notification_target_issue: `#914`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"backup_spotlight_id_sha256_12: `",
		"backup_spotlight_id_auto: `false`",
		"spotlight_focus_sha256_12: `",
		"spotlight_focus_bytes: `",
		"spotlight_focus_terms: `2`",
		"spotlight_focus_source: `positional`",
		"spotlight_note_sha256_12: `",
		"backup_schema_version: `1`",
		"verification_failures: `0`",
		"issue_count: `2`",
		"search_status: `ok`",
		"matched_issues: `1`",
		"matched_lines: `1`",
		"candidate_backups: `1`",
		"selected_index: `0`",
		"selected_backup_issue_sha256_12: `",
		"selected_backup_path_sha256_12: `",
		"selected_backup_source_sha256_12: `",
		"selected_backup_role_sha256_12: `",
		"selected_backup_line_sha256_12: `",
		"selected_backup_payload_sha256_12: `",
		"selection_seed_sha256_12: `",
		"selection_sha256_12: `",
		"notification_body_sha256_12: `",
		"deterministic_selection: `true`",
		"external_randomness_used: `false`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_read_performed: `true`",
		"backup_branch_write_performed: `false`",
		"restore_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_focus_included: `false`",
		"raw_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_backup_spotlight_id_included: `false`",
		"raw_selection_seed_included: `false`",
		"raw_backup_root_included: `false`",
		"raw_backup_paths_included: `false`",
		"raw_backup_issue_titles_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_transcript_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_backup_spotlight_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup spotlight receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"rescuecode", "CHANNEL_BACKUP_SPOTLIGHT_TITLE_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_BODY_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_TRANSCRIPT_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_ASSISTANT_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_COMMENT_SECRET", "OTHER_CHANNEL_BACKUP_SPOTLIGHT_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_QUERY_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_INGEST_MARKER", "CHANNEL_BACKUP_SPOTLIGHT_COMMAND_MARKER", "chat-backup-spotlight-123", "backup-spotlight-inbound-914", "backup-spotlight-notify-914", "Backup.Spotlight.Secret.914", "issues/000007.json", "owner__repo"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup spotlight receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 914,
			"title": "GitClaw telegram thread chat-backup-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91402,
			"body": "@gitclaw /channels backup-draw rescuecode CHANNEL_BACKUP_SPOTLIGHT_QUERY_SECRET --message-id backup-spotlight-inbound-914 --notify-message-id backup-spotlight-notify-914 --spotlight-id Backup.Spotlight.Secret.914\nDo not leak duplicate token CHANNEL_BACKUP_SPOTLIGHT_DUPLICATE_SECRET.",
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
	if got := len(github.CommentsByIssue[914]); got != 4 {
		t.Fatalf("duplicate backup spotlight posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[914])
	}
	duplicateReceipt := github.CommentsByIssue[914][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels backup-draw`",
		"channel_backup_spotlight_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_write_performed: `false`",
		"restore_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup spotlight receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"rescuecode", "CHANNEL_BACKUP_SPOTLIGHT_QUERY_SECRET", "CHANNEL_BACKUP_SPOTLIGHT_DUPLICATE_SECRET", "chat-backup-spotlight-123", "backup-spotlight-inbound-914", "backup-spotlight-notify-914", "Backup.Spotlight.Secret.914", "issues/000007.json"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup spotlight receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupSpotlightActionRequestParsesRouteAliasAndTrailingNote(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel backup spotlight"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel recovery-draw --route team-demo --message-id source-1 --notify-message-id notify-1 --id Backup.Spotlight.One --focus deployment
Note: pick a deploy recovery backup.`,
		},
	}
	req, err := BuildChannelBackupSpotlightActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupSpotlightActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "recovery-draw" || req.Options.Route != "team-demo" || req.Options.Focus != "deployment" || req.Options.Note != "pick a deploy recovery backup." || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SpotlightID != "backup-spotlight-one" {
		t.Fatalf("unexpected channel backup spotlight parsing: %#v", req)
	}
	if req.FocusSource != "flag" || req.NoteSource != "trailing-note" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSpotlightID {
		t.Fatalf("unexpected channel backup spotlight defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SpotlightIDHash == "" || req.FocusSHA == "" || req.NoteSHA == "" {
		t.Fatalf("expected route spotlight hashes: %#v", req)
	}
	if !IsChannelBackupSpotlightActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel recovery-draw alias to be recognized")
	}
}
