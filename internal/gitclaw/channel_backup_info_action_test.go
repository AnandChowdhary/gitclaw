package gitclaw

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleChannelBackupInfoQueuesFocusedCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(root, defaultBackupRoot)
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw backup info title CHANNEL_BACKUP_INFO_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_INFO_BODY_SECRET",
			Labels: []string{"gitclaw", "CHANNEL_BACKUP_INFO_LABEL_SECRET"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "CHANNEL_BACKUP_INFO_TRANSCRIPT_SECRET", Actor: "alice", Trusted: true},
			{Role: "assistant", Body: "CHANNEL_BACKUP_INFO_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nCHANNEL_BACKUP_INFO_COMMENT_SECRET"},
			{ID: 13, Body: "<!-- gitclaw:error -->\nCHANNEL_BACKUP_INFO_ERROR_SECRET"},
		},
	})
	if _, err := WriteBackupIndex(backupRoot, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-backup-info-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-backup-info-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-info-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90401,
			"body": "@gitclaw /channels backup-info --issue 7 --message-id backup-info-inbound-904 --notify-message-id backup-info-notify-904 --backup-info-id Backup.Info.Secret.904\nDo not include this command hidden token in the receipt: CHANNEL_BACKUP_INFO_COMMAND_MARKER.",
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
			Number: 904,
			Title:  "GitClaw telegram thread chat-backup-info-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{904: {{
			ID: 90400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-backup-info-123",
				MessageID: "backup-info-inbound-904",
				Author:    "telegram",
				Body:      "Original mirrored backup info command with CHANNEL_BACKUP_INFO_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{904: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup info action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("backup info action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[904]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="backup-info-notify-904"`,
		"GitClaw channel backup info",
		"Backup info status: ok",
		"Backup verify status: ok",
		"Backup branch: gitclaw-backups",
		"Backup fetch status: local",
		"Issue: #7",
		"Issue path: issues/000007.json",
		"Backup generated at: 2026-05-29T13:00:00Z",
		"Backup event name: issue_comment",
		"Backup schema version: 1",
		"Verification failures: 0",
		"Payload bytes: ",
		"Payload hash: ",
		"Labels: 2",
		"Label names hash: ",
		"Comments: 2",
		"Transcript messages: 2",
		"User messages: 1",
		"Assistant messages: 1",
		"Assistant turn comments: 1",
		"Error comments: 1",
		"Issue title hash: ",
		"Issue body hash: ",
		"Comment body hashes: 2",
		"Transcript body hashes: 2",
		"Backup info id hash: ",
		"Raw backup payloads, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw backup info ids are not included.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Backup branch write: not performed by this action.",
		"Restore: not performed by this action.",
		"GitHub API replay: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("backup info notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_INFO_TITLE_SECRET", "CHANNEL_BACKUP_INFO_BODY_SECRET", "CHANNEL_BACKUP_INFO_LABEL_SECRET", "CHANNEL_BACKUP_INFO_TRANSCRIPT_SECRET", "CHANNEL_BACKUP_INFO_ASSISTANT_SECRET", "CHANNEL_BACKUP_INFO_COMMENT_SECRET", "CHANNEL_BACKUP_INFO_ERROR_SECRET", "CHANNEL_BACKUP_INFO_INGEST_MARKER", "CHANNEL_BACKUP_INFO_COMMAND_MARKER", "Backup.Info.Secret.904", "@gitclaw backup info title"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("backup info notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Info Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels backup-info`",
		"channel_backup_info_status: `queued`",
		"backup_info_status: `ok`",
		"backup_verify_status: `ok`",
		"backup_fetch_status: `local`",
		"backup_branch: `gitclaw-backups`",
		"info_mode: `gitclaw-backups-single-issue-metadata`",
		"notification_target_issue: `#904`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"backup_info_id_sha256_12: `",
		"backup_info_id_auto: `false`",
		"backup_issue_number_sha256_12: `",
		"backup_issue_source: `flag`",
		"backup_root_sha256_12: `",
		"repo_backup_dir_sha256_12: `",
		"index_path_sha256_12: `",
		"readme_path_sha256_12: `",
		"issue_backup_path_sha256_12: `",
		"backup_schema_version: `1`",
		"backup_event_name: `issue_comment`",
		"verification_failures: `0`",
		"payload_bytes: `",
		"payload_sha256_12: `",
		"labels: `2`",
		"label_names_sha256_12: `",
		"comments: `2`",
		"transcript_messages: `2`",
		"user_messages: `1`",
		"assistant_messages: `1`",
		"assistant_turn_comments: `1`",
		"error_comments: `1`",
		"issue_title_sha256_12: `",
		"issue_body_sha256_12: `",
		"comment_body_hashes_sha256_12: `",
		"transcript_body_hashes_sha256_12: `",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_write_performed: `false`",
		"restore_performed: `false`",
		"github_api_replay_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_backup_issue_number_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_backup_info_id_included: `false`",
		"raw_backup_root_included: `false`",
		"raw_backup_paths_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_label_names_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_titles_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_transcript_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_backup_info_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup info receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"#7", "issues/000007.json", "owner__repo", "CHANNEL_BACKUP_INFO_TITLE_SECRET", "CHANNEL_BACKUP_INFO_BODY_SECRET", "CHANNEL_BACKUP_INFO_LABEL_SECRET", "CHANNEL_BACKUP_INFO_TRANSCRIPT_SECRET", "CHANNEL_BACKUP_INFO_ASSISTANT_SECRET", "CHANNEL_BACKUP_INFO_COMMENT_SECRET", "CHANNEL_BACKUP_INFO_ERROR_SECRET", "CHANNEL_BACKUP_INFO_INGEST_MARKER", "CHANNEL_BACKUP_INFO_COMMAND_MARKER", "chat-backup-info-123", "backup-info-inbound-904", "backup-info-notify-904", "Backup.Info.Secret.904", "@gitclaw backup info title"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup info receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-backup-info-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-info-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90402,
			"body": "@gitclaw /channels describe-backup --issue 7 --message-id backup-info-inbound-904 --notify-message-id backup-info-notify-904 --backup-info-id Backup.Info.Secret.904\nDo not include duplicate hidden token CHANNEL_BACKUP_INFO_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[904]); got != 4 {
		t.Fatalf("duplicate backup info posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[904])
	}
	duplicateReceipt := github.CommentsByIssue[904][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels describe-backup`",
		"channel_backup_info_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_write_performed: `false`",
		"restore_performed: `false`",
		"github_api_replay_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup info receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"#7", "issues/000007.json", "CHANNEL_BACKUP_INFO_DUPLICATE_MARKER", "chat-backup-info-123", "backup-info-inbound-904", "backup-info-notify-904", "Backup.Info.Secret.904"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup info receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupInfoActionRequestParsesRouteAliasAndTrailingIssue(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel backup info"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel archive-info --route team-demo --message-id source-1 --notify-message-id notify-1 --id Backup.Info.One
Issue: #7`,
		},
	}
	req, err := BuildChannelBackupInfoActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupInfoActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "archive-info" || req.Options.Route != "team-demo" || req.Options.IssueNumber != 7 || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.InfoID != "backup-info-one" {
		t.Fatalf("unexpected channel backup info parsing: %#v", req)
	}
	if req.IssueSource != "trailing-issue" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoInfoID {
		t.Fatalf("unexpected channel backup info defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.InfoIDHash == "" || req.IssueNumberHash == "" {
		t.Fatalf("expected route info hashes: %#v", req)
	}
	if !IsChannelBackupInfoActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel archive-info alias to be recognized")
	}
}
