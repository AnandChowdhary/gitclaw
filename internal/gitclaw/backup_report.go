package gitclaw

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultBackupBranch = "gitclaw-backups"
const defaultBackupRoot = ".gitclaw/backups"

type backupIssueCommand struct {
	Name         string
	Status       string
	LocalCommand string
	QueryHash    string
	QueryTerms   int
}

func IsBackupReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/backup" || command == "/backups"
}

func RenderBackupReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) string {
	request := requestedBackupIssueCommand(ev, cfg)
	relIssuePath := issueBackupPath(defaultBackupRoot, ev.Repo, ev.Issue.Number)
	repoDir := backupRepoDir(defaultBackupRoot, ev.Repo)
	indexPath := filepath.ToSlash(filepath.Join(repoDir, "index.json"))
	readmePath := filepath.ToSlash(filepath.Join(repoDir, "README.md"))
	relIssuePath = filepath.ToSlash(relIssuePath)

	var b strings.Builder
	b.WriteString("## GitClaw Backup Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_backup_command: `%s`\n", request.Name)
	fmt.Fprintf(&b, "- backup_command_status: `%s`\n", request.Status)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", defaultBackupRoot)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", filepath.ToSlash(repoDir))
	fmt.Fprintf(&b, "- issue_backup_path: `%s`\n", relIssuePath)
	fmt.Fprintf(&b, "- index_path: `%s`\n", indexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", readmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", 1)
	fmt.Fprintf(&b, "- raw_comments_now: `%d`\n", len(comments))
	fmt.Fprintf(&b, "- transcript_messages_now: `%d`\n", len(transcript))
	fmt.Fprintf(&b, "- assistant_turn_comments_now: `%d`\n", countSessionMarkers(comments).AssistantTurns)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "deferred_to_post_turn_backup_branch")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	if request.LocalCommand != "" {
		fmt.Fprintf(&b, "- requested_local_command: `%s`\n", backupInlineCommand(request.LocalCommand))
	}
	if request.Name == "verify" {
		fmt.Fprintf(&b, "- llm_e2e_required_after_backup_verify_change: `%t`\n", true)
	}
	if request.Name == "search" {
		fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", request.QueryHash)
		fmt.Fprintf(&b, "- query_terms: `%d`\n", request.QueryTerms)
		fmt.Fprintf(&b, "- llm_e2e_required_after_backup_search_change: `%t`\n", true)
	}
	if request.Name == "risk" {
		writeBackupIssueRiskSummary(&b)
	}
	if request.Name == "provenance" {
		writeBackupIssueProvenanceSummary(&b)
	}
	if request.Name == "drill" {
		writeBackupIssueDrillSummary(&b)
	}
	if request.Name == "restore-plan" {
		writeBackupIssueRestorePlanSummary(&b)
	}
	if request.Name == "retention-plan" {
		writeBackupIssueRetentionPlanSummary(&b)
	}
	if request.Name == "export-jsonl" {
		writeBackupIssueExportJSONLSummary(&b)
	}
	b.WriteByte('\n')

	b.WriteString("The backup job runs after a successful assistant turn and writes the raw transcript backup plus repo-scoped index to the dedicated backup branch.\n\n")
	b.WriteString("Issue and comment bodies are not included in this report; the raw backup JSON is the canonical transcript copy. Requested backup subcommands are recorded here as issue-visible intent, then run locally against a fetched `gitclaw-backups` branch.\n\n")
	b.WriteString("### Backup Contents\n")
	b.WriteString("- issue metadata\n")
	b.WriteString("- raw issue comments\n")
	b.WriteString("- reconstructed transcript with GitClaw assistant markers stripped\n")
	b.WriteString("- generation timestamp\n")
	b.WriteString("- schema version\n")
	b.WriteString("\n### Requested Command\n")
	writeBackupIssueCommandSummary(&b, request)
	b.WriteString("\n### Verification\n")
	b.WriteString("- `gitclaw backup verify --root .gitclaw/backups --repo <owner/repo>`\n")
	b.WriteString("- `gitclaw backup coverage --root .gitclaw/backups --repo <owner/repo> --issue <number>`\n")
	b.WriteString("- `gitclaw backup drill --root .gitclaw/backups --repo <owner/repo> --issue <number>`\n")
	b.WriteString("- `gitclaw backup risk --root .gitclaw/backups --repo <owner/repo>`\n")
	b.WriteString("- `gitclaw backup provenance --root .gitclaw/backups --repo <owner/repo>`\n")
	b.WriteString("- `gitclaw backup timeline --root .gitclaw/backups --repo <owner/repo> --limit 20`\n")
	b.WriteString("- `gitclaw backup search --root .gitclaw/backups --repo <owner/repo> <query>`\n")
	b.WriteString("- `gitclaw backup retention-plan --root .gitclaw/backups --repo <owner/repo> --keep-latest 50`\n")
	b.WriteString("- validates the repo-scoped index, README, canonical issue paths, JSON schema version, counts, timestamps, traversal-safe payload paths, and git provenance; search reports hashes and metadata without printing raw backup bodies\n")

	return strings.TrimSpace(b.String())
}

