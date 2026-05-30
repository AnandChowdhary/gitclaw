package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const standingOrdersPath = ".gitclaw/STANDING_ORDERS.md"

type standingOrderSurface struct {
	Orders    configSurfaceFile
	Agents    configSurfaceFile
	Proactive proactiveSurface
	Programs  []standingOrderProgram
}

type standingOrderProgram struct {
	Index        int
	TitleSHA     string
	Lines        int
	Authority    bool
	Trigger      bool
	ApprovalGate bool
	Escalation   bool
}

type standingOrderFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsStandingOrdersReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/orders" || command == "/standing-orders"
}

func RenderStandingOrdersReport(ev Event, cfg Config) string {
	return renderStandingOrdersReport(ev, cfg, true)
}

func RenderStandingOrdersCLIReport(cfg Config) string {
	return renderStandingOrdersReport(Event{}, cfg, false)
}

func renderStandingOrdersReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectStandingOrderSurface(cfg.Workdir)
	findings := standingOrderFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Standing Orders Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- standing_orders_status: `%s`\n", standingOrderStatus(surface, findings))
	fmt.Fprintf(&b, "- standing_orders_path: `%s`\n", standingOrdersPath)
	fmt.Fprintf(&b, "- standing_orders_present: `%t`\n", surface.Orders.Present)
	fmt.Fprintf(&b, "- standing_orders_loaded_for_model: `%t`\n", standingOrdersLoadedForModel(surface))
	fmt.Fprintf(&b, "- agents_path: `%s`\n", "AGENTS.md")
	fmt.Fprintf(&b, "- agents_present: `%t`\n", surface.Agents.Present)
	fmt.Fprintf(&b, "- agents_mentions_standing_orders: `%t`\n", agentsMentionsStandingOrders(cfg.Workdir))
	fmt.Fprintf(&b, "- standing_order_programs: `%d`\n", len(surface.Programs))
	fmt.Fprintf(&b, "- programs_with_authority: `%d`\n", countStandingPrograms(surface.Programs, func(p standingOrderProgram) bool { return p.Authority }))
	fmt.Fprintf(&b, "- programs_with_trigger: `%d`\n", countStandingPrograms(surface.Programs, func(p standingOrderProgram) bool { return p.Trigger }))
	fmt.Fprintf(&b, "- programs_with_approval_gate: `%d`\n", countStandingPrograms(surface.Programs, func(p standingOrderProgram) bool { return p.ApprovalGate }))
	fmt.Fprintf(&b, "- programs_with_escalation: `%d`\n", countStandingPrograms(surface.Programs, func(p standingOrderProgram) bool { return p.Escalation }))
	fmt.Fprintf(&b, "- complete_programs: `%d`\n", countStandingPrograms(surface.Programs, standingOrderProgramComplete))
	fmt.Fprintf(&b, "- proactive_prompt_files: `%d`\n", len(surface.Proactive.Prompts))
	fmt.Fprintf(&b, "- proactive_workflow_present: `%t`\n", surface.Proactive.Workflow.Present)
	fmt.Fprintf(&b, "- proactive_schedule_trigger: `%t`\n", surface.Proactive.Workflow.Schedule)
	fmt.Fprintf(&b, "- enforcement_strategy: `%s`\n", "repo-reviewed-proactive-workflows-or-manual-trigger")
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_orders_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Standing orders are durable operating authority. GitClaw keeps them as reviewed repo files and audits structure without executing orders, opening issues, changing schedules, or printing raw order text.\n\n")

	b.WriteString("### Standing Orders File\n")
	writeConfigSurfaceFile(&b, surface.Orders)

	b.WriteString("\n### Bootstrap Linkage\n")
	writeConfigSurfaceFile(&b, surface.Agents)
	if standingOrdersPathInContext() {
		b.WriteString("- `.gitclaw/STANDING_ORDERS.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/STANDING_ORDERS.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Program Cards\n")
	if len(surface.Programs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, program := range surface.Programs {
			fmt.Fprintf(
				&b,
				"- program=`%02d` title_sha256_12=`%s` lines=`%d` authority=`%t` trigger=`%t` approval_gate=`%t` escalation=`%t` complete=`%t`\n",
				program.Index,
				program.TitleSHA,
				program.Lines,
				program.Authority,
				program.Trigger,
				program.ApprovalGate,
				program.Escalation,
				standingOrderProgramComplete(program),
			)
		}
	}

	b.WriteString("\n### Enforcement Surface\n")
	writeProactiveWorkflowInfo(&b, surface.Proactive.Workflow)
	if len(surface.Proactive.Prompts) == 0 {
		b.WriteString("- proactive_prompt_files=`0`\n")
	} else {
		for _, prompt := range surface.Proactive.Prompts {
			fmt.Fprintf(&b, "- proactive_prompt path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", prompt.Path, prompt.Bytes, prompt.Lines, prompt.SHA)
		}
	}

	b.WriteString("\n### Verification Findings\n")
	if len(findings) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, finding := range findings {
			fmt.Fprintf(&b, "- severity=`%s` code=`%s` subject=`%s` message=`%s`\n", finding.Severity, finding.Code, finding.Subject, finding.Message)
		}
	}

	return strings.TrimSpace(b.String())
}

