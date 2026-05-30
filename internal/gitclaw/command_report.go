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
	{Command: "/agents", Aliases: []string{"/agent"}, Model: "gitclaw/agents", Category: "agents", Summary: "Audit repo-local agent policy, agent specs, single-assistant runtime boundaries, and agent risk posture.", LocalCLI: []string{"gitclaw agents list", "gitclaw agents risk", "gitclaw agents verify"}},
	{Command: "/approvals", Aliases: []string{"/approval"}, Model: "gitclaw/approvals", Category: "approval", Summary: "Inspect write-request approval gates without enabling write actions.", LocalCLI: []string{"gitclaw approvals list", "gitclaw approvals verify"}},
	{Command: "/artifacts", Aliases: []string{"/artifact"}, Model: "gitclaw/artifacts", Category: "artifacts", Summary: "Audit Actions artifact policy, artifact specs, upload workflow metadata, and body-safe retention boundaries.", LocalCLI: []string{"gitclaw artifacts list", "gitclaw artifacts verify"}},
	{Command: "/backup", Aliases: []string{"/backups"}, Model: "gitclaw/backup", Category: "backup", Summary: "Show expected backup branch paths and inspect fetched backups, risk posture, manifests, retention, restore plans, and search metadata.", LocalCLI: []string{"gitclaw backup verify", "gitclaw backup risk", "gitclaw backup manifest", "gitclaw backup list", "gitclaw backup info --issue <number>", "gitclaw backup stats", "gitclaw backup search <query>", "gitclaw backup export-jsonl", "gitclaw backup restore-plan", "gitclaw backup retention-plan"}},
	{Command: "/bundles", Model: "gitclaw/skills", Category: "skills", Summary: "List or inspect repo-local skill bundles that group existing skills into task profiles.", LocalCLI: []string{"gitclaw bundles list", "gitclaw bundles info <name>"}},
	{Command: "/channels", Aliases: []string{"/channel"}, Model: "gitclaw/channels", Category: "channels", Summary: "Audit channel bridge settings, workflow-dispatch ingress, provider contracts, and channel risk posture.", LocalCLI: []string{"gitclaw channels verify", "gitclaw channels risk", "gitclaw channels list", "gitclaw channels info <provider>", "gitclaw channel-state", "gitclaw channel-gateway", "gitclaw channel-delivery"}},
	{Command: "/checkpoints", Aliases: []string{"/checkpoint", "/rollback"}, Model: "gitclaw/checkpoints", Category: "checkpoint", Summary: "Inspect git rollback readiness without restoring files or printing diffs.", LocalCLI: []string{"gitclaw checkpoints status", "gitclaw checkpoints list", "gitclaw checkpoints verify", "gitclaw rollback list"}},
	{Command: "/config", Aliases: []string{"/configuration"}, Model: "gitclaw/config", Category: "control-plane", Summary: "Show effective config, labels, prompt budgets, commands, and workflows.", LocalCLI: []string{"gitclaw config list"}},
	{Command: "/context", Model: "gitclaw/context", Category: "context", Summary: "Show loaded context documents, explicit context references, and deterministic tool output metadata.", LocalCLI: []string{"gitclaw context list", "gitclaw context info <path>"}},
	{Command: "/diffs", Aliases: []string{"/diff", "/changes"}, Model: "gitclaw/diffs", Category: "diffs", Summary: "Audit git working-tree changes by metadata without printing raw patches or file bodies.", LocalCLI: []string{"gitclaw diffs summary", "gitclaw diffs verify"}},
	{Command: "/doctor", Aliases: []string{"/health"}, Model: "gitclaw/doctor", Category: "health", Summary: "Run a body-free health check with skill, soul, and tool validation rollups.", LocalCLI: []string{"gitclaw doctor", "gitclaw doctor list"}},
	{Command: "/heartbeat", Model: "gitclaw/heartbeat", Category: "heartbeat", Summary: "Audit scheduled heartbeat workflow, context, permissions, idempotency, and LLM runner contract.", LocalCLI: []string{"gitclaw heartbeat status"}},
	{Command: "/hooks", Aliases: []string{"/hook"}, Model: "gitclaw/hooks", Category: "hooks", Summary: "Audit declarative hook policy, hook specs, ignored handlers, event-driven automation boundaries, and hook risk posture.", LocalCLI: []string{"gitclaw hooks list", "gitclaw hooks risk", "gitclaw hooks verify"}},
	{Command: "/help", Aliases: []string{"/commands"}, Model: "gitclaw/commands", Category: "control-plane", Summary: "List deterministic GitClaw slash commands, aliases, and local CLI helpers.", LocalCLI: []string{"gitclaw commands"}},
	{Command: "/memory", Aliases: []string{"/memories"}, Model: "gitclaw/memory", Category: "memory", Summary: "Audit long-term memory, provenance verification, risk posture, promotion plans, focused memory-file metadata, search, and memory hygiene findings.", LocalCLI: []string{"gitclaw memory verify", "gitclaw memory risk", "gitclaw memory validate", "gitclaw memory list", "gitclaw memory promote-plan [target]", "gitclaw memory info <path>", "gitclaw memory search <query>"}},
	{Command: "/migrate", Aliases: []string{"/migration"}, Model: "gitclaw/migration", Category: "migration", Summary: "Preview OpenClaw, Hermes, Codex, or Claude state imports into GitClaw's repo-reviewed layout without applying them.", LocalCLI: []string{"gitclaw migrate plan <source>"}},
	{Command: "/models", Aliases: []string{"/model"}, Model: "gitclaw/models", Category: "model", Summary: "Show model provider, endpoint, token source, and retry policy.", LocalCLI: []string{"gitclaw models list"}},
	{Command: "/nodes", Aliases: []string{"/node"}, Model: "gitclaw/nodes", Category: "nodes", Summary: "Audit repo-local node policy, node specs, and GitHub Actions runtime boundaries.", LocalCLI: []string{"gitclaw nodes list", "gitclaw nodes verify"}},
	{Command: "/orders", Aliases: []string{"/standing-orders"}, Model: "gitclaw/orders", Category: "standing-orders", Summary: "Audit repo-reviewed standing orders, authority clauses, and proactive enforcement metadata.", LocalCLI: []string{"gitclaw orders list", "gitclaw orders verify"}},
	{Command: "/plugins", Aliases: []string{"/plugin"}, Model: "gitclaw/plugins", Category: "plugins", Summary: "Audit declarative plugin policy, plugin specs, ignored package files, runtime extension boundaries, and plugin risk posture.", LocalCLI: []string{"gitclaw plugins list", "gitclaw plugins risk", "gitclaw plugins verify"}},
	{Command: "/policy", Model: "gitclaw/policy", Category: "policy", Summary: "Show or verify preflight, actor trust, write-intent, labels, and workflow permissions.", LocalCLI: []string{"gitclaw policy list", "gitclaw policy verify"}},
	{Command: "/profile", Aliases: []string{"/profiles"}, Model: "gitclaw/profile", Category: "profile", Summary: "Show the repo-local profile envelope across soul, memory, skills, tools, model, and run gates.", LocalCLI: []string{"gitclaw profile show", "gitclaw profile verify"}},
	{Command: "/prompt", Aliases: []string{"/budget", "/prompt-budget"}, Model: "gitclaw/prompt", Category: "prompt", Summary: "Show prompt budget, truncation, context, skill, and tool-output metadata.", LocalCLI: []string{"gitclaw prompt list"}},
	{Command: "/proactive", Aliases: []string{"/cron"}, Model: "gitclaw/proactive", Category: "proactive", Summary: "Audit proactive prompt files, reminder due gates, scheduled workflow metadata, and proactive risk posture.", LocalCLI: []string{"gitclaw proactive list", "gitclaw proactive risk", "gitclaw proactive info <name>", "gitclaw proactive init", "gitclaw proactive enqueue"}},
	{Command: "/runs", Aliases: []string{"/run", "/ledger"}, Model: "gitclaw/runs", Category: "run-ledger", Summary: "Show current turn/run provenance, labels, markers, and prompt-visible input hashes.", LocalCLI: []string{"gitclaw runs current", "gitclaw runs verify"}},
	{Command: "/sandbox", Aliases: []string{"/sandboxes", "/exec-policy"}, Model: "gitclaw/sandbox", Category: "security", Summary: "Explain the GitHub Actions sandbox, host-exec policy, tool modes, and workflow permission boundary.", LocalCLI: []string{"gitclaw sandbox explain", "gitclaw sandbox verify"}},
	{Command: "/secrets", Aliases: []string{"/secret"}, Model: "gitclaw/secrets", Category: "security", Summary: "Run a read-only repo secret audit without printing matched values.", LocalCLI: []string{"gitclaw secrets audit", "gitclaw secrets scan", "gitclaw secrets list"}},
	{Command: "/session", Model: "gitclaw/session", Category: "session", Summary: "Show or search reconstructed transcript counts, markers, trust, and hashes.", LocalCLI: []string{"gitclaw session list --backup <issue.json>", "gitclaw session search <query> --backup <issue.json>"}},
	{Command: "/skills", Model: "gitclaw/skills", Category: "skills", Summary: "List local skill metadata, trust verification, risk audit, validation findings, selection/install/upgrade plans, or one focused skill info/search report.", LocalCLI: []string{"gitclaw skills verify", "gitclaw skills risk", "gitclaw skills validate", "gitclaw skills check", "gitclaw skills list", "gitclaw skills select-plan <name>", "gitclaw skills install-plan <target>", "gitclaw skills upgrade-plan <target>", "gitclaw skills info <name>", "gitclaw skills search <query>"}},
	{Command: "/soul", Model: "gitclaw/soul", Category: "soul", Summary: "Audit, verify, inspect, plan edits for, risk-scan, or search high-authority context files and soul validation findings.", LocalCLI: []string{"gitclaw soul verify", "gitclaw soul risk", "gitclaw soul validate", "gitclaw soul list", "gitclaw soul edit-plan <path>", "gitclaw soul info <path>", "gitclaw soul search <query>"}},
	{Command: "/tasks", Aliases: []string{"/task"}, Model: "gitclaw/tasks", Category: "tasks", Summary: "Audit GitHub issue-native task policy, task specs, labels, flow boundaries, and task risk posture.", LocalCLI: []string{"gitclaw tasks list", "gitclaw tasks risk", "gitclaw tasks verify"}},
	{Command: "/tools", Model: "gitclaw/tools", Category: "tools", Summary: "Audit, verify, risk-scan, inspect, plan runs for, or search deterministic tool contracts, active outputs, and validation findings.", LocalCLI: []string{"gitclaw tools verify", "gitclaw tools risk", "gitclaw tools validate", "gitclaw tools list", "gitclaw tools run-plan <name>", "gitclaw tools info <name>", "gitclaw tools search <query>"}},
	{Command: "/workspace", Aliases: []string{"/workdir", "/repo"}, Model: "gitclaw/workspace", Category: "workspace", Summary: "Audit the GitHub Actions repository checkout, workspace policy, specs, and isolation boundaries.", LocalCLI: []string{"gitclaw workspace summary", "gitclaw workspace verify"}},
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
