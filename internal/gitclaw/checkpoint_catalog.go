package gitclaw

import (
	"fmt"
	"strings"
)

type checkpointCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

type checkpointCatalogLayer struct {
	Name      string
	Store     string
	Source    string
	Gate      string
	Count     int
	RawBodies bool
}

func RenderCheckpointCatalogReport(ev Event, report CheckpointReport) string {
	return renderCheckpointCatalogReport(ev, report, true)
}

func RenderCheckpointCatalogCLIReport(report CheckpointReport) string {
	return renderCheckpointCatalogReport(Event{}, report, false)
}

func renderCheckpointCatalogReport(ev Event, checkpoint CheckpointReport, includeIssue bool) string {
	risk := BuildCheckpointRiskReport(checkpoint)
	entries := checkpointCatalogEntries()
	layers := checkpointCatalogLayers(checkpoint)

	var b strings.Builder
	b.WriteString("## GitClaw Checkpoints Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_checkpoints_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- checkpoints_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "github_actions_checkpoint_metadata")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- checkpoint_catalog_status: `%s`\n", risk.Status)
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-git-history-rollback-discovery")
	fmt.Fprintf(&b, "- checkpoint_strategy: `%s`\n", risk.CheckpointStrategy)
	fmt.Fprintf(&b, "- rollback_model: `%s`\n", "github-actions-git-metadata-inspect-only")
	fmt.Fprintf(&b, "- rollback_mode: `%s`\n", risk.RollbackMode)
	fmt.Fprintf(&b, "- git_available: `%t`\n", risk.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", risk.GitRepository)
	fmt.Fprintf(&b, "- worktree_root: `%s`\n", risk.WorktreeRoot)
	fmt.Fprintf(&b, "- branch: `%s`\n", risk.Branch)
	fmt.Fprintf(&b, "- head_commit: `%s`\n", risk.HeadCommit)
	fmt.Fprintf(&b, "- commits_available: `%d`\n", risk.CommitsAvailable)
	fmt.Fprintf(&b, "- recent_commits_returned: `%d`\n", risk.RecentCommitsReturned)
	fmt.Fprintf(&b, "- recent_commit_limit: `%d`\n", risk.RecentCommitLimit)
	fmt.Fprintf(&b, "- worktree_clean: `%t`\n", risk.WorktreeClean)
	fmt.Fprintf(&b, "- staged_changes: `%d`\n", risk.StagedChanges)
	fmt.Fprintf(&b, "- unstaged_changes: `%d`\n", risk.UnstagedChanges)
	fmt.Fprintf(&b, "- untracked_files: `%d`\n", risk.UntrackedFiles)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", risk.BackupBranch)
	fmt.Fprintf(&b, "- backup_branch_local_ref: `%t`\n", risk.BackupBranchLocalRef)
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- checkpoint_layers: `%d`\n", len(layers))
	fmt.Fprintf(&b, "- restore_operations_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- git_reset_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- git_clean_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- checkout_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- pre_restore_snapshot_required: `%t`\n", true)
	fmt.Fprintf(&b, "- rollback_diff_preview_required: `%t`\n", true)
	fmt.Fprintf(&b, "- backup_manifest_required_for_restore: `%t`\n", true)
	fmt.Fprintf(&b, "- raw_diffs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_checkpoint_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog maps GitClaw's checkpoint and rollback surface inspired by Hermes' shadow-store checkpoints, rollback diff previews, and worktree isolation plus OpenClaw's backup manifest verification posture: it exposes commands, layers, and gates while keeping diffs, file bodies, commit subjects, issue/comment bodies, prompts, tool outputs, credentials, and secret values out of the report.\n\n")

	b.WriteString("### Catalog Entries\n")
	for _, entry := range entries {
		fmt.Fprintf(&b, "- command=`%s` issue_intent=`%s` local_command=`%s` execution=`%s` gate=`%s` raw_bodies_included=`%t` mutation_allowed=`%t`\n",
			entry.Name,
			entry.IssueIntent,
			entry.LocalCommand,
			entry.Execution,
			entry.Gate,
			entry.RawBodies,
			entry.MutationAllowed,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Checkpoint Layers\n")
	for _, layer := range layers {
		fmt.Fprintf(&b, "- layer=`%s` store=`%s` source=`%s` gate=`%s` count=`%d` raw_bodies_included=`%t`\n",
			layer.Name,
			layer.Store,
			layer.Source,
			layer.Gate,
			layer.Count,
			layer.RawBodies,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Catalog Gates\n")
	fmt.Fprintf(&b, "- checkpoint_catalog_gate=`%s`\n", risk.Status)
	b.WriteString("- git_metadata_gate=`available-before-report`\n")
	b.WriteString("- worktree_gate=`clean-preferred-dirty-is-warning`\n")
	b.WriteString("- backup_branch_gate=`manifest-required-before-restore`\n")
	b.WriteString("- rollback_preview_gate=`diff-preview-required-before-restore`\n")
	b.WriteString("- restore_gate=`disabled-inspect-only-v1`\n")
	b.WriteString("- destructive_git_gate=`reset-clean-checkout-disabled`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-and-metadata-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func checkpointCatalogEntries() []checkpointCatalogEntry {
	return []checkpointCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /checkpoints catalog", LocalCommand: "gitclaw checkpoints catalog", Execution: "metadata-only", Gate: "body-free-checkpoint-command-map"},
		{Name: "status", IssueIntent: "@gitclaw /checkpoints", LocalCommand: "gitclaw checkpoints status", Execution: "metadata-only", Gate: "rollback-readiness-inventory"},
		{Name: "list", IssueIntent: "@gitclaw /checkpoints list", LocalCommand: "gitclaw checkpoints list", Execution: "metadata-only", Gate: "rollback-readiness-inventory"},
		{Name: "verify", IssueIntent: "@gitclaw /checkpoints verify", LocalCommand: "gitclaw checkpoints verify", Execution: "metadata-only", Gate: "rollback-readiness-inventory"},
		{Name: "preview", IssueIntent: "@gitclaw /checkpoints preview HEAD~1", LocalCommand: "gitclaw checkpoints preview HEAD~1", Execution: "diff-stat-preview", Gate: "rollback-diff-preview-before-restore"},
		{Name: "risk", IssueIntent: "@gitclaw /checkpoints risk", LocalCommand: "gitclaw checkpoints risk", Execution: "risk-audit", Gate: "rollback-safety-risk-audit"},
		{Name: "rollback-catalog", IssueIntent: "@gitclaw /rollback catalog", LocalCommand: "gitclaw rollback catalog", Execution: "metadata-only", Gate: "body-free-rollback-command-map"},
		{Name: "rollback-list", IssueIntent: "@gitclaw /rollback", LocalCommand: "gitclaw rollback list", Execution: "metadata-only", Gate: "rollback-readiness-inventory"},
		{Name: "rollback-diff", IssueIntent: "@gitclaw /rollback diff HEAD~1", LocalCommand: "gitclaw rollback diff HEAD~1", Execution: "diff-stat-preview", Gate: "rollback-diff-preview-before-restore"},
		{Name: "rollback-risk", IssueIntent: "@gitclaw /rollback risk", LocalCommand: "gitclaw rollback risk", Execution: "risk-audit", Gate: "rollback-safety-risk-audit"},
	}
}

func checkpointCatalogLayers(checkpoint CheckpointReport) []checkpointCatalogLayer {
	return []checkpointCatalogLayer{
		{Name: "git-history", Store: "repository .git metadata", Source: "checked-out-repository", Gate: "git-repository-required", Count: checkpointBoolCount(checkpoint.GitRepository)},
		{Name: "worktree", Store: "git status --porcelain", Source: "checked-out-worktree", Gate: "dirty-state-counts-only", Count: checkpoint.StagedChanges + checkpoint.UnstagedChanges + checkpoint.UntrackedFiles},
		{Name: "backup-branch", Store: checkpoint.BackupBranch, Source: "git references", Gate: "backup-manifest-before-restore", Count: checkpointBoolCount(checkpoint.BackupBranchLocalRef)},
		{Name: "recent-commits", Store: "git log metadata", Source: "recent-commit-window", Gate: "commit-subject-hashes-only", Count: checkpoint.RecentCommitsReturned},
		{Name: "restore-preview", Store: "rollback diff stat and path hashes", Source: "git diff --numstat --name-status", Gate: "preview-required-before-restore", Count: 1},
		{Name: "operation-boundary", Store: "unsupported restore/reset/clean/checkout", Source: "explicit-negative-capability", Gate: "inspect-only-v1", Count: 0},
		{Name: "payloads", Store: "unsupported in reports", Source: "explicit-negative-capability", Gate: "body-free-reporting", Count: 0},
	}
}

func isCheckpointCatalogRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/checkpoints" && command != "/checkpoint" && command != "/rollback" {
		return false
	}
	return strings.EqualFold(fields[1], "catalog") ||
		strings.EqualFold(fields[1], "commands") ||
		strings.EqualFold(fields[1], "index") ||
		strings.EqualFold(fields[1], "map")
}

func checkpointBoolCount(ok bool) int {
	if ok {
		return 1
	}
	return 0
}
