package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHandleChannelBackupRehearsalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-backup-rehearsal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 488,
			"title": "GitClaw telegram thread channel-backup-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-backup-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48801,
			"body": "@gitclaw /channels rehearse-backup --id channel-backup-review --message-id inbound-488 --notify-message-id notify-488\nPlease rehearse this channel-origin backup request.\nCHANNEL_BACKUP_REHEARSAL_SOURCE_SECRET",
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
			Number: 488,
			Title:  "GitClaw telegram thread channel-backup-rehearsal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{488: {{
			ID: 48800,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-backup-rehearsal-thread-123",
				MessageID: "inbound-488",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_BACKUP_REHEARSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{488: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one backup rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsalIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:backup-rehearsal-issue",
		`id="channel-backup-review"`,
		`backup_issue="488"`,
		"GitClaw backup recovery rehearsal issue",
		"rehearsal_id: channel-backup-review",
		"backup_issue: #488",
		"backup_branch: gitclaw-backups",
		"issue_backup_path: .gitclaw/backups/owner__repo/issues/000488.json",
		"source_issue: #488",
		"source_kind: channel_comment",
		"rehearsal_mode: recovery-conversation",
		"restore_mode: dry-run",
		"repository_mutation_allowed: false",
		"backup_branch_write_allowed: false",
		"github_api_replay_allowed: false",
		"raw_source_body_included: false",
		"raw_backup_bodies_included: false",
		"gitclaw backup coverage --root .gitclaw/backups --repo owner/repo --issue 488",
		"gitclaw backup drill --root .gitclaw/backups --repo owner/repo --issue 488",
		"gitclaw backup restore-plan --root .gitclaw/backups --repo owner/repo --issue 488",
	} {
		if !strings.Contains(rehearsalIssue.Body, want) {
			t.Fatalf("backup rehearsal issue missing %q:\n%s", want, rehearsalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_REHEARSAL_SOURCE_SECRET", "CHANNEL_BACKUP_REHEARSAL_INGEST_SECRET", "Please rehearse this channel-origin"} {
		if strings.Contains(rehearsalIssue.Body, leaked) {
			t.Fatalf("backup rehearsal issue leaked %q:\n%s", leaked, rehearsalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[488]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-488"`,
		"GitClaw channel backup rehearsal",
		"Rehearsal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Backup issue: #488",
		"Backup branch: gitclaw-backups",
		"Restore mode: dry-run",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel backup rehearsal notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_BACKUP_REHEARSAL_SOURCE_SECRET") || strings.Contains(outbound, "CHANNEL_BACKUP_REHEARSAL_INGEST_SECRET") {
		t.Fatalf("channel backup rehearsal notification leaked source:\n%s", outbound)
	}
	outboundParts := strings.SplitN(outbound, "\n", 2)
	if len(outboundParts) != 2 {
		t.Fatalf("channel backup rehearsal outbound missing provider body:\n%s", outbound)
	}
	notificationBody := strings.TrimSpace(outboundParts[1])
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Rehearsal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rehearse-backup`",
		"channel_backup_rehearsal_status: `created`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#488`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"backup_issue: `#488`",
		"backup_branch: `gitclaw-backups`",
		"backup_root: `.gitclaw/backups`",
		"notification_body_sha256_12: `" + shortDocumentHash(notificationBody) + "`",
		fmt.Sprintf("notification_body_bytes: `%d`", len(notificationBody)),
		fmt.Sprintf("notification_body_lines: `%d`", lineCount(notificationBody)),
		"rehearsal_mode: `recovery-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"restore_mode: `dry-run`",
		"repository_mutation_allowed: `false`",
		"backup_branch_write_allowed: `false`",
		"github_api_replay_allowed: `false`",
		"raw_rehearsal_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_source_body_included: `false`",
		"raw_backup_bodies_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_backup_rehearsal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_REHEARSAL_SOURCE_SECRET", "CHANNEL_BACKUP_REHEARSAL_INGEST_SECRET", "Please rehearse this channel-origin", "channel-backup-review", "channel-backup-rehearsal-thread-123", "inbound-488", "notify-488", "owner__repo/issues/000488.json"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 488,
			"title": "GitClaw telegram thread channel-backup-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-backup-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48802,
			"body": "@gitclaw /channels rehearse-backup --id channel-backup-review --message-id inbound-488 --notify-message-id notify-488\nDo not leak duplicate token CHANNEL_BACKUP_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel backup rehearsal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[488]); got != 4 {
		t.Fatalf("duplicate channel backup rehearsal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[488])
	}
	duplicateReceipt := github.CommentsByIssue[488][3].Body
	for _, want := range []string{
		"channel_backup_rehearsal_status: `duplicate`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel backup rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_REHEARSAL_DUPLICATE_SECRET", "channel-backup-review", "channel-backup-rehearsal-thread-123", "inbound-488", "notify-488"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel backup rehearsal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupRehearsalActionRequestParsesAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 34, Title: "Channel backup rehearsal"},
		Comment: &Comment{
			ID:   3401,
			Body: `@gitclaw /channel recovery-drill #27 --id Channel.Backup.Rehearsal --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelBackupRehearsalActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupRehearsalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "recovery-drill" || req.Options.Channel != "slack" || req.Options.BackupIssueNumber != 27 || req.Options.RehearsalID != "channel-backup-rehearsal" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel backup rehearsal parsing: %#v", req)
	}
	if req.Rehearsal.BackupIssueNumber != 27 || req.Rehearsal.SourceKind != "channel_comment" || req.Rehearsal.SourceCommentID != 3401 {
		t.Fatalf("unexpected channel backup rehearsal request: %#v", req.Rehearsal)
	}
	if req.Rehearsal.IssueBackupPath != ".gitclaw/backups/owner__repo/issues/000027.json" || !strings.Contains(req.Rehearsal.RestorePlanCmd, "--issue 27") {
		t.Fatalf("unexpected backup paths: %#v", req.Rehearsal)
	}
}
