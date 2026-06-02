package gitclaw

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleChannelBackupSearchQueuesRecallWithoutLLM(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(root, defaultBackupRoot)
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number:            7,
			Title:             "@gitclaw backup search title CHANNEL_BACKUP_SEARCH_TITLE_SECRET",
			Body:              "Backup body has retrieval notes and CHANNEL_BACKUP_SEARCH_BODY_SECRET.",
			Author:            "alice",
			AuthorAssociation: "OWNER",
			Labels:            []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "retrieval transcript token CHANNEL_BACKUP_SEARCH_TRANSCRIPT_SECRET", Actor: "alice", AuthorAssociation: "OWNER", Trusted: true},
			{Role: "assistant", Body: "assistant backup search reply CHANNEL_BACKUP_SEARCH_ASSISTANT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nCHANNEL_BACKUP_SEARCH_COMMENT_SECRET", Author: "github-actions[bot]", AuthorAssociation: "NONE"}},
	})
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 8, Title: "@gitclaw unrelated", Body: "OTHER_CHANNEL_BACKUP_SEARCH_SECRET"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "other body"}},
	})
	if _, err := WriteBackupIndex(backupRoot, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-backup-search-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 903,
			"title": "GitClaw telegram thread chat-backup-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90301,
			"body": "@gitclaw /channels backup-search retrieval CHANNEL_BACKUP_SEARCH_QUERY_SECRET --message-id backup-search-inbound-903 --notify-message-id backup-search-notify-903 --search-id Backup.Search.Secret.903 --max-results 2\nDo not include this command hidden token in the receipt: CHANNEL_BACKUP_SEARCH_COMMAND_MARKER.",
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
			Number: 903,
			Title:  "GitClaw telegram thread chat-backup-search-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{903: {{
			ID: 90300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-backup-search-123",
				MessageID: "backup-search-inbound-903",
				Author:    "telegram",
				Body:      "Original mirrored backup search command with CHANNEL_BACKUP_SEARCH_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{903: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup search action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("backup search action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[903]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="backup-search-notify-903"`,
		"GitClaw channel backup search",
		"Backup search status: ok",
		"Backup verify status: ok",
		"Backup branch: gitclaw-backups",
		"Backup fetch status: local",
		"Query hash: ",
		"Query terms: 2",
		"Max results: 2",
		"Issue count: 2",
		"Issue fields searched: 4",
		"Comment bodies searched: 1",
		"Transcript messages searched: 3",
		"Matched issues: 1",
		"Matched lines: 2",
		"Results returned: 2",
		"issue=#7 path=issues/000007.json",
		"source=issue.body",
		"source=transcript:01",
		"line_sha256_12=",
		"Raw backup payloads, channel bodies, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw search queries are not included.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Backup branch write: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("backup search notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_SEARCH_TITLE_SECRET", "CHANNEL_BACKUP_SEARCH_BODY_SECRET", "CHANNEL_BACKUP_SEARCH_TRANSCRIPT_SECRET", "CHANNEL_BACKUP_SEARCH_ASSISTANT_SECRET", "CHANNEL_BACKUP_SEARCH_COMMENT_SECRET", "OTHER_CHANNEL_BACKUP_SEARCH_SECRET", "CHANNEL_BACKUP_SEARCH_QUERY_SECRET", "retrieval CHANNEL_BACKUP_SEARCH_QUERY_SECRET", "Backup.Search.Secret.903"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("backup search notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Search Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels backup-search`",
		"channel_backup_search_status: `queued`",
		"backup_search_status: `ok`",
		"backup_verify_status: `ok`",
		"backup_fetch_status: `local`",
		"backup_branch: `gitclaw-backups`",
		"search_mode: `gitclaw-backups-local-lexical`",
		"notification_target_issue: `#903`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"search_id_sha256_12: `",
		"search_id_auto: `false`",
		"query_sha256_12: `",
		"query_terms: `2`",
		"query_bytes: `44`",
		"query_source: `positional`",
		"max_results: `2`",
		"issue_count: `2`",
		"issue_fields_searched: `4`",
		"comment_bodies_searched: `1`",
		"transcript_messages_searched: `3`",
		"matched_issues: `1`",
		"matched_lines: `2`",
		"results_returned: `2`",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_write_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_query_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_search_id_included: `false`",
		"raw_backup_root_included: `false`",
		"raw_backup_paths_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_transcript_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_backup_search_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup search receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_SEARCH_TITLE_SECRET", "CHANNEL_BACKUP_SEARCH_BODY_SECRET", "CHANNEL_BACKUP_SEARCH_TRANSCRIPT_SECRET", "CHANNEL_BACKUP_SEARCH_ASSISTANT_SECRET", "CHANNEL_BACKUP_SEARCH_COMMENT_SECRET", "OTHER_CHANNEL_BACKUP_SEARCH_SECRET", "CHANNEL_BACKUP_SEARCH_QUERY_SECRET", "CHANNEL_BACKUP_SEARCH_INGEST_MARKER", "CHANNEL_BACKUP_SEARCH_COMMAND_MARKER", "chat-backup-search-123", "backup-search-inbound-903", "backup-search-notify-903", "Backup.Search.Secret.903", "owner__repo"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup search receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 903,
			"title": "GitClaw telegram thread chat-backup-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90302,
			"body": "@gitclaw /channels search-backups retrieval CHANNEL_BACKUP_SEARCH_QUERY_SECRET --message-id backup-search-inbound-903 --notify-message-id backup-search-notify-903 --search-id Backup.Search.Secret.903 --max-results 2\nDo not include duplicate hidden token CHANNEL_BACKUP_SEARCH_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[903]); got != 4 {
		t.Fatalf("duplicate backup search posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[903])
	}
	duplicateReceipt := github.CommentsByIssue[903][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels search-backups`",
		"channel_backup_search_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_write_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup search receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_SEARCH_QUERY_SECRET", "CHANNEL_BACKUP_SEARCH_DUPLICATE_MARKER", "chat-backup-search-123", "backup-search-inbound-903", "backup-search-notify-903", "Backup.Search.Secret.903"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup search receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupSearchActionRequestParsesRouteAliasAndTrailingQuery(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel backup search"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel recovery-search --route team-demo --message-id source-1 --notify-message-id notify-1 --id Backup.Search.One --max-results 5
Query: deployment archive`,
		},
	}
	req, err := BuildChannelBackupSearchActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupSearchActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "recovery-search" || req.Options.Route != "team-demo" || req.Options.Query != "deployment archive" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SearchID != "backup-search-one" || req.Options.MaxResults != 5 {
		t.Fatalf("unexpected channel backup search parsing: %#v", req)
	}
	if req.QuerySource != "trailing-query" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSearchID {
		t.Fatalf("unexpected channel backup search defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SearchIDHash == "" || req.QuerySHA == "" {
		t.Fatalf("expected route search hashes: %#v", req)
	}
	if !IsChannelBackupSearchActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel recovery-search alias to be recognized")
	}
}
