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
	{Command: "/agents", Aliases: []string{"/agent"}, Model: "gitclaw/agents", Category: "agents", Summary: "Catalog, audit git provenance for, or risk-audit repo-local agent policy, agent specs, and single-assistant runtime boundaries.", LocalCLI: []string{"gitclaw agents catalog", "gitclaw agents list", "gitclaw agents provenance", "gitclaw agents risk", "gitclaw agents verify"}},
	{Command: "/approvals", Aliases: []string{"/approval"}, Model: "gitclaw/approvals", Category: "approval", Summary: "Catalog, inspect, prove provenance for, or risk-audit write-request approval gates without enabling write actions.", LocalCLI: []string{"gitclaw approvals catalog", "gitclaw approvals list", "gitclaw approvals verify", "gitclaw approvals provenance", "gitclaw approvals risk"}},
	{Command: "/artifacts", Aliases: []string{"/artifact"}, Model: "gitclaw/artifacts", Category: "artifacts", Summary: "Catalog or audit Actions artifact policy, artifact specs, upload workflow metadata, body-safe retention boundaries, and artifact risk posture.", LocalCLI: []string{"gitclaw artifacts catalog", "gitclaw artifacts list", "gitclaw artifacts risk", "gitclaw artifacts verify"}},
	{Command: "/backup", Aliases: []string{"/backups"}, Model: "gitclaw/backup", Category: "backup", Summary: "Catalog backup commands, show expected backup branch paths, and inspect fetched backups, snapshots, coverage, restore-readiness drills, risk posture, git provenance, manifests, freshness, continuity, timelines, retention, restore plans, and search metadata.", LocalCLI: []string{"gitclaw backup catalog", "gitclaw backup verify", "gitclaw backup snapshot", "gitclaw backup coverage --issue <number>", "gitclaw backup drill --issue <number>", "gitclaw backup risk", "gitclaw backup provenance", "gitclaw backup manifest", "gitclaw backup list", "gitclaw backup timeline", "gitclaw backup info --issue <number>", "gitclaw backup stats", "gitclaw backup freshness", "gitclaw backup continuity", "gitclaw backup search <query>", "gitclaw backup export-jsonl", "gitclaw backup restore-plan", "gitclaw backup retention-plan"}},
	{Command: "/bundles", Model: "gitclaw/skills", Category: "skills", Summary: "Catalog, list, search, inspect, risk-audit, or inspect git provenance for repo-local skill bundles that group existing skills into task profiles.", LocalCLI: []string{"gitclaw bundles catalog", "gitclaw bundles list", "gitclaw bundles risk", "gitclaw bundles provenance", "gitclaw bundles info <name>", "gitclaw bundles search <query>"}},
	{Command: "/channels", Aliases: []string{"/channel"}, Model: "gitclaw/channels", Category: "channels", Summary: "Audit channel bridge settings, workflow-dispatch ingress, outbound reply outboxes, provider contracts, risk posture, and queue outbound channel messages with `/channels send`.", LocalCLI: []string{"gitclaw channels verify", "gitclaw channels risk", "gitclaw channels list", "gitclaw channels info <provider>", "gitclaw channel-ingest", "gitclaw channel-send", "gitclaw channel-send --route <name>", "gitclaw channel-state", "gitclaw channel-gateway", "gitclaw channel-outbox", "gitclaw channel-delivery"}},
	{Command: "/checkpoints", Aliases: []string{"/checkpoint", "/rollback"}, Model: "gitclaw/checkpoints", Category: "checkpoint", Summary: "Catalog or inspect git rollback readiness, checkpoint metadata, diff-stat previews, and checkpoint risk posture without restoring files or printing diffs.", LocalCLI: []string{"gitclaw checkpoints catalog", "gitclaw checkpoints status", "gitclaw checkpoints list", "gitclaw checkpoints preview <ref>", "gitclaw checkpoints risk", "gitclaw checkpoints verify", "gitclaw rollback catalog", "gitclaw rollback diff <ref>", "gitclaw rollback list", "gitclaw rollback risk"}},
	{Command: "/config", Aliases: []string{"/configuration"}, Model: "gitclaw/config", Category: "control-plane", Summary: "Show or risk-audit effective config, labels, trust, budgets, gates, commands, and workflows.", LocalCLI: []string{"gitclaw config list", "gitclaw config risk"}},
	{Command: "/context", Model: "gitclaw/context", Category: "context", Summary: "Show loaded context documents, explicit context references, tool output metadata, or context risk posture.", LocalCLI: []string{"gitclaw context list", "gitclaw context risk", "gitclaw context info <path>"}},
	{Command: "/diffs", Aliases: []string{"/diff", "/changes"}, Model: "gitclaw/diffs", Category: "diffs", Summary: "Audit git working-tree changes, raw-patch boundaries, and diff risk posture by metadata without printing raw patches or file bodies.", LocalCLI: []string{"gitclaw diffs summary", "gitclaw diffs risk", "gitclaw diffs verify"}},
	{Command: "/doctor", Aliases: []string{"/health"}, Model: "gitclaw/doctor", Category: "health", Summary: "Run a body-free health check with skill, soul, and tool validation rollups.", LocalCLI: []string{"gitclaw doctor", "gitclaw doctor list"}},
	{Command: "/heartbeat", Model: "gitclaw/heartbeat", Category: "heartbeat", Summary: "Audit scheduled heartbeat workflow, context, permissions, idempotency, LLM runner contract, and heartbeat risk posture.", LocalCLI: []string{"gitclaw heartbeat status", "gitclaw heartbeat risk"}},
	{Command: "/hooks", Aliases: []string{"/hook"}, Model: "gitclaw/hooks", Category: "hooks", Summary: "Catalog or audit declarative hook policy, hook specs, ignored handlers, event-driven automation boundaries, hook risk posture, and hook git provenance.", LocalCLI: []string{"gitclaw hooks catalog", "gitclaw hooks list", "gitclaw hooks risk", "gitclaw hooks verify", "gitclaw hooks provenance"}},
	{Command: "/help", Aliases: []string{"/commands"}, Model: "gitclaw/commands", Category: "control-plane", Summary: "List deterministic GitClaw slash commands, aliases, and local CLI helpers.", LocalCLI: []string{"gitclaw commands"}},
	{Command: "/memory", Aliases: []string{"/memories"}, Model: "gitclaw/memory", Category: "memory", Summary: "Audit compact memory catalogs, fingerprints, long-term memory, body-free git provenance, timeline chronology, trust verification, risk posture, promotion plans, focused memory-file metadata, search, and memory hygiene findings.", LocalCLI: []string{"gitclaw memory catalog", "gitclaw memory snapshot", "gitclaw memory provenance", "gitclaw memory verify", "gitclaw memory risk", "gitclaw memory validate", "gitclaw memory timeline", "gitclaw memory list", "gitclaw memory promote-plan [target]", "gitclaw memory info <path>", "gitclaw memory search <query>"}},
	{Command: "/migrate", Aliases: []string{"/migration"}, Model: "gitclaw/migration", Category: "migration", Summary: "Preview or risk-audit OpenClaw, Hermes, Codex, or Claude state imports into GitClaw's repo-reviewed layout without applying them.", LocalCLI: []string{"gitclaw migrate plan <source>", "gitclaw migrate risk <source>"}},
	{Command: "/models", Aliases: []string{"/model"}, Model: "gitclaw/models", Category: "model", Summary: "Show, catalog-audit, usage-audit, cost-audit, or risk-audit model provider, endpoint, token source, default selection, fallback, retry, normalized token telemetry, and GitHub Models cost-estimation support.", LocalCLI: []string{"gitclaw models list", "gitclaw models catalog", "gitclaw models usage", "gitclaw models cost", "gitclaw models risk"}},
	{Command: "/nodes", Aliases: []string{"/node"}, Model: "gitclaw/nodes", Category: "nodes", Summary: "Catalog or audit repo-local node policy, node specs, GitHub Actions runtime boundaries, and node risk posture.", LocalCLI: []string{"gitclaw nodes catalog", "gitclaw nodes list", "gitclaw nodes risk", "gitclaw nodes verify"}},
	{Command: "/orders", Aliases: []string{"/standing-orders"}, Model: "gitclaw/orders", Category: "standing-orders", Summary: "Audit repo-reviewed standing orders, authority clauses, proactive enforcement metadata, and standing-order risk posture.", LocalCLI: []string{"gitclaw orders list", "gitclaw orders verify", "gitclaw orders risk"}},
	{Command: "/plugins", Aliases: []string{"/plugin"}, Model: "gitclaw/plugins", Category: "plugins", Summary: "Audit declarative plugin policy, plugin specs, MCP metadata specs, ignored package files, runtime extension boundaries, and plugin risk posture.", LocalCLI: []string{"gitclaw plugins list", "gitclaw plugins risk", "gitclaw plugins verify", "gitclaw plugins mcp", "gitclaw plugins mcp risk", "gitclaw plugins mcp provenance", "gitclaw plugins mcp info <name>"}},
	{Command: "/policy", Model: "gitclaw/policy", Category: "policy", Summary: "Show, verify, or risk-audit preflight, actor trust, write-intent, labels, workflow permissions, and write boundaries.", LocalCLI: []string{"gitclaw policy list", "gitclaw policy verify", "gitclaw policy risk"}},
	{Command: "/profile", Aliases: []string{"/profiles"}, Model: "gitclaw/profile", Category: "profile", Summary: "Catalog, show, verify, inspect git provenance for, search, compare git diffs for, fingerprint, dry-run package, or risk-audit the repo-local profile envelope across soul, memory, skills, tools, model, proactive, backup, session, and channel gates.", LocalCLI: []string{"gitclaw profile catalog", "gitclaw profile show", "gitclaw profile verify", "gitclaw profile provenance", "gitclaw profile search <query>", "gitclaw profile diff [base-ref]", "gitclaw profile snapshot", "gitclaw profile manifest", "gitclaw profile export-plan", "gitclaw profile risk"}},
	{Command: "/prompt", Aliases: []string{"/budget", "/prompt-budget"}, Model: "gitclaw/prompt", Category: "prompt", Summary: "Show prompt budget, packing, prompt-context manifests, cache-readiness, compression/compaction readiness, truncation, context, skill, tool-output metadata, and prompt risk posture.", LocalCLI: []string{"gitclaw prompt list", "gitclaw prompt pack", "gitclaw prompt context", "gitclaw prompt cache", "gitclaw prompt compression", "gitclaw prompt risk"}},
	{Command: "/proactive", Aliases: []string{"/cron"}, Model: "gitclaw/proactive", Category: "proactive", Summary: "Audit proactive prompt files, scheduled workflow calendars, context-from chains, reminder due gates, scheduled workflow metadata, and proactive risk posture.", LocalCLI: []string{"gitclaw proactive list", "gitclaw proactive schedule", "gitclaw proactive chain", "gitclaw proactive risk", "gitclaw proactive info <name>", "gitclaw proactive init", "gitclaw proactive enqueue"}},
	{Command: "/research", Aliases: []string{"/landscape"}, Model: "gitclaw/research", Category: "research", Summary: "Catalog OpenClaw/Hermes research sources, adopted and rejected design patterns, and repo-native coverage without fetching sources or printing research bodies.", LocalCLI: []string{"gitclaw research catalog", "gitclaw research sources", "gitclaw research coverage", "gitclaw research verify"}},
	{Command: "/runs", Aliases: []string{"/run", "/ledger"}, Model: "gitclaw/runs", Category: "run-ledger", Summary: "Show current turn/run provenance, body-free run history, labels, markers, and prompt-visible input hashes.", LocalCLI: []string{"gitclaw runs current", "gitclaw runs verify", "gitclaw runs history --backup <issue.json>"}},
	{Command: "/sandbox", Aliases: []string{"/sandboxes", "/exec-policy"}, Model: "gitclaw/sandbox", Category: "security", Summary: "Explain or risk-audit the GitHub Actions sandbox, host-exec policy, tool modes, and workflow permission boundary.", LocalCLI: []string{"gitclaw sandbox explain", "gitclaw sandbox verify", "gitclaw sandbox risk"}},
	{Command: "/security", Aliases: []string{"/sec"}, Model: "gitclaw/security", Category: "security", Summary: "Run an OpenClaw-style personal-assistant security audit across config, policy, sandbox, channels, tools, skills, plugins, and secrets without printing bodies or secret values.", LocalCLI: []string{"gitclaw security audit", "gitclaw security risk"}},
	{Command: "/secrets", Aliases: []string{"/secret"}, Model: "gitclaw/secrets", Category: "security", Summary: "Run a read-only repo secret audit or risk report without printing matched values.", LocalCLI: []string{"gitclaw secrets audit", "gitclaw secrets scan", "gitclaw secrets list", "gitclaw secrets risk"}},
	{Command: "/session", Model: "gitclaw/session", Category: "session", Summary: "Catalog, show, inspect prompt provenance for, audit tool-use, skill-use, token-usage, trajectory, compaction-readiness, and resume-readiness ledgers for, status-check, summarize, search, coverage-check, or risk-audit reconstructed transcript counts, markers, trust, provenance, and hashes.", LocalCLI: []string{"gitclaw session catalog", "gitclaw session list --backup <issue.json>", "gitclaw session provenance --backup <issue.json>", "gitclaw session tools --backup <issue.json>", "gitclaw session skills --backup <issue.json>", "gitclaw session usage --backup <issue.json>", "gitclaw session trajectory --backup <issue.json>", "gitclaw session compaction --backup <issue.json>", "gitclaw session resume --backup <issue.json>", "gitclaw session status --backup <issue.json>", "gitclaw session stats --backup <issue.json>", "gitclaw session coverage --backup <issue.json>", "gitclaw session risk --backup <issue.json>", "gitclaw session search <query> --backup <issue.json>"}},
	{Command: "/skills", Model: "gitclaw/skills", Category: "skills", Summary: "List local skill metadata, compact eligibility catalogs, body-free fingerprints, git provenance, trust verification, runtime metadata, risk audit, source pins, validation findings, selection/refresh/proposal/install/upgrade plans, or one focused skill info/search report.", LocalCLI: []string{"gitclaw skills verify", "gitclaw skills risk", "gitclaw skills runtime", "gitclaw skills catalog", "gitclaw skills snapshot", "gitclaw skills validate", "gitclaw skills check", "gitclaw skills list", "gitclaw skills provenance", "gitclaw skills select-plan <name>", "gitclaw skills refresh-plan", "gitclaw skills sources", "gitclaw skills sources verify", "gitclaw skills sources lock", "gitclaw skills sources update-plan", "gitclaw skills sources provenance", "gitclaw skills sources risk", "gitclaw skills sources info <name>", "gitclaw skills sources search <query>", "gitclaw skills proposals [risk]", "gitclaw skills proposal-plan <name>", "gitclaw skills install-plan <target>", "gitclaw skills upgrade-plan <target>", "gitclaw skills info <name>", "gitclaw skills search <query>"}},
	{Command: "/soul", Model: "gitclaw/soul", Category: "soul", Summary: "Audit, compact-catalog, map, fingerprint, verify, inspect git provenance for, plan edits for, risk-scan, or search high-authority context files and soul validation findings.", LocalCLI: []string{"gitclaw soul catalog", "gitclaw soul anchors", "gitclaw soul snapshot", "gitclaw soul provenance", "gitclaw soul verify", "gitclaw soul risk", "gitclaw soul validate", "gitclaw soul list", "gitclaw soul edit-plan <path>", "gitclaw soul info <path>", "gitclaw soul search <query>"}},
	{Command: "/tasks", Aliases: []string{"/task"}, Model: "gitclaw/tasks", Category: "tasks", Summary: "Audit GitHub issue-native task policy, task specs, labels, ledger, flow boundaries, and task risk posture.", LocalCLI: []string{"gitclaw tasks list", "gitclaw tasks risk", "gitclaw tasks verify", "gitclaw tasks ledger --backup <issue.json>"}},
	{Command: "/tools", Model: "gitclaw/tools", Category: "tools", Summary: "Audit compact tool catalogs, fingerprint, verify, risk-scan, inspect exposure, progressive disclosure, prompt boundaries, approval gates, and provenance, plan runs for, search, or inspect repo-reviewed toolsets for deterministic tool contracts.", LocalCLI: []string{"gitclaw tools catalog", "gitclaw tools snapshot", "gitclaw tools verify", "gitclaw tools risk", "gitclaw tools validate", "gitclaw tools list", "gitclaw tools exposure", "gitclaw tools exposure risk", "gitclaw tools defer-plan", "gitclaw tools boundary [query]", "gitclaw tools provenance [query]", "gitclaw tools toolsets", "gitclaw tools toolsets risk", "gitclaw tools toolsets provenance", "gitclaw tools toolsets info <name>", "gitclaw tools approval-plan <name>", "gitclaw tools run-plan <name>", "gitclaw tools info <name>", "gitclaw tools search <query>"}},
	{Command: "/workspace", Aliases: []string{"/workdir", "/repo"}, Model: "gitclaw/workspace", Category: "workspace", Summary: "Catalog or audit the GitHub Actions repository checkout, workspace policy, specs, isolation boundaries, and workspace risk posture.", LocalCLI: []string{"gitclaw workspace catalog", "gitclaw workspace summary", "gitclaw workspace risk", "gitclaw workspace verify"}},
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
	fmt.Fprintf(&b, "- llm_e2e_required_after_commands_report_change: `%t`\n", true)
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