func requestedBackupIssueCommand(ev Event, cfg Config) backupIssueCommand {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return backupIssueCommand{
			Name:         "summary",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup verify --root %s --repo %s", defaultBackupRoot, backupReportRepo(ev.Repo)),
		}
	}
	name := cleanBackupCommandName(fields[1])
	switch name {
	case "verify":
		return backupIssueCommand{
			Name:         "verify",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup verify --root %s --repo %s", defaultBackupRoot, backupReportRepo(ev.Repo)),
		}
	case "coverage", "covered", "check":
		issueNumber := ev.Issue.Number
		if len(fields) >= 3 {
			parsed, ok := parseBackupIssueNumber(fields[2])
			if ok {
				issueNumber = parsed
			}
		}
		return backupIssueCommand{
			Name:         "coverage",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup coverage --root %s --repo %s --issue %d", defaultBackupRoot, backupReportRepo(ev.Repo), issueNumber),
		}
	case "drill", "restore-drill":
		issueNumber := ev.Issue.Number
		if len(fields) >= 3 {
			parsed, ok := parseBackupIssueNumber(fields[2])
			if ok {
				issueNumber = parsed
			}
		}
		return backupIssueCommand{
			Name:         "drill",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup drill --root %s --repo %s --issue %d", defaultBackupRoot, backupReportRepo(ev.Repo), issueNumber),
		}
	case "risk", "risk-audit":
		return backupIssueCommand{
			Name:         "risk",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup risk --root %s --repo %s", defaultBackupRoot, backupReportRepo(ev.Repo)),
		}
	case "provenance", "git-provenance":
		return backupIssueCommand{
			Name:         "provenance",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup provenance --root %s --repo %s", defaultBackupRoot, backupReportRepo(ev.Repo)),
		}
	case "manifest":
		return backupIssueCommand{
			Name:         "manifest",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup manifest --root %s --repo %s --issue %d", defaultBackupRoot, backupReportRepo(ev.Repo), ev.Issue.Number),
		}
	case "list":
		return backupIssueCommand{
			Name:         "list",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup list --root %s --repo %s --limit 20", defaultBackupRoot, backupReportRepo(ev.Repo)),
		}
	case "timeline", "history":
		return backupIssueCommand{
			Name:         "timeline",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup timeline --root %s --repo %s --limit 20", defaultBackupRoot, backupReportRepo(ev.Repo)),
		}
	case "info":
		issueNumber := ev.Issue.Number
		if len(fields) >= 3 {
			parsed, ok := parseBackupIssueNumber(fields[2])
			if !ok {
				return backupIssueCommand{
					Name:   "info",
					Status: "invalid_issue",
				}
			}
			issueNumber = parsed
		}
		return backupIssueCommand{
			Name:         "info",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup info --root %s --repo %s --issue %d", defaultBackupRoot, backupReportRepo(ev.Repo), issueNumber),
		}
	case "stats":
		return backupIssueCommand{
			Name:         "stats",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup stats --root %s --repo %s", defaultBackupRoot, backupReportRepo(ev.Repo)),
		}
	case "search":
		query := cleanBackupSearchQuery(strings.Join(fields[2:], " "))
		return backupIssueCommand{
			Name:         "search",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup search --root %s --repo %s <query>", defaultBackupRoot, backupReportRepo(ev.Repo)),
			QueryHash:    shortDocumentHash(query),
			QueryTerms:   len(strings.Fields(query)),
		}
	case "export", "export-jsonl":
		return backupIssueCommand{
			Name:         "export-jsonl",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup export-jsonl --root %s --repo %s --issue %d", defaultBackupRoot, backupReportRepo(ev.Repo), ev.Issue.Number),
		}
	case "restore", "restore-plan":
		return backupIssueCommand{
			Name:         "restore-plan",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup restore-plan --root %s --repo %s --issue %d", defaultBackupRoot, backupReportRepo(ev.Repo), ev.Issue.Number),
		}
	case "retention", "retention-plan":
		return backupIssueCommand{
			Name:         "retention-plan",
			Status:       "ok",
			LocalCommand: fmt.Sprintf("gitclaw backup retention-plan --root %s --repo %s --keep-latest 50", defaultBackupRoot, backupReportRepo(ev.Repo)),
		}
	default:
		return backupIssueCommand{
			Name:   "unknown",
			Status: "unknown",
		}
	}
}

