package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const hookPolicyPath = ".gitclaw/HOOKS.md"
const hookSpecsDir = ".gitclaw/hooks"

type hookSurface struct {
	Policy             configSurfaceFile
	Specs              []hookSpecCard
	ExecutableHandlers []configSurfaceFile
}

type hookSpecCard struct {
	Name             string
	Path             string
	Present          bool
	Bytes            int
	Lines            int
	SHA              string
	Frontmatter      bool
	Events           []string
	Mode             string
	Delivery         string
	RequiresApproval bool
}

type hookSpecFrontmatter struct {
	Name             string   `yaml:"name"`
	Events           []string `yaml:"events"`
	Mode             string   `yaml:"mode"`
	Delivery         string   `yaml:"delivery"`
	RequiresApproval bool     `yaml:"requires_approval"`
}

type hookFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsHookReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/hooks" || command == "/hook"
}

func RenderHookReport(ev Event, cfg Config) string {
	if isHookRiskRequest(ev, cfg) {
		return renderHookRiskReport(ev, cfg, true)
	}
	if isHookProvenanceRequest(ev, cfg) {
		return renderHookProvenanceReport(ev, cfg, true)
	}
	return renderHookReport(ev, cfg, true)
}

func RenderHookCLIReport(cfg Config) string {
	return renderHookReport(Event{}, cfg, false)
}

func RenderHookRiskCLIReport(cfg Config) string {
	return renderHookRiskReport(Event{}, cfg, false)
}

func renderHookReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectHookSurface(cfg.Workdir)
	findings := hookFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Hooks Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- hooks_status: `%s`\n", hookStatus(surface, findings))
	fmt.Fprintf(&b, "- hook_policy_path: `%s`\n", hookPolicyPath)
	fmt.Fprintf(&b, "- hook_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- hook_policy_loaded_for_model: `%t`\n", hookPolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- hook_specs_dir: `%s`\n", hookSpecsDir)
	fmt.Fprintf(&b, "- hook_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- hook_specs_with_frontmatter: `%d`\n", hookSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- hook_events: `%d`\n", hookEventCount(surface.Specs))
	fmt.Fprintf(&b, "- hook_specs_requiring_approval: `%d`\n", hookSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- hook_specs_audit_only: `%d`\n", hookSpecsAuditOnly(surface.Specs))
	fmt.Fprintf(&b, "- executable_handlers_present: `%d`\n", len(surface.ExecutableHandlers))
	fmt.Fprintf(&b, "- hook_execution_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- hook_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_hook_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Hooks describe event-driven automation boundaries. GitClaw treats hook specs as reviewed metadata only: no hook handler is executed, no workflow is dispatched, and no hook, issue, comment, or provider payload body is printed by this report.\n\n")

	b.WriteString("### Hook Policy\n")
	writeConfigSurfaceFile(&b, surface.Policy)
	if hookPolicyPathInContext() {
		b.WriteString("- `.gitclaw/HOOKS.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/HOOKS.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Hook Specs\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, spec := range surface.Specs {
			fmt.Fprintf(
				&b,
				"- name=`%s` path=`%s` bytes=`%d` lines=`%d` frontmatter=`%t` events=`%d` mode=`%s` delivery=`%s` requires_approval=`%t` sha256_12=`%s`\n",
				inlineCode(spec.Name),
				spec.Path,
				spec.Bytes,
				spec.Lines,
				spec.Frontmatter,
				len(spec.Events),
				inlineCode(spec.Mode),
				inlineCode(spec.Delivery),
				spec.RequiresApproval,
				spec.SHA,
			)
		}
	}

	b.WriteString("\n### Runtime Boundary\n")
	b.WriteString("- hook handlers are not executed by `gitclaw handle`\n")
	b.WriteString("- executable-looking hook files are reported as ignored metadata, not run\n")
	b.WriteString("- future executable hooks require explicit workflow permissions, approval gates, and body-free audit cards\n")
	b.WriteString("- GitHub-native hook effects should use reviewed workflows or `workflow_dispatch`, never untrusted issue text as code\n")

	b.WriteString("\n### Executable Handler Files\n")
	if len(surface.ExecutableHandlers) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, handler := range surface.ExecutableHandlers {
			writeConfigSurfaceFile(&b, handler)
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

func inspectHookSurface(root string) hookSurface {
	if root == "" {
		root = "."
	}
	surface := hookSurface{
		Policy: inspectConfigSurfaceFile(root, hookPolicyPath),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	surface.Specs = inspectHookSpecs(absRoot)
	surface.ExecutableHandlers = inspectHookExecutableHandlers(absRoot)
	return surface
}

func inspectHookSpecs(absRoot string) []hookSpecCard {
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "hooks", "*.md"))
	sort.Strings(matches)
	specs := make([]hookSpecCard, 0, len(matches))
	for _, match := range matches {
		body, err := os.ReadFile(match)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, match)
		if err != nil {
			continue
		}
		relPath := filepath.ToSlash(rel)
		text := string(body)
		spec := hookSpecCard{
			Name:    strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)),
			Path:    relPath,
			Present: true,
			Bytes:   len(body),
			Lines:   lineCount(text),
			SHA:     shortDocumentHash(text),
		}
		if meta, ok := parseHookFrontmatter(text); ok {
			spec.Frontmatter = true
			if strings.TrimSpace(meta.Name) != "" {
				spec.Name = strings.TrimSpace(meta.Name)
			}
			spec.Events = cleanHookEvents(meta.Events)
			spec.Mode = strings.TrimSpace(meta.Mode)
			spec.Delivery = strings.TrimSpace(meta.Delivery)
			spec.RequiresApproval = meta.RequiresApproval
		}
		specs = append(specs, spec)
	}
	return specs
}

