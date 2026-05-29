package gitclaw

import (
	"fmt"
	"strings"
)

type commandCatalogEntry struct {
	Command  string
	Aliases  []string
	Model    string
	Category string
	Summary  string
	LocalCLI []string
}

var commandCatalog = []commandCatalogEntry{
	{Command: "/backup", Aliases: []string{"/backups"}, Model: "gitclaw/backup", Category: "backup", Summary: "Show expected backup branch paths for the current issue.", LocalCLI: []string{"gitclaw backup verify", "gitclaw backup manifest", "gitclaw backup stats", "gitclaw backup export-jsonl", "gitclaw backup restore-plan", "gitclaw backup retention-plan"}},
	{Command: "/channels", Aliases: []string{"/channel"}, Model: "gitclaw/channels", Category: "channels", Summary: "Audit channel bridge settings and workflow-dispatch ingress."},
	{Command: "/config", Aliases: []string{"/configuration"}, Model: "gitclaw/config", Category: "control-plane", Summary: "Show effective config, labels, prompt budgets, commands, and workflows."},
	{Command: "/context", Model: "gitclaw/context", Category: "context", Summary: "Show loaded context documents and deterministic tool output metadata."},
	{Command: "/doctor", Aliases: []string{"/health"}, Model: "gitclaw/doctor", Category: "health", Summary: "Run a body-free health check with skill, soul, and tool validation rollups.", LocalCLI: []string{"gitclaw doctor"}},
	{Command: "/help", Aliases: []string{"/commands"}, Model: "gitclaw/commands", Category: "control-plane", Summary: "List deterministic GitClaw slash commands, aliases, and local CLI helpers.", LocalCLI: []string{"gitclaw commands"}},
	{Command: "/memory", Aliases: []string{"/memories"}, Model: "gitclaw/memory", Category: "memory", Summary: "Audit long-term memory, dated memory note metadata, search, and memory hygiene findings.", LocalCLI: []string{"gitclaw memory validate", "gitclaw memory search <query>"}},
	{Command: "/models", Aliases: []string{"/model"}, Model: "gitclaw/models", Category: "model", Summary: "Show model provider, endpoint, token source, and retry policy."},
	{Command: "/policy", Model: "gitclaw/policy", Category: "policy", Summary: "Show preflight, actor trust, write-intent, labels, and workflow permissions."},
	{Command: "/prompt", Model: "gitclaw/prompt", Category: "prompt", Summary: "Show prompt budget, truncation, context, skill, and tool-output metadata."},
	{Command: "/proactive", Model: "gitclaw/proactive", Category: "proactive", Summary: "Audit proactive prompt files and scheduled workflow metadata.", LocalCLI: []string{"gitclaw proactive init", "gitclaw proactive enqueue"}},
	{Command: "/session", Model: "gitclaw/session", Category: "session", Summary: "Show reconstructed transcript counts, markers, trust, and hashes."},
	{Command: "/skills", Model: "gitclaw/skills", Category: "skills", Summary: "List local skill metadata, selection state, validation findings, or one focused skill info/search report.", LocalCLI: []string{"gitclaw skills validate", "gitclaw skills info <name>", "gitclaw skills search <query>"}},
	{Command: "/soul", Model: "gitclaw/soul", Category: "soul", Summary: "Audit or search high-authority context files and soul validation findings.", LocalCLI: []string{"gitclaw soul validate", "gitclaw soul search <query>"}},
	{Command: "/tools", Model: "gitclaw/tools", Category: "tools", Summary: "Audit or search deterministic tool contracts, active outputs, and validation findings.", LocalCLI: []string{"gitclaw tools validate", "gitclaw tools search <query>"}},
}

func IsCommandReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/help" || command == "/commands"
}

func RenderCommandReport(ev Event, cfg Config) string {
	return renderCommandReport(ev, cfg, true)
}

func RenderCommandCLIReport(cfg Config) string {
	return renderCommandReport(Event{}, cfg, false)
}

func renderCommandReport(ev Event, cfg Config, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Commands Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- trigger_prefix: `%s`\n", cfg.TriggerPrefix)
	fmt.Fprintf(&b, "- commands: `%d`\n", len(commandCatalog))
	fmt.Fprintf(&b, "- aliases: `%d`\n", commandAliasCount(commandCatalog))
	fmt.Fprintf(&b, "- local_cli_helpers: `%d`\n", commandLocalCLICount(commandCatalog))
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report lists GitClaw's deterministic slash commands. Issue bodies, comments, config bodies, prompts, and backup payloads are not included.\n\n")

	b.WriteString("### Slash Commands\n")
	for _, entry := range commandCatalog {
		fmt.Fprintf(&b, "- `%s` model=`%s` category=`%s` aliases=`%s` - %s\n", entry.Command, entry.Model, entry.Category, inlineList(entry.Aliases), entry.Summary)
	}

	b.WriteString("\n### Local CLI Helpers\n")
	wrote := false
	for _, entry := range commandCatalog {
		for _, helper := range entry.LocalCLI {
			wrote = true
			fmt.Fprintf(&b, "- `%s` command=`%s`\n", helper, entry.Command)
		}
	}
	if !wrote {
		b.WriteString("- none\n")
	}
	return strings.TrimSpace(b.String())
}

func commandCatalogNames() []string {
	commands := make([]string, 0, len(commandCatalog))
	for _, entry := range commandCatalog {
		commands = append(commands, entry.Command)
	}
	return commands
}

func commandAliasCount(entries []commandCatalogEntry) int {
	count := 0
	for _, entry := range entries {
		count += len(entry.Aliases)
	}
	return count
}

func commandLocalCLICount(entries []commandCatalogEntry) int {
	count := 0
	for _, entry := range entries {
		count += len(entry.LocalCLI)
	}
	return count
}

func inlineList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return inlineCode(strings.Join(values, ", "))
}
