package gitclaw

import (
	"fmt"
	"strings"
	"time"
)

type memoryPromoteTarget struct {
	Requested string
	Kind      string
	Path      string
	Supported bool
}

type memoryPromotePlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func RenderMemoryPromotePlanCLIReport(cfg Config, repoContext RepoContext, target string) string {
	return renderMemoryPromotePlanReport(Event{ActiveText: "/memory promote-plan " + cleanMemoryPromoteTarget(target)}, cfg, repoContext, nil, target, false)
}

func renderMemoryPromotePlanReport(ev Event, cfg Config, repoContext RepoContext, transcript []TranscriptMessage, target string, includeIssue bool) string {
	targetInfo := normalizeMemoryPromoteTarget(target)
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	validation := ValidateMemory(cfg.Workdir, repoContext)
	findings := memoryPromotePlanFindings(targetInfo, validation)
	status := memoryPromotePlanStatus(findings)
	requestText := activeRequestText(ev)
	targetFile := memoryPromoteTargetFile(surface, targetInfo)
	remainingBytes := maxContextDocumentBytes - targetFile.Bytes
	if remainingBytes < 0 {
		remainingBytes = 0
	}

	var b strings.Builder
	b.WriteString("## GitClaw Memory Promote Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- memory_promote_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- requested_target_sha256_12: `%s`\n", shortDocumentHash(targetInfo.Requested))
	fmt.Fprintf(&b, "- request_text_sha256_12: `%s`\n", shortDocumentHash(requestText))
	fmt.Fprintf(&b, "- request_terms: `%d`\n", len(memorySearchTerms(requestText)))
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	sourceScope := "local-cli-request-metadata"
	if includeIssue {
		sourceScope = "issue-thread-transcript-metadata"
	}
	fmt.Fprintf(&b, "- source_scope: `%s`\n", sourceScope)
	fmt.Fprintf(&b, "- normalized_target_kind: `%s`\n", targetInfo.Kind)
	fmt.Fprintf(&b, "- normalized_target_path: `%s`\n", targetInfo.Path)
	fmt.Fprintf(&b, "- supported_target: `%t`\n", targetInfo.Supported)
	fmt.Fprintf(&b, "- target_present: `%t`\n", targetFile.Present)
	fmt.Fprintf(&b, "- target_bytes: `%d`\n", targetFile.Bytes)
	fmt.Fprintf(&b, "- target_lines: `%d`\n", targetFile.Lines)
	if targetFile.SHA != "" {
		fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", targetFile.SHA)
	}
	fmt.Fprintf(&b, "- memory_budget_bytes: `%d`\n", maxContextDocumentBytes)
	fmt.Fprintf(&b, "- memory_budget_remaining_bytes: `%d`\n", remainingBytes)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", len(surface.DatedNotes))
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", latestMemoryNotePath(surface.DatedNotes))
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_writes_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- candidate_generation_included: `%t`\n", false)
	fmt.Fprintf(&b, "- manual_review_required: `%t`\n", true)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	fmt.Fprintf(&b, "- raw_candidate_memory_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", false)
	writeMemoryValidationSummary(&b, validation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This is a promotion planner only. It identifies where reviewed memory could land, but it does not summarize the conversation, write memory files, call a model, or include transcript or memory bodies.\n\n")

	b.WriteString("### Target Memory File\n")
	writeConfigSurfaceFile(&b, targetFile)

	b.WriteString("\n### Promotion Boundaries\n")
	b.WriteString("- promote compact durable facts, preferences, project conventions, or action-sensitive constraints only after review\n")
	b.WriteString("- keep raw transcripts, logs, one-off debugging context, and easily rediscovered facts out of long-term memory\n")
	b.WriteString("- use `.gitclaw/memory/YYYY-MM-DD.md` for working notes and `.gitclaw/MEMORY.md` for compact always-loaded memory\n")
	b.WriteString("- route user-profile or communication-style changes through `/soul edit-plan user`, not this memory plan\n")

	b.WriteString("\n### Review Steps\n")
	if !targetInfo.Supported {
		b.WriteString("1. Choose `long-term` for `.gitclaw/MEMORY.md` or `daily-note` for `.gitclaw/memory/YYYY-MM-DD.md`.\n")
	} else {
		fmt.Fprintf(&b, "1. Draft a compact candidate outside this report for `%s`; keep source transcript bodies out of issue comments.\n", targetInfo.Path)
		b.WriteString("2. Check whether the candidate is durable, specific, and useful across future sessions.\n")
		b.WriteString("3. Apply the memory change as a reviewed git diff, then run `gitclaw memory validate`, `gitclaw memory verify`, and `gitclaw profile verify`.\n")
		b.WriteString("4. Run a live GitHub Models conversation E2E that performs an actual model call after the memory change.\n")
	}

	b.WriteString("\n### Findings\n")
	writeMemoryPromotePlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func isMemoryPromotePlanRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 &&
		(fields[0] == "/memory" || fields[0] == "/memories") &&
		(strings.EqualFold(fields[1], "promote-plan") || strings.EqualFold(fields[1], "promote") || strings.EqualFold(fields[1], "remember-plan"))
}

func requestedMemoryPromoteTarget(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || (fields[0] != "/memory" && fields[0] != "/memories") {
		return ""
	}
	if !strings.EqualFold(fields[1], "promote-plan") && !strings.EqualFold(fields[1], "promote") && !strings.EqualFold(fields[1], "remember-plan") {
		return ""
	}
	if len(fields) < 3 {
		return "long-term"
	}
	return cleanMemoryPromoteTarget(fields[2])
}

func cleanMemoryPromoteTarget(target string) string {
	return strings.Trim(strings.TrimSpace(target), " \t\r\n.,:;!?`\"'")
}

func normalizeMemoryPromoteTarget(target string) memoryPromoteTarget {
	requested := cleanMemoryPromoteTarget(target)
	if requested == "" {
		requested = "long-term"
	}
	normalized := strings.ToLower(requested)
	switch normalized {
	case "long-term", "longterm", "memory", "memory.md", "durable", "curated":
		return memoryPromoteTarget{Requested: requested, Kind: "long-term", Path: longTermMemoryPath, Supported: true}
	case "daily", "daily-note", "dated-note", "note", "working":
		return memoryPromoteTarget{Requested: requested, Kind: "dated-note", Path: ".gitclaw/memory/" + time.Now().UTC().Format("2006-01-02") + ".md", Supported: true}
	case "user", "user-profile", "profile":
		return memoryPromoteTarget{Requested: requested, Kind: "user-profile", Path: ".gitclaw/USER.md", Supported: false}
	default:
		return memoryPromoteTarget{Requested: requested, Kind: normalized, Path: "", Supported: false}
	}
}

func memoryPromoteTargetFile(surface memorySurface, target memoryPromoteTarget) configSurfaceFile {
	if target.Kind == "long-term" {
		return surface.LongTerm
	}
	for _, file := range surface.DatedNotes {
		if file.Path == target.Path {
			return file
		}
	}
	return configSurfaceFile{Path: target.Path}
}

func memoryPromotePlanFindings(target memoryPromoteTarget, validation MemoryValidationReport) []memoryPromotePlanFinding {
	var findings []memoryPromotePlanFinding
	add := func(severity, code, detail string) {
		findings = append(findings, memoryPromotePlanFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "durable_memory_is_prompt_authority", "promoted memory is prompt-visible context for future turns")
	add("info", "repository_mutation_disabled", "promotion planning does not create, update, delete, commit, push, or apply files")
	add("info", "body_suppression_enabled", "candidate memory, transcript bodies, and current memory bodies are not printed")
	if !target.Supported {
		if target.Kind == "user-profile" {
			add("error", "user_profile_requires_soul_plan", "user profile changes should use /soul edit-plan user")
		} else {
			add("error", "unsupported_memory_target", "supported promotion targets are long-term and daily-note")
		}
	} else {
		add("warning", "manual_review_required", "memory promotions must be reviewed as repository changes before they affect future context")
		if target.Kind == "long-term" {
			add("warning", "compact_memory_required", "long-term memory should contain compact durable facts, not raw transcript summaries")
		}
	}
	if validation.Errors > 0 {
		add("error", "memory_validation_errors_present", "fix current memory validation errors before promoting new memory")
	} else if validation.Warnings > 0 {
		add("warning", "memory_validation_warnings_present", "review current memory validation warnings before promoting new memory")
	}
	return findings
}

func memoryPromotePlanStatus(findings []memoryPromotePlanFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "blocked"
		}
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "needs_review"
		}
	}
	return "ok"
}

func writeMemoryPromotePlanFindings(b *strings.Builder, findings []memoryPromotePlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