func inspectStandingOrderSurface(root string) standingOrderSurface {
	if root == "" {
		root = "."
	}
	surface := standingOrderSurface{
		Orders:    inspectConfigSurfaceFile(root, standingOrdersPath),
		Agents:    inspectConfigSurfaceFile(root, "AGENTS.md"),
		Proactive: inspectProactiveSurface(root),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(standingOrdersPath)))
	if err != nil {
		return surface
	}
	surface.Programs = parseStandingOrderPrograms(string(body))
	return surface
}

func parseStandingOrderPrograms(text string) []standingOrderProgram {
	lines := strings.Split(text, "\n")
	var programs []standingOrderProgram
	currentStart := -1
	currentTitle := ""
	flush := func(end int) {
		if currentStart < 0 {
			return
		}
		block := strings.Join(lines[currentStart:end], "\n")
		program := standingOrderProgram{
			Index:    len(programs) + 1,
			TitleSHA: shortDocumentHash(currentTitle),
			Lines:    lineCount(block),
		}
		lower := strings.ToLower(block)
		program.Authority = strings.Contains(lower, "authority:")
		program.Trigger = strings.Contains(lower, "trigger:")
		program.ApprovalGate = strings.Contains(lower, "approval gate:")
		program.Escalation = strings.Contains(lower, "escalation:")
		programs = append(programs, program)
	}
	for i, line := range lines {
		title, ok := standingOrderProgramTitle(line)
		if !ok {
			continue
		}
		flush(i)
		currentStart = i
		currentTitle = title
	}
	flush(len(lines))
	return programs
}

func standingOrderProgramTitle(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(strings.ToLower(trimmed), "## program:") {
		return strings.TrimSpace(trimmed[len("## program:"):]), true
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "## standing order:") {
		return strings.TrimSpace(trimmed[len("## standing order:"):]), true
	}
	return "", false
}

func standingOrderFindings(surface standingOrderSurface) []standingOrderFinding {
	var findings []standingOrderFinding
	if !surface.Orders.Present {
		return append(findings, standingOrderFinding{"info", "standing_orders_not_configured", standingOrdersPath, "no standing orders file is configured"})
	}
	if surface.Orders.Lines == 0 {
		findings = append(findings, standingOrderFinding{"error", "standing_orders_empty", standingOrdersPath, "standing orders file is empty"})
	}
	if !standingOrdersPathInContext() {
		findings = append(findings, standingOrderFinding{"error", "standing_orders_not_loaded", standingOrdersPath, "standing orders file is not in the model context allowlist"})
	}
	if len(surface.Programs) == 0 {
		findings = append(findings, standingOrderFinding{"warning", "no_programs", standingOrdersPath, "expected at least one standing order program heading"})
	}
	for _, program := range surface.Programs {
		subject := fmt.Sprintf("program:%02d", program.Index)
		if !program.Authority {
			findings = append(findings, standingOrderFinding{"warning", "program_missing_authority", subject, "standing order should define authority"})
		}
		if !program.Trigger {
			findings = append(findings, standingOrderFinding{"warning", "program_missing_trigger", subject, "standing order should define trigger"})
		}
		if !program.ApprovalGate {
			findings = append(findings, standingOrderFinding{"warning", "program_missing_approval_gate", subject, "standing order should define approval gate"})
		}
		if !program.Escalation {
			findings = append(findings, standingOrderFinding{"warning", "program_missing_escalation", subject, "standing order should define escalation"})
		}
	}
	return findings
}

func standingOrderStatus(surface standingOrderSurface, findings []standingOrderFinding) string {
	if !surface.Orders.Present {
		return "not_configured"
	}
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "error"
		}
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "warning"
		}
	}
	return "ok"
}

func standingOrdersLoadedForModel(surface standingOrderSurface) bool {
	return surface.Orders.Present && standingOrdersPathInContext()
}

func standingOrdersPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == standingOrdersPath {
			return true
		}
	}
	return false
}

func agentsMentionsStandingOrders(root string) bool {
	if root == "" {
		root = "."
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "standing_orders") || strings.Contains(lower, "standing orders")
}

func countStandingPrograms(programs []standingOrderProgram, predicate func(standingOrderProgram) bool) int {
	count := 0
	for _, program := range programs {
		if predicate(program) {
			count++
		}
	}
	return count
}

func standingOrderProgramComplete(program standingOrderProgram) bool {
	return program.Authority && program.Trigger && program.ApprovalGate && program.Escalation
}