func parseHookFrontmatter(text string) (hookSpecFrontmatter, bool) {
	var meta hookSpecFrontmatter
	if !strings.HasPrefix(text, "---\n") {
		return meta, false
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return meta, false
	}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(rest[:end])))
	decoder.KnownFields(true)
	if err := decoder.Decode(&meta); err != nil {
		return hookSpecFrontmatter{}, false
	}
	return meta, true
}

func inspectHookExecutableHandlers(absRoot string) []configSurfaceFile {
	root := filepath.Join(absRoot, ".gitclaw", "hooks")
	var handlers []configSurfaceFile
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}
		relPath := filepath.ToSlash(rel)
		if relPath == hookPolicyPath || strings.HasSuffix(relPath, ".md") {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		ext := strings.ToLower(filepath.Ext(path))
		if strings.HasPrefix(base, "handler.") || ext == ".ts" || ext == ".js" || ext == ".mjs" || ext == ".cjs" || ext == ".sh" || ext == ".py" {
			handlers = append(handlers, inspectConfigSurfaceFile(absRoot, relPath))
		}
		return nil
	})
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].Path < handlers[j].Path
	})
	return handlers
}

func hookFindings(surface hookSurface) []hookFinding {
	var findings []hookFinding
	if !surface.Policy.Present {
		findings = append(findings, hookFinding{"info", "hook_policy_not_configured", hookPolicyPath, "no hook policy file is configured"})
	}
	if surface.Policy.Present && !hookPolicyPathInContext() {
		findings = append(findings, hookFinding{"error", "hook_policy_not_loaded", hookPolicyPath, "hook policy file is not in the model context allowlist"})
	}
	if len(surface.ExecutableHandlers) > 0 {
		findings = append(findings, hookFinding{"warning", "executable_handlers_ignored", hookSpecsDir, "executable-looking hook files are present but ignored by GitClaw"})
	}
	for _, spec := range surface.Specs {
		if !spec.Frontmatter {
			findings = append(findings, hookFinding{"warning", "hook_frontmatter_missing", spec.Path, "hook spec should start with YAML frontmatter"})
		}
		if len(spec.Events) == 0 {
			findings = append(findings, hookFinding{"warning", "hook_events_missing", spec.Path, "hook spec should declare at least one event"})
		}
		if !strings.EqualFold(spec.Mode, "audit-only") {
			findings = append(findings, hookFinding{"warning", "hook_mode_not_audit_only", spec.Path, "GitClaw v1 only supports audit-only hook specs"})
		}
		if !spec.RequiresApproval {
			findings = append(findings, hookFinding{"warning", "hook_approval_gate_missing", spec.Path, "hook spec should require approval before side effects"})
		}
	}
	return findings
}

func hookStatus(surface hookSurface, findings []hookFinding) string {
	if !surface.Policy.Present && len(surface.Specs) == 0 {
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

func hookPolicyLoadedForModel(surface hookSurface) bool {
	return surface.Policy.Present && hookPolicyPathInContext()
}

func hookPolicyPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == hookPolicyPath {
			return true
		}
	}
	return false
}

func cleanHookEvents(events []string) []string {
	cleaned := make([]string, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		event = strings.TrimSpace(event)
		if event == "" || seen[event] {
			continue
		}
		seen[event] = true
		cleaned = append(cleaned, event)
	}
	sort.Strings(cleaned)
	return cleaned
}

func hookSpecsWithFrontmatter(specs []hookSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.Frontmatter {
			count++
		}
	}
	return count
}

func hookEventCount(specs []hookSpecCard) int {
	count := 0
	for _, spec := range specs {
		count += len(spec.Events)
	}
	return count
}

func hookSpecsRequiringApproval(specs []hookSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RequiresApproval {
			count++
		}
	}
	return count
}

func hookSpecsAuditOnly(specs []hookSpecCard) int {
	count := 0
	for _, spec := range specs {
		if strings.EqualFold(spec.Mode, "audit-only") {
			count++
		}
	}
	return count
}

func isHookRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/hooks" && command != "/hook" {
		return false
	}
	return strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit")
}

func isHookProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/hooks" && command != "/hook" {
		return false
	}
	return strings.EqualFold(fields[1], "provenance") ||
		strings.EqualFold(fields[1], "history") ||
		strings.EqualFold(fields[1], "timeline")
}