func writeBackupIssueCommandSummary(b *strings.Builder, request backupIssueCommand) {
	switch request.Status {
	case "ok":
		fmt.Fprintf(b, "- `%s` requested from the issue thread\n", request.Name)
		if request.LocalCommand != "" {
			fmt.Fprintf(b, "- run `%s` after fetching `%s`\n", backupInlineCommand(request.LocalCommand), defaultBackupBranch)
		}
		if request.Name == "search" {
			b.WriteString("- raw search query is not printed; only query hash and term count are shown\n")
		}
	case "unknown":
		b.WriteString("- unknown backup subcommand; supported issue intents are `verify`, `coverage`, `drill`, `risk`, `provenance`, `manifest`, `list`, `timeline`, `info`, `stats`, `search`, `export-jsonl`, `restore-plan`, and `retention-plan`\n")
	case "invalid_issue":
		b.WriteString("- invalid backup issue number; use `@gitclaw /backup info <issue-number>`, `@gitclaw /backup coverage <issue-number>`, `@gitclaw /backup drill <issue-number>`, or inspect the current issue without an explicit issue number\n")
	default:
		b.WriteString("- summary report requested\n")
	}
	b.WriteString("- issue-side execution is metadata-only because the backup branch is written after this assistant turn\n")
}

func writeBackupIssueRiskSummary(b *strings.Builder) {
	b.WriteString("- backup_risk_status: `deferred`\n")
	b.WriteString("- backup_risk_execution: `local_fetched_backup_branch`\n")
	b.WriteString("- backup_risk_categories: `integrity, path-safety, credential-handling, prompt-boundary, restore-safety, retention`\n")
	b.WriteString("- raw_backup_payloads_scanned_issue_side: `false`\n")
	b.WriteString("- llm_e2e_required_after_backup_risk_change: `true`\n")
}

func writeBackupIssueProvenanceSummary(b *strings.Builder) {
	b.WriteString("- backup_provenance_status: `deferred`\n")
	b.WriteString("- backup_provenance_execution: `local_fetched_backup_branch`\n")
	b.WriteString("- backup_provenance_gates: `verify, git-history, body-free-output`\n")
	b.WriteString("- raw_backup_payloads_scanned_issue_side: `false`\n")
	b.WriteString("- raw_git_subjects_included: `false`\n")
	b.WriteString("- author_identities_included: `false`\n")
	b.WriteString("- llm_e2e_required_after_backup_provenance_change: `true`\n")
}

func writeBackupIssueDrillSummary(b *strings.Builder) {
	b.WriteString("- backup_drill_status: `deferred`\n")
	b.WriteString("- backup_drill_execution: `local_fetched_backup_branch`\n")
	b.WriteString("- backup_drill_gates: `verify, coverage, restore-plan`\n")
	b.WriteString("- raw_backup_payloads_scanned_issue_side: `false`\n")
	b.WriteString("- llm_e2e_required_after_backup_drill_change: `true`\n")
}

