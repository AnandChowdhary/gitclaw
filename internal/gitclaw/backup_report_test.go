package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderBackupSummaryIssueCommandRequiresLLME2EWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 96,
			Title:  "@gitclaw /backup",
			Body:   "BACKUP_REPORT_BODY_SECRET",
		},
	}
	comments := []Comment{{
		ID:   21,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_REPORT_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_REPORT_TRANSCRIPT_SECRET"},
		{Role: "assistant", Body: "BACKUP_REPORT_ASSISTANT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Report",
		"requested_backup_command: `summary`",
		"backup_command_status: `ok`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
		"requested_local_command: `gitclaw backup verify --root .gitclaw/backups --repo owner/repo`",
		"llm_e2e_required_after_backup_report_change: `true`",
		"run `gitclaw backup verify --root .gitclaw/backups --repo owner/repo` after fetching `gitclaw-backups`",
		".gitclaw/backups/owner__repo/issues/000096.json",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup summary report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_REPORT_BODY_SECRET", "BACKUP_REPORT_COMMENT_SECRET", "BACKUP_REPORT_TRANSCRIPT_SECRET", "BACKUP_REPORT_ASSISTANT_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup summary report leaked %q:\n%s", leaked, report)
		}
	}
}

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
		"llm_e2e_required_after_backup_verify_change: `true`",
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

func TestRenderBackupCatalogIssueCommandListsBodyFreeRecoverySurface(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 96,
			Title:  "@gitclaw /backup catalog",
			Body:   "BACKUP_CATALOG_BODY_SECRET",
		},
	}
	comments := []Comment{{
		ID:   21,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_CATALOG_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_CATALOG_TRANSCRIPT_SECRET"},
		{Role: "assistant", Body: "BACKUP_CATALOG_ASSISTANT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Catalog Report",
		"requested_backup_command: `catalog`",
		"backup_command_status: `ok`",
		"backup_catalog_status: `ok`",
		"catalog_strategy: `compact-git-backed-recovery-discovery`",
		"backup_model: `github-issues-plus-gitclaw-backups-branch`",
		"catalog_scope: `backup-commands-and-recovery-gates`",
		"catalog_entries: `18`",
		"fetched_branch_required_commands: `17`",
		"requested_local_command: `gitclaw backup catalog --repo owner/repo`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
		"raw_backup_payloads_included: `false`",
		"repository_mutation_allowed: `false`",
		"restore_mutation_allowed: `false`",
		"retention_mutation_allowed: `false`",
		"llm_e2e_required_after_backup_catalog_change: `true`",
		".gitclaw/backups/owner__repo/issues/000096.json",
		"command=`catalog` issue_intent=`@gitclaw /backup catalog` local_command=`gitclaw backup catalog` execution=`metadata-only` gate=`body-free-output` raw_bodies_included=`false` mutation_allowed=`false`",
		"command=`search` issue_intent=`@gitclaw /backup search <query>` local_command=`gitclaw backup search --root .gitclaw/backups --repo owner/repo <query>`",
		"command=`snapshot` issue_intent=`@gitclaw /backup snapshot` local_command=`gitclaw backup snapshot --root .gitclaw/backups --repo owner/repo` execution=`local-fetched-backup-branch` gate=`composite-lockfile-hash`",
		"backup_branch_gate=`fetched-before-local-inspection`",
		"restore_gate=`plan-only`",
		"search_gate=`query-hash-and-match-metadata`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup catalog report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_CATALOG_BODY_SECRET", "BACKUP_CATALOG_COMMENT_SECRET", "BACKUP_CATALOG_TRANSCRIPT_SECRET", "BACKUP_CATALOG_ASSISTANT_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup catalog report leaked %q:\n%s", leaked, report)
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
		"llm_e2e_required_after_backup_search_change: `true`",
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
		"llm_e2e_required_after_backup_info_change: `true`",
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

func TestRenderBackupCoverageIssueCommandDefaultsToCurrentIssueWithTrailingProse(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 97,
			Title:  "@gitclaw /backup coverage e2e 20260530T223109Z",
			Body:   "BACKUP_COVERAGE_TRAILING_PROSE_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `coverage`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup coverage --root .gitclaw/backups --repo owner/repo --issue 97`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup coverage report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_COVERAGE_TRAILING_PROSE_SECRET") {
		t.Fatalf("backup coverage report leaked body:\n%s", report)
	}
}

func TestRenderBackupStatsIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 98,
			Title:  "@gitclaw /backup stats e2e",
			Body:   "BACKUP_STATS_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `stats`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup stats --root .gitclaw/backups --repo owner/repo`",
		"run `gitclaw backup stats --root .gitclaw/backups --repo owner/repo` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
		"llm_e2e_required_after_backup_stats_change: `true`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup stats report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_STATS_INTENT_SECRET") {
		t.Fatalf("backup stats report leaked body:\n%s", report)
	}
}

func TestRenderBackupSnapshotIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 98,
			Title:  "@gitclaw /backup snapshot e2e",
			Body:   "BACKUP_SNAPSHOT_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `snapshot`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup snapshot --root .gitclaw/backups --repo owner/repo`",
		"run `gitclaw backup snapshot --root .gitclaw/backups --repo owner/repo` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"backup_snapshot_status: `deferred`",
		"backup_snapshot_execution: `local_fetched_backup_branch`",
		"backup_snapshot_gate: `verify + composite lockfile hash`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"raw_issue_titles_included_issue_side: `false`",
		"repository_mutation_allowed_issue_side: `false`",
		"github_api_calls_performed_issue_side: `false`",
		"raw_bodies_included: `false`",
		"llm_e2e_required_after_backup_snapshot_change: `true`",
		"`gitclaw backup snapshot --root .gitclaw/backups --repo <owner/repo>`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup snapshot report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_SNAPSHOT_INTENT_SECRET") || strings.Contains(report, "@gitclaw /backup snapshot e2e") {
		t.Fatalf("backup snapshot report leaked request text:\n%s", report)
	}
}

func TestRenderBackupFreshnessIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 98,
			Title:  "@gitclaw /backup freshness e2e",
			Body:   "BACKUP_FRESHNESS_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `freshness`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup freshness --root .gitclaw/backups --repo owner/repo --max-age-hours 24`",
		"run `gitclaw backup freshness --root .gitclaw/backups --repo owner/repo --max-age-hours 24` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"backup_freshness_status: `deferred`",
		"backup_freshness_execution: `local_fetched_backup_branch`",
		"backup_freshness_gate: `latest-backup-age <= max-age`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"raw_bodies_included: `false`",
		"llm_e2e_required_after_backup_freshness_change: `true`",
		"`gitclaw backup freshness --root .gitclaw/backups --repo <owner/repo> --max-age-hours 24`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup freshness report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_FRESHNESS_INTENT_SECRET") || strings.Contains(report, "@gitclaw /backup freshness e2e") {
		t.Fatalf("backup freshness report leaked request text:\n%s", report)
	}
}

func TestRenderBackupContinuityIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 98,
			Title:  "@gitclaw /backup continuity e2e",
			Body:   "BACKUP_CONTINUITY_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `continuity`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup continuity --root .gitclaw/backups --repo owner/repo --max-gap-hours 168`",
		"run `gitclaw backup continuity --root .gitclaw/backups --repo owner/repo --max-gap-hours 168` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"backup_continuity_status: `deferred`",
		"backup_continuity_execution: `local_fetched_backup_branch`",
		"backup_continuity_gate: `longest-backup-gap <= max-gap`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"raw_bodies_included: `false`",
		"llm_e2e_required_after_backup_continuity_change: `true`",
		"`gitclaw backup continuity --root .gitclaw/backups --repo <owner/repo> --max-gap-hours 168`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup continuity report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_CONTINUITY_INTENT_SECRET") || strings.Contains(report, "@gitclaw /backup continuity e2e") {
		t.Fatalf("backup continuity report leaked request text:\n%s", report)
	}
}

func TestRenderBackupListIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 99,
			Title:  "@gitclaw /backup list e2e",
			Body:   "BACKUP_LIST_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `list`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup list --root .gitclaw/backups --repo owner/repo --limit 20`",
		"run `gitclaw backup list --root .gitclaw/backups --repo owner/repo --limit 20` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
		"llm_e2e_required_after_backup_list_change: `true`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup list report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_LIST_INTENT_SECRET") {
		t.Fatalf("backup list report leaked body:\n%s", report)
	}
}

func TestRenderBackupManifestIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 100,
			Title:  "@gitclaw /backup manifest e2e",
			Body:   "BACKUP_MANIFEST_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `manifest`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup manifest --root .gitclaw/backups --repo owner/repo --issue 100`",
		"run `gitclaw backup manifest --root .gitclaw/backups --repo owner/repo --issue 100` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
		"llm_e2e_required_after_backup_manifest_change: `true`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup manifest report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_MANIFEST_INTENT_SECRET") {
		t.Fatalf("backup manifest report leaked body:\n%s", report)
	}
}

func TestRenderBackupRestorePlanIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 97,
			Title:  "@gitclaw /backup restore-plan e2e",
			Body:   "BACKUP_RESTORE_PLAN_INTENT_SECRET",
		},
	}
	comments := []Comment{{
		ID:   24,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_RESTORE_PLAN_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_RESTORE_PLAN_TRANSCRIPT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Report",
		"requested_backup_command: `restore-plan`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup restore-plan --root .gitclaw/backups --repo owner/repo --issue 97`",
		"run `gitclaw backup restore-plan --root .gitclaw/backups --repo owner/repo --issue 97` after fetching `gitclaw-backups`",
		"backup_restore_plan_status: `deferred`",
		"backup_restore_plan_execution: `local_fetched_backup_branch`",
		"backup_restore_plan_mode: `dry-run`",
		"backup_restore_plan_gates: `verify, body-free-output, explicit-future-approval`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"repository_mutation_allowed_issue_side: `false`",
		"github_api_calls_performed_issue_side: `false`",
		"llm_e2e_required_after_backup_restore_plan_change: `true`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup restore-plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_RESTORE_PLAN_INTENT_SECRET", "BACKUP_RESTORE_PLAN_COMMENT_SECRET", "BACKUP_RESTORE_PLAN_TRANSCRIPT_SECRET", "@gitclaw /backup restore-plan e2e"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup restore-plan report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderBackupRetentionPlanIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 97,
			Title:  "@gitclaw /backup retention-plan e2e",
			Body:   "BACKUP_RETENTION_PLAN_INTENT_SECRET",
		},
	}
	comments := []Comment{{
		ID:   25,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_RETENTION_PLAN_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_RETENTION_PLAN_TRANSCRIPT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Report",
		"requested_backup_command: `retention-plan`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup retention-plan --root .gitclaw/backups --repo owner/repo --keep-latest 50`",
		"run `gitclaw backup retention-plan --root .gitclaw/backups --repo owner/repo --keep-latest 50` after fetching `gitclaw-backups`",
		"backup_retention_plan_status: `deferred`",
		"backup_retention_plan_execution: `local_fetched_backup_branch`",
		"backup_retention_plan_mode: `dry-run`",
		"backup_retention_plan_gates: `verify, keep-latest-plan, body-free-output, explicit-future-approval`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"repository_mutation_allowed_issue_side: `false`",
		"branch_deletion_allowed_issue_side: `false`",
		"github_api_calls_performed_issue_side: `false`",
		"llm_e2e_required_after_backup_retention_plan_change: `true`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup retention-plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_RETENTION_PLAN_INTENT_SECRET", "BACKUP_RETENTION_PLAN_COMMENT_SECRET", "BACKUP_RETENTION_PLAN_TRANSCRIPT_SECRET", "@gitclaw /backup retention-plan e2e"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup retention-plan report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderBackupTimelineIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 97,
			Title:  "@gitclaw /backup timeline e2e",
			Body:   "BACKUP_TIMELINE_INTENT_SECRET",
		},
	}

	report := RenderBackupReport(ev, DefaultConfig(), nil, nil)
	for _, want := range []string{
		"requested_backup_command: `timeline`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup timeline --root .gitclaw/backups --repo owner/repo --limit 20`",
		"run `gitclaw backup timeline --root .gitclaw/backups --repo owner/repo --limit 20` after fetching `gitclaw-backups`",
		"issue_side_execution: `deferred_to_post_turn_backup_branch`",
		"raw_bodies_included: `false`",
		"`gitclaw backup timeline --root .gitclaw/backups --repo <owner/repo> --limit 20`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup timeline report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "BACKUP_TIMELINE_INTENT_SECRET") || strings.Contains(report, "@gitclaw /backup timeline e2e") {
		t.Fatalf("backup timeline report leaked request text:\n%s", report)
	}
}

func TestRenderBackupExportJSONLIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 101,
			Title:  "@gitclaw /backup export-jsonl",
			Body:   "BACKUP_EXPORT_JSONL_ISSUE_BODY_SECRET",
		},
	}
	comments := []Comment{{
		ID:   24,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_EXPORT_JSONL_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_EXPORT_JSONL_TRANSCRIPT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Report",
		"requested_backup_command: `export-jsonl`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup export-jsonl --root .gitclaw/backups --repo owner/repo --issue 101`",
		"run `gitclaw backup export-jsonl --root .gitclaw/backups --repo owner/repo --issue 101` after fetching `gitclaw-backups`",
		"backup_export_jsonl_status: `deferred`",
		"backup_export_jsonl_execution: `local_fetched_backup_branch`",
		"backup_export_jsonl_mode: `explicit_raw_recovery_path`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"raw_jsonl_included_issue_side: `false`",
		"llm_e2e_required_after_backup_export_jsonl_change: `true`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup export-jsonl report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_EXPORT_JSONL_ISSUE_BODY_SECRET", "BACKUP_EXPORT_JSONL_COMMENT_SECRET", "BACKUP_EXPORT_JSONL_TRANSCRIPT_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup export-jsonl report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderBackupProvenanceIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 97,
			Title:  "@gitclaw /backup provenance e2e",
			Body:   "BACKUP_PROVENANCE_INTENT_SECRET",
		},
	}
	comments := []Comment{{
		ID:   23,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_PROVENANCE_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_PROVENANCE_TRANSCRIPT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Report",
		"requested_backup_command: `provenance`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup provenance --root .gitclaw/backups --repo owner/repo`",
		"run `gitclaw backup provenance --root .gitclaw/backups --repo owner/repo` after fetching `gitclaw-backups`",
		"backup_provenance_status: `deferred`",
		"backup_provenance_execution: `local_fetched_backup_branch`",
		"backup_provenance_gates: `verify, git-history, body-free-output`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_backup_provenance_change: `true`",
		"raw_bodies_included: `false`",
		"`gitclaw backup provenance --root .gitclaw/backups --repo <owner/repo>`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup provenance report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_PROVENANCE_INTENT_SECRET", "BACKUP_PROVENANCE_COMMENT_SECRET", "BACKUP_PROVENANCE_TRANSCRIPT_SECRET", "@gitclaw /backup provenance e2e"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup provenance report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderBackupDrillIssueCommandRecordsDeferredIntentWithoutBodies(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 99,
			Title:  "@gitclaw /backup drill",
			Body:   "BACKUP_DRILL_ISSUE_BODY_SECRET",
		},
	}
	comments := []Comment{{
		ID:   22,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nBACKUP_DRILL_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "BACKUP_DRILL_TRANSCRIPT_SECRET"},
	}

	report := RenderBackupReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Backup Report",
		"requested_backup_command: `drill`",
		"backup_command_status: `ok`",
		"requested_local_command: `gitclaw backup drill --root .gitclaw/backups --repo owner/repo --issue 99`",
		"run `gitclaw backup drill --root .gitclaw/backups --repo owner/repo --issue 99` after fetching `gitclaw-backups`",
		"backup_drill_status: `deferred`",
		"backup_drill_execution: `local_fetched_backup_branch`",
		"backup_drill_gates: `verify, coverage, restore-plan`",
		"raw_backup_payloads_scanned_issue_side: `false`",
		"llm_e2e_required_after_backup_drill_change: `true`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup drill report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_DRILL_ISSUE_BODY_SECRET", "BACKUP_DRILL_COMMENT_SECRET", "BACKUP_DRILL_TRANSCRIPT_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup drill report leaked %q:\n%s", leaked, report)
		}
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
