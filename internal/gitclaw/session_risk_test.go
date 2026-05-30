package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestSessionRiskCommandReportsBackupRisksWithoutBodies(t *testing.T) {
	root := t.TempDir()
	backup := IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-30T20:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number:            19,
			Title:             "@gitclaw session risk backup",
			Body:              "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"risk\" -->\n<!-- gitclaw:proactive-run name=\"repo-hygiene\" slot=\"2026-05-30T20:00:00Z\" -->\nSESSION_RISK_BACKUP_ISSUE_SECRET",
			Author:            "alice",
			AuthorAssociation: "MEMBER",
			Labels:            []string{"gitclaw", "gitclaw:channel", "gitclaw:proactive"},
		},
		Comments: []IssueBackupComment{
			{
				ID:                11,
				Body:              "<!-- gitclaw:assistant-turn idempotency_key=\"old\" model=\"openai/gpt-4.1-nano\" -->\nSESSION_RISK_ASSISTANT_SECRET",
				Author:            "github-actions[bot]",
				AuthorAssociation: "NONE",
			},
			{
				ID:                12,
				Body:              "<!-- gitclaw:error run_id=\"old\" -->\nSESSION_RISK_ERROR_SECRET",
				Author:            "github-actions[bot]",
				AuthorAssociation: "NONE",
			},
			{
				ID:                13,
				Body:              "<!-- gitclaw:channel-message channel=\"telegram\" message_id=\"abc\" -->\nSESSION_RISK_CHANNEL_SECRET",
				Author:            "github-actions[bot]",
				AuthorAssociation: "NONE",
				CreatedAt:         "2026-05-30T20:00:00Z",
				UpdatedAt:         "2026-05-30T20:01:00Z",
			},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "SESSION_RISK_TRANSCRIPT_ISSUE_SECRET", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
			{Role: "assistant", Body: "SESSION_RISK_TRANSCRIPT_ASSISTANT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 11, Trusted: true},
			{Role: "user", Body: "SESSION_RISK_TRANSCRIPT_CHANNEL_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 13, Trusted: false, Edited: true},
		},
	}
	writeBackupFixture(t, root, backup)
	backupPath := issueBackupPath(root, backup.Repo, backup.Issue.Number)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"session", "risk", "--backup", backupPath}); err != nil {
			t.Fatalf("session risk returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Session Risk Report",
		"Generated without a model call",
		"scope: `local-backup`",
		"backup_file:",
		"backup_repo: `owner/repo`",
		"backup_issue: `#19`",
		"session_risk_status: `warn`",
		"verification_scope: `github_issue_session_provenance`",
		"event_kind: `issue_comment`",
		"raw_comments: `3`",
		"transcript_messages: `3`",
		"user_messages: `2`",
		"assistant_messages: `1`",
		"trusted_messages: `2`",
		"untrusted_messages: `1`",
		"edited_messages: `1`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `0`",
		"assistant_turns_missing_prompt_provenance: `1`",
		"unique_prompt_context_hashes: `0`",
		"heartbeat_comments: `0`",
		"error_marker_comments: `1`",
		"channel_message_comments: `1`",
		"channel_thread_issue: `true`",
		"proactive_run_issue: `true`",
		"session_risk_findings: `9`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `5`",
		"info_risk_findings: `4`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_session_risk_change: `true`",
		"### Transcript Trust Risk Card",
		"kind=`session-transcript` transcript_messages=`3`",
		"### Assistant Provenance Risk Card",
		"kind=`session-provenance` assistant_turn_comments=`1`",
		"### Marker Risk Card",
		"kind=`session-marker` heartbeat_comments=`0` error_marker_comments=`1` channel_message_comments=`1`",
		"code=`assistant_prompt_provenance_missing`",
		"code=`assistant_turn_marker_without_prompt_context_hash`",
		"code=`channel_message_markers_present`",
		"code=`channel_thread_issue`",
		"code=`edited_session_message_present`",
		"code=`error_marker_comment`",
		"code=`error_marker_comments_present`",
		"code=`proactive_run_issue`",
		"code=`untrusted_session_message_visible`",
		"evidence_sha256_12=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("session risk output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"SESSION_RISK_BACKUP_ISSUE_SECRET",
		"SESSION_RISK_ASSISTANT_SECRET",
		"SESSION_RISK_ERROR_SECRET",
		"SESSION_RISK_CHANNEL_SECRET",
		"SESSION_RISK_TRANSCRIPT_ISSUE_SECRET",
		"SESSION_RISK_TRANSCRIPT_ASSISTANT_SECRET",
		"SESSION_RISK_TRANSCRIPT_CHANNEL_SECRET",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("session risk leaked body token %q:\n%s", leaked, output)
		}
	}
}

func TestHandleSessionRiskCommandPostsReportWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 163,
			"title": "GitClaw session risk handler test",
			"body": "Initial body token: SESSION_RISK_HANDLER_ISSUE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 26,
			"body": "@gitclaw /session risk\nHidden comment token: SESSION_RISK_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"created_at": "2026-05-30T20:00:00Z",
			"updated_at": "2026-05-30T20:00:00Z"
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{163: {
		{
			ID:                25,
			Body:              "<!-- gitclaw:assistant-turn idempotency_key=\"old\" model=\"openai/gpt-5-nano\" prompt_context_sha256_12=\"abcdef123456\" context_documents=\"7\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" -->\nAssistant body token: SESSION_RISK_HANDLER_ASSISTANT_SECRET.",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                26,
			Body:              "@gitclaw /session risk\nHidden comment token: SESSION_RISK_HANDLER_BODY_SECRET.",
			User:              User{Login: "alice", Type: "User"},
			AuthorAssociation: "MEMBER",
			CreatedAt:         "2026-05-30T20:00:00Z",
			UpdatedAt:         "2026-05-30T20:00:00Z",
		},
	}}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic session risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Session Risk Report",
		"Generated without a model call",
		"model=\"gitclaw/session\"",
		"session_risk_status: `ok`",
		"raw_comments: `2`",
		"transcript_messages: `3`",
		"user_messages: `2`",
		"assistant_messages: `1`",
		"trusted_messages: `3`",
		"untrusted_messages: `0`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"assistant_turns_missing_prompt_provenance: `0`",
		"unique_prompt_context_hashes: `1`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"session_risk_findings: `0`",
		"llm_e2e_required_after_session_risk_change: `true`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session risk report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_RISK_HANDLER_ISSUE_SECRET", "SESSION_RISK_HANDLER_BODY_SECRET", "SESSION_RISK_HANDLER_ASSISTANT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session risk report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[163], "gitclaw:done") || hasLabel(github.IssueLabels[163], "gitclaw:running") || hasLabel(github.IssueLabels[163], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[163])
	}
}