func writeBackupIssueRestorePlanSummary(b *strings.Builder) {
	b.WriteString("- backup_restore_plan_status: `deferred`\n")
	b.WriteString("- backup_restore_plan_execution: `local_fetched_backup_branch`\n")
	b.WriteString("- backup_restore_plan_mode: `dry-run`\n")
	b.WriteString("- backup_restore_plan_gates: `verify, body-free-output, explicit-future-approval`\n")
	b.WriteString("- raw_backup_payloads_scanned_issue_side: `false`\n")
	b.WriteString("- repository_mutation_allowed_issue_side: `false`\n")
	b.WriteString("- github_api_calls_performed_issue_side: `false`\n")
	b.WriteString("- llm_e2e_required_after_backup_restore_plan_change: `true`\n")
}

func writeBackupIssueRetentionPlanSummary(b *strings.Builder) {
	b.WriteString("- backup_retention_plan_status: `deferred`\n")
	b.WriteString("- backup_retention_plan_execution: `local_fetched_backup_branch`\n")
	b.WriteString("- backup_retention_plan_mode: `dry-run`\n")
	b.WriteString("- backup_retention_plan_gates: `verify, keep-latest-plan, body-free-output, explicit-future-approval`\n")
	b.WriteString("- raw_backup_payloads_scanned_issue_side: `false`\n")
	b.WriteString("- repository_mutation_allowed_issue_side: `false`\n")
	b.WriteString("- branch_deletion_allowed_issue_side: `false`\n")
	b.WriteString("- github_api_calls_performed_issue_side: `false`\n")
	b.WriteString("- llm_e2e_required_after_backup_retention_plan_change: `true`\n")
}

func writeBackupIssueExportJSONLSummary(b *strings.Builder) {
	b.WriteString("- backup_export_jsonl_status: `deferred`\n")
	b.WriteString("- backup_export_jsonl_execution: `local_fetched_backup_branch`\n")
	b.WriteString("- backup_export_jsonl_mode: `explicit_raw_recovery_path`\n")
	b.WriteString("- raw_backup_payloads_scanned_issue_side: `false`\n")
	b.WriteString("- raw_jsonl_included_issue_side: `false`\n")
	b.WriteString("- llm_e2e_required_after_backup_export_jsonl_change: `true`\n")
}

func parseBackupIssueNumber(value string) (int, bool) {
	cleaned := strings.Trim(strings.TrimSpace(value), "#.,:;!?`\"'")
	parsed, err := strconv.Atoi(cleaned)
	return parsed, err == nil && parsed > 0
}

func backupReportRepo(repo string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "<owner/repo>"
	}
	return repo
}

func cleanBackupCommandName(value string) string {
	return strings.Trim(strings.TrimSpace(strings.ToLower(value)), " \t\r\n.,:;!?`\"'")
}

func cleanBackupSearchQuery(query string) string {
	return strings.Trim(strings.TrimSpace(query), " \t\r\n.,:;!?`\"'")
}

func backupInlineCommand(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "`", "'")
	return strings.ReplaceAll(value, "\n", " ")
}

func RenderBackupVerifyReport(result BackupVerifyResult) string {
	status := "ok"
	if !result.OK() {
		status = "warn"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Backup Verify Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", result.Repo)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", status)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", result.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", result.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", result.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", result.ReadmePath)
	fmt.Fprintf(&b, "- issues_checked: `%d`\n", result.IssuesChecked)
	fmt.Fprintf(&b, "- comments_checked: `%d`\n", result.CommentsChecked)
	fmt.Fprintf(&b, "- transcript_messages_checked: `%d`\n", result.TranscriptMessages)
	fmt.Fprintf(&b, "- unindexed_issue_files: `%d`\n", result.UnindexedIssueFiles)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", len(result.VerificationFailures))
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_verify_change: `%t`\n\n", true)

	b.WriteString("### Verification Scope\n")
	b.WriteString("- repo-scoped `index.json`\n")
	b.WriteString("- repo-scoped `README.md`\n")
	b.WriteString("- canonical `issues/000000.json` payload paths\n")
	b.WriteString("- traversal-safe index paths\n")
	b.WriteString("- issue backup schema version, repository, number, title, counts, and timestamps\n")
	b.WriteString("- no unindexed issue backup JSON files\n\n")

	b.WriteString("### Failures\n")
	if result.OK() {
		b.WriteString("- none\n")
	} else {
		for _, failure := range result.VerificationFailures {
			fmt.Fprintf(&b, "- `%s`\n", inlineCode(failure))
		}
	}
	return strings.TrimSpace(b.String())
}
