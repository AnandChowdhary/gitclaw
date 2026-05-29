package gitclaw

import (
	"fmt"
	"path/filepath"
	"strings"
)

const defaultBackupBranch = "gitclaw-backups"

func IsBackupReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/backup" || command == "/backups"
}

func RenderBackupReport(ev Event, comments []Comment, transcript []TranscriptMessage) string {
	relIssuePath := issueBackupPath(".gitclaw/backups", ev.Repo, ev.Issue.Number)
	repoDir := backupRepoDir(".gitclaw/backups", ev.Repo)
	indexPath := filepath.ToSlash(filepath.Join(repoDir, "index.json"))
	readmePath := filepath.ToSlash(filepath.Join(repoDir, "README.md"))
	relIssuePath = filepath.ToSlash(relIssuePath)

	var b strings.Builder
	b.WriteString("## GitClaw Backup Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- issue_backup_path: `%s`\n", relIssuePath)
	fmt.Fprintf(&b, "- index_path: `%s`\n", indexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", readmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", 1)
	fmt.Fprintf(&b, "- raw_comments_now: `%d`\n", len(comments))
	fmt.Fprintf(&b, "- transcript_messages_now: `%d`\n", len(transcript))
	fmt.Fprintf(&b, "- assistant_turn_comments_now: `%d`\n", countSessionMarkers(comments).AssistantTurns)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n\n", shortDocumentHash(ev.Issue.Title))

	b.WriteString("The backup job runs after a successful assistant turn and writes the raw transcript backup plus repo-scoped index to the dedicated backup branch.\n\n")
	b.WriteString("Issue and comment bodies are not included in this report; the raw backup JSON is the canonical transcript copy.\n\n")
	b.WriteString("### Backup Contents\n")
	b.WriteString("- issue metadata\n")
	b.WriteString("- raw issue comments\n")
	b.WriteString("- reconstructed transcript with GitClaw assistant markers stripped\n")
	b.WriteString("- generation timestamp\n")
	b.WriteString("- schema version\n")
	b.WriteString("\n### Verification\n")
	b.WriteString("- `gitclaw backup verify --root .gitclaw/backups --repo <owner/repo>`\n")
	b.WriteString("- validates the repo-scoped index, README, canonical issue paths, JSON schema version, counts, timestamps, and traversal-safe payload paths\n")

	return strings.TrimSpace(b.String())
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
	fmt.Fprintf(&b, "- verification_failures: `%d`\n\n", len(result.VerificationFailures))

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
