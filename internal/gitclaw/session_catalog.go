package gitclaw

import (
	"fmt"
	"strings"
)

type sessionCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

func RenderSessionCatalogIssueReport(ev Event, comments []Comment, transcript []TranscriptMessage) string {
	return renderSessionCatalogReport(sessionCatalogRenderOptions{
		Repo:           ev.Repo,
		IssueNumber:    ev.Issue.Number,
		IssueTitle:     ev.Issue.Title,
		IncludeIssue:   true,
		RawComments:    len(comments),
		Transcript:     len(transcript),
		AssistantTurns: countSessionMarkers(comments).AssistantTurns,
	})
}

func RenderSessionCatalogCLIReport() string {
	return renderSessionCatalogReport(sessionCatalogRenderOptions{})
}

type sessionCatalogRenderOptions struct {
	Repo           string
	IssueNumber    int
	IssueTitle     string
	IncludeIssue   bool
	RawComments    int
	Transcript     int
	AssistantTurns int
}

func renderSessionCatalogReport(opts sessionCatalogRenderOptions) string {
	entries := sessionCatalogEntries()
	var b strings.Builder
	b.WriteString("## GitClaw Session Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if opts.IncludeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", opts.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", opts.IssueNumber)
		fmt.Fprintf(&b, "- requested_session_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- session_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(opts.IssueTitle))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "current_issue_thread_metadata")
		fmt.Fprintf(&b, "- raw_comments_now: `%d`\n", opts.RawComments)
		fmt.Fprintf(&b, "- transcript_messages_now: `%d`\n", opts.Transcript)
		fmt.Fprintf(&b, "- assistant_turn_comments_now: `%d`\n", opts.AssistantTurns)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- session_catalog_status: `%s`\n", "ok")
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-issue-thread-session-discovery")
	fmt.Fprintf(&b, "- session_model: `%s`\n", "github-issue-thread-plus-backup-json")
	fmt.Fprintf(&b, "- canonical_session_store: `%s`\n", "github-issue-thread")
	fmt.Fprintf(&b, "- local_backup_store: `%s`\n", "gitclaw-backups issue JSON")
	fmt.Fprintf(&b, "- catalog_scope: `%s`\n", "session-commands-and-recall-gates")
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- issue_side_commands: `%d`\n", countIssueSessionCatalogEntries(entries))
	fmt.Fprintf(&b, "- local_backup_commands: `%d`\n", countLocalBackupSessionCatalogEntries(entries))
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_assistant_replies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- session_deletion_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- session_export_allowed_issue_side: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog is a body-free map of GitClaw's session inspection surface. It records which session commands exist, which commands need a fetched backup JSON, and which gates keep transcript recall inspectable without printing raw messages.\n\n")

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
	b.WriteString("- issue_thread_gate=`canonical-session-is-github-issue-thread`\n")
	b.WriteString("- local_backup_gate=`fetched-backup-json-required-for-local-transcript-inspection`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-and-metadata-only`\n")
	b.WriteString("- provenance_gate=`assistant-turn-marker-prompt-context`\n")
	b.WriteString("- tools_gate=`assistant-turn-marker-tool-context`\n")
	b.WriteString("- skills_gate=`assistant-turn-marker-skill-context`\n")
	b.WriteString("- search_gate=`query-hash-and-line-hash-metadata`\n")
	b.WriteString("- coverage_gate=`prompt-provenance-skill-tool-telemetry`\n")
	b.WriteString("- channel_boundary_gate=`provider-session-keys-collapse-to-canonical-github-issue`\n")

	return strings.TrimSpace(b.String())
}

func sessionCatalogEntries() []sessionCatalogEntry {
	return []sessionCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /session catalog", LocalCommand: "gitclaw session catalog", Execution: "metadata-only", Gate: "body-free-output"},
		{Name: "list", IssueIntent: "@gitclaw /session list", LocalCommand: "gitclaw session list --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "hash-only-message-list"},
		{Name: "provenance", IssueIntent: "@gitclaw /session provenance", LocalCommand: "gitclaw session provenance --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "assistant-turn-marker-prompt-context"},
		{Name: "tools", IssueIntent: "@gitclaw /session tools", LocalCommand: "gitclaw session tools --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "assistant-turn-marker-tool-context"},
		{Name: "skills", IssueIntent: "@gitclaw /session skills", LocalCommand: "gitclaw session skills --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "assistant-turn-marker-skill-context"},
		{Name: "status", IssueIntent: "@gitclaw /session status", LocalCommand: "gitclaw session status --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "latest-message-hashes"},
		{Name: "stats", IssueIntent: "@gitclaw /session stats", LocalCommand: "gitclaw session stats --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "aggregate-counts-and-provenance"},
		{Name: "coverage", IssueIntent: "@gitclaw /session coverage", LocalCommand: "gitclaw session coverage --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "model-skill-tool-provenance"},
		{Name: "risk", IssueIntent: "@gitclaw /session risk", LocalCommand: "gitclaw session risk --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "prompt-boundary-and-marker-hygiene"},
		{Name: "search", IssueIntent: "@gitclaw /session search <query>", LocalCommand: "gitclaw session search <query> --backup <issue.json>", Execution: "current-issue-or-local-backup", Gate: "query-hash-and-line-hash-metadata"},
	}
}

func countIssueSessionCatalogEntries(entries []sessionCatalogEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.IssueIntent != "" {
			count++
		}
	}
	return count
}

func countLocalBackupSessionCatalogEntries(entries []sessionCatalogEntry) int {
	count := 0
	for _, entry := range entries {
		if strings.Contains(entry.LocalCommand, "--backup") {
			count++
		}
	}
	return count
}
