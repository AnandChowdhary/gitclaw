package gitclaw

import (
	"fmt"
	"path/filepath"
	"strings"
)

type backupCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

func RenderBackupCatalogIssueReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) string {
	repo := backupReportRepo(ev.Repo)
	paths := backupCatalogPaths(defaultBackupRoot, repo, ev.Issue.Number)
	return renderBackupCatalogReport(backupCatalogRenderOptions{
		Repo:           repo,
		IssueNumber:    ev.Issue.Number,
		IssueTitle:     ev.Issue.Title,
		IncludeIssue:   true,
		RawComments:    len(comments),
		Transcript:     len(transcript),
		AssistantTurns: countSessionMarkers(comments).AssistantTurns,
		Paths:          paths,
	})
}

func RenderBackupCatalogCLIReport(root string, repo string) string {
	repo = backupReportRepo(repo)
	if strings.TrimSpace(root) == "" {
		root = defaultBackupRoot
	}
	return renderBackupCatalogReport(backupCatalogRenderOptions{
		Repo:  repo,
		Root:  root,
		Paths: backupCatalogPaths(root, repo, 0),
	})
}

type backupCatalogRenderOptions struct {
	Repo           string
	Root           string
	IssueNumber    int
	IssueTitle     string
	IncludeIssue   bool
	RawComments    int
	Transcript     int
	AssistantTurns int
	Paths          backupCatalogPathSet
}

type backupCatalogPathSet struct {
	Root       string
	RepoDir    string
	IssuePath  string
	IndexPath  string
	ReadmePath string
}

func backupCatalogPaths(root string, repo string, issueNumber int) backupCatalogPathSet {
	repoDir := backupRepoDir(root, repo)
	paths := backupCatalogPathSet{
		Root:       filepath.ToSlash(root),
		RepoDir:    filepath.ToSlash(repoDir),
		IndexPath:  filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath: filepath.ToSlash(filepath.Join(repoDir, "README.md")),
	}
	if issueNumber > 0 {
		paths.IssuePath = filepath.ToSlash(issueBackupPath(root, repo, issueNumber))
	}
	return paths
}

