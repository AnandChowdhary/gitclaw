package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderBackupVerifyIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 96,
			Title:  "@gitclaw /backup verify",
			Body:   "BACKUP_VERIFY_BODY_SECRET",
		},
	}
	comments := []Comment{{
		ID:   21,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_VERIFY_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_VERIFY_TRANSCRIPT_SECRET"},
		{Role: "assistant", Body: "BACKUP_VERIFY_ASSISTANT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Report",
		"requested_backup_command: `verify`",
		"backup_command_status: `ok`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
		"requested_local_command: `gitclaw backup verify --root .gitclaw/backups --repo owner/repo`",
		"run `gitclaw backup verify --root .gitclaw/backups --repo owner/repo` after fetching `gitclaw-backups`",
		".gitclaw/backups/owner__repo/issues/000096.json",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup verify report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_VERIFY_BODY_SECRET", "BACKUP_VERIFY_COMMENT_SECRET", "BACKUP_VERIFY_TRANSCRIPT_SECRET", "BACKUP_VERIFY_ASSISTANT_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup verify report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderBackupSearchIssueCommandHashesQueryWithoutPrintingIt(t *testing.T) {
	query := "rare-secret search phrase"
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 97,
			Title:  "@gitclaw /backup search " + query,
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `search`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup search --root .gitclaw/backups --repo owner/repo <query>`",
		"query_sha256_12: `" + shortDocumentHash(query) + "`",
		"query_terms: `3`",
		"raw search query is not printed",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup search report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, query) || strings.Contains(report, "rare-secret") {
		t.Fatalf("backup search report leaked raw query:\n%s", report)
	}
}

func TestRenderBackupInfoIssueCommandRecordsTargetIssue(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 97,
			Title:  "@gitclaw /backup info #42",
			Body:   "BACKUP_INFO_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `info`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup info --root .gitclaw/backups --repo owner/repo --issue 42`",
		"run `gitclaw backup info --root .gitclaw/backups --repo owner/repo --issue 42` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup info report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_INFO_INTENT_SECRET") {
		t.Fatalf("backup info report leaked body:\n%s", report)
	}
}

func TestRenderBackupCoverageIssueCommandRecordsTargetIssue(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 97,
			Title:  "@gitclaw /backup coverage #42",
			Body:   "BACKUP_COVERAGE_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `coverage`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup coverage --root .gitclaw/backups --repo owner/repo --issue 42`",
		"run `gitclaw backup coverage --root .gitclaw/backups --repo owner/repo --issue 42` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup coverage report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_COVERAGE_INTENT_SECRET") {
		t.Fatalf("backup coverage report leaked body:\n%s", report)
	}
}

func TestRenderBackupRiskIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 98,
			Title:  "@gitclaw /backup risk",
			Body:   "BACKUP_RISK_ISSUE_BODY_SECRET",
		},
	}
	comments := []Comment{{
		ID:   21,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_RISK_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_RISK_TRANSCRIPT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Report",
		"requested_backup_command: `risk`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup risk --root .gitclaw/backups --repo owner/repo`",
		"run `gitclaw backup risk --root .gitclaw/backups --repo owner/repo` after fetching `gitclaw-backups`",
		"backup_risk_status: `deferred`",
		"backup_risk_execution: `local_fetched_backup_branch`",
		"backup_risk_categories: `integrity, path-safety, credential-handling, prompt-boundary, restore-safety, retention`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"llm_e2e_required_after_backup_risk_change: `true`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup risk report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_RISK_ISSUE_BODY_SECRET", "BACKUP_RISK_COMMENT_SECRET", "BACKUP_RISK_TRANSCRIPT_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup risk report leaked %q:\n%s", leaked, report)
		}
	}
}