func renderBackupCatalogReport(opts backupCatalogRenderOptions) string {
	root := opts.Root
	if root == "" {
		root = opts.Paths.Root
	}
	if root == "" {
		root = defaultBackupRoot
	}
	entries := backupCatalogEntries(root, opts.Repo)
	var b strings.Builder
	b.WriteString("## GitClaw Backup Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if opts.IncludeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", opts.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", opts.IssueNumber)
		fmt.Fprintf(&b, "- requested_backup_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- backup_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- requested_local_command: `%s`\n", backupInlineCommand(fmt.Sprintf("gitclaw backup catalog --repo %s", opts.Repo)))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(opts.IssueTitle))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "deferred_to_post_turn_backup_branch")
		fmt.Fprintf(&b, "- raw_comments_now: `%d`\n", opts.RawComments)
		fmt.Fprintf(&b, "- transcript_messages_now: `%d`\n", opts.Transcript)
		fmt.Fprintf(&b, "- assistant_turn_comments_now: `%d`\n", opts.AssistantTurns)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
		fmt.Fprintf(&b, "- repository: `%s`\n", opts.Repo)
	}
	fmt.Fprintf(&b, "- backup_catalog_status: `%s`\n", "ok")
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-git-backed-recovery-discovery")
	fmt.Fprintf(&b, "- backup_model: `%s`\n", "github-issues-plus-gitclaw-backups-branch")
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", opts.Paths.RepoDir)
	if opts.Paths.IssuePath != "" {
		fmt.Fprintf(&b, "- issue_backup_path: `%s`\n", opts.Paths.IssuePath)
	}
	fmt.Fprintf(&b, "- index_path: `%s`\n", opts.Paths.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", opts.Paths.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", 1)
	fmt.Fprintf(&b, "- catalog_scope: `%s`\n", "backup-commands-and-recovery-gates")
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- fetched_branch_required_commands: `%d`\n", countFetchedBackupCatalogEntries(entries))
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- retention_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog is a body-free map of the backup and recovery surface. It records which commands exist, where backup state lives, and which gates apply before any raw recovery workflow.\n\n")

	b.WriteString("### Backup Paths\n")
	fmt.Fprintf(&b, "- backup_branch=`%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- backup_root=`%s`\n", root)
	fmt.Fprintf(&b, "- repo_backup_dir=`%s`\n", opts.Paths.RepoDir)
	fmt.Fprintf(&b, "- index_path=`%s`\n", opts.Paths.IndexPath)
	fmt.Fprintf(&b, "- readme_path=`%s`\n", opts.Paths.ReadmePath)
	if opts.Paths.IssuePath != "" {
		fmt.Fprintf(&b, "- issue_backup_path=`%s`\n", opts.Paths.IssuePath)
	}
	b.WriteByte('\n')

	b.WriteString("### Catalog Entries\n")
	for _, entry := range entries {
		fmt.Fprintf(&b, "- command=`%s` issue_intent=`%s` local_command=`%s` execution=`%s` gate=`%s` raw_bodies_included=`%t` mutation_allowed=`%t`\n",
			entry.Name,
			entry.IssueIntent,
			backupInlineCommand(entry.LocalCommand),
			entry.Execution,
			entry.Gate,
			entry.RawBodies,
			entry.MutationAllowed,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Catalog Gates\n")
	b.WriteString("- backup_branch_gate=`fetched-before-local-inspection`\n")
	b.WriteString("- raw_body_gate=`body-free-issue-report`\n")
	b.WriteString("- restore_gate=`plan-only`\n")
	b.WriteString("- retention_gate=`plan-only`\n")
	b.WriteString("- search_gate=`query-hash-and-match-metadata`\n")
	b.WriteString("- provenance_gate=`tracked-clean-committed-backup-files`\n")

	return strings.TrimSpace(b.String())
}

func backupCatalogEntries(root string, repo string) []backupCatalogEntry {
	if strings.TrimSpace(root) == "" {
		root = defaultBackupRoot
	}
	repo = backupReportRepo(repo)
	return []backupCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /backup catalog", LocalCommand: "gitclaw backup catalog", Execution: "metadata-only", Gate: "body-free-output"},
		{Name: "verify", IssueIntent: "@gitclaw /backup verify", LocalCommand: fmt.Sprintf("gitclaw backup verify --root %s --repo %s", root, repo), Execution: "local-fetched-backup-branch", Gate: "schema-index-paths"},
		{Name: "coverage", IssueIntent: "@gitclaw /backup coverage <issue>", LocalCommand: fmt.Sprintf("gitclaw backup coverage --root %s --repo %s --issue <number>", root, repo), Execution: "local-fetched-backup-branch", Gate: "issue-backup-present"},
		{Name: "drill", IssueIntent: "@gitclaw /backup drill <issue>", LocalCommand: fmt.Sprintf("gitclaw backup drill --root %s --repo %s --issue <number>", root, repo), Execution: "local-fetched-backup-branch", Gate: "verify-coverage-restore-plan"},
		{Name: "risk", IssueIntent: "@gitclaw /backup risk", LocalCommand: fmt.Sprintf("gitclaw backup risk --root %s --repo %s", root, repo), Execution: "local-fetched-backup-branch", Gate: "integrity-path-safety-credentials"},
		{Name: "provenance", IssueIntent: "@gitclaw /backup provenance", LocalCommand: fmt.Sprintf("gitclaw backup provenance --root %s --repo %s", root, repo), Execution: "local-fetched-backup-branch", Gate: "tracked-clean-committed"},
		{Name: "manifest", IssueIntent: "@gitclaw /backup manifest", LocalCommand: fmt.Sprintf("gitclaw backup manifest --root %s --repo %s", root, repo), Execution: "local-fetched-backup-branch", Gate: "hash-only-file-inventory"},
		{Name: "list", IssueIntent: "@gitclaw /backup list", LocalCommand: fmt.Sprintf("gitclaw backup list --root %s --repo %s --limit 20", root, repo), Execution: "local-fetched-backup-branch", Gate: "indexed-navigation"},
		{Name: "timeline", IssueIntent: "@gitclaw /backup timeline", LocalCommand: fmt.Sprintf("gitclaw backup timeline --root %s --repo %s --limit 20", root, repo), Execution: "local-fetched-backup-branch", Gate: "chronological-navigation"},
		{Name: "info", IssueIntent: "@gitclaw /backup info <issue>", LocalCommand: fmt.Sprintf("gitclaw backup info --root %s --repo %s --issue <number>", root, repo), Execution: "local-fetched-backup-branch", Gate: "single-issue-metadata"},
		{Name: "stats", IssueIntent: "@gitclaw /backup stats", LocalCommand: fmt.Sprintf("gitclaw backup stats --root %s --repo %s", root, repo), Execution: "local-fetched-backup-branch", Gate: "aggregate-health"},
		{Name: "freshness", IssueIntent: "@gitclaw /backup freshness", LocalCommand: fmt.Sprintf("gitclaw backup freshness --root %s --repo %s --max-age-hours 24", root, repo), Execution: "local-fetched-backup-branch", Gate: "latest-backup-age"},
		{Name: "continuity", IssueIntent: "@gitclaw /backup continuity", LocalCommand: fmt.Sprintf("gitclaw backup continuity --root %s --repo %s --max-gap-hours 168", root, repo), Execution: "local-fetched-backup-branch", Gate: "longest-gap"},
		{Name: "search", IssueIntent: "@gitclaw /backup search <query>", LocalCommand: fmt.Sprintf("gitclaw backup search --root %s --repo %s <query>", root, repo), Execution: "local-fetched-backup-branch", Gate: "query-hash-and-match-metadata"},
		{Name: "export-jsonl", IssueIntent: "@gitclaw /backup export-jsonl", LocalCommand: fmt.Sprintf("gitclaw backup export-jsonl --root %s --repo %s --issue <number>", root, repo), Execution: "explicit-local-raw-recovery", Gate: "operator-local-only"},
		{Name: "restore-plan", IssueIntent: "@gitclaw /backup restore-plan", LocalCommand: fmt.Sprintf("gitclaw backup restore-plan --root %s --repo %s --issue <number>", root, repo), Execution: "local-fetched-backup-branch", Gate: "plan-only"},
		{Name: "retention-plan", IssueIntent: "@gitclaw /backup retention-plan", LocalCommand: fmt.Sprintf("gitclaw backup retention-plan --root %s --repo %s --keep-latest 50", root, repo), Execution: "local-fetched-backup-branch", Gate: "plan-only"},
	}
}

func countFetchedBackupCatalogEntries(entries []backupCatalogEntry) int {
	count := 0
	for _, entry := range entries {
		if strings.Contains(entry.Execution, "fetched-backup-branch") || entry.Execution == "explicit-local-raw-recovery" {
			count++
		}
	}
	return count
}
