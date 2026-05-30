package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

var requiredSoulDocumentPaths = []string{
	".gitclaw/SOUL.md",
	".gitclaw/IDENTITY.md",
	".gitclaw/USER.md",
	".gitclaw/TOOLS.md",
	".gitclaw/MEMORY.md",
	".gitclaw/HEARTBEAT.md",
}

type SoulValidationReport struct {
	Status                  string
	Errors                  int
	Warnings                int
	RequiredFiles           int
	PresentRequiredFiles    int
	MissingRequiredFiles    int
	MemoryNotes             int
	NoncanonicalMemoryNotes int
	Findings                []SoulValidationFinding
}

type SoulValidationFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func ValidateSoulContext(repoContext RepoContext) SoulValidationReport {
	report := SoulValidationReport{
		Status:        "ok",
		RequiredFiles: len(requiredSoulDocumentPaths),
	}
	byPath := map[string]ContextDocument{}
	for _, doc := range repoContext.Documents {
		byPath[doc.Path] = doc
		if isSoulMemoryNote(doc.Path) {
			report.MemoryNotes++
			if !datedMemoryNotePattern.MatchString(doc.Path) {
				report.NoncanonicalMemoryNotes++
				report.addFinding("warning", "noncanonical_memory_note", doc.Path, "memory note should use .gitclaw/memory/YYYY-MM-DD.md")
			}
			continue
		}
		if strings.TrimSpace(doc.Body) == "" {
			report.addFinding("error", "empty_context_file", doc.Path, "loaded high-authority context file is empty")
		}
		if len(doc.Body) >= maxContextDocumentBytes {
			report.addFinding("warning", "context_file_at_limit", doc.Path, fmt.Sprintf("loaded context reached the %d byte prompt limit", maxContextDocumentBytes))
		}
	}
	for _, path := range requiredSoulDocumentPaths {
		if _, ok := byPath[path]; !ok {
			report.MissingRequiredFiles++
			report.addFinding("error", "missing_required_context_file", path, "required GitClaw soul context file was not loaded")
			continue
		}
		report.PresentRequiredFiles++
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Severity != report.Findings[j].Severity {
			return report.Findings[i].Severity < report.Findings[j].Severity
		}
		if report.Findings[i].Code != report.Findings[j].Code {
			return report.Findings[i].Code < report.Findings[j].Code
		}
		return report.Findings[i].Path < report.Findings[j].Path
	})
	if report.Errors > 0 {
		report.Status = "error"
	} else if report.Warnings > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderSoulValidationReport(repoContext RepoContext) string {
	return renderSoulValidationReport(Event{}, repoContext, false)
}

func renderSoulValidationReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	validation := ValidateSoulContext(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Validate Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeSoulValidationSummary(&b, validation)
	b.WriteString("\nThis report validates high-authority context metadata without dumping soul, identity, user, memory, tool, heartbeat, issue, comment, prompt, or secret bodies.\n")
	b.WriteString("\n### Findings\n")
	writeSoulValidationFindings(&b, validation)
	return strings.TrimSpace(b.String())
}

func writeSoulValidationSummary(b *strings.Builder, validation SoulValidationReport) {
	fmt.Fprintf(b, "- soul_validation_status: `%s`\n", validation.Status)
	fmt.Fprintf(b, "- soul_validation_errors: `%d`\n", validation.Errors)
	fmt.Fprintf(b, "- soul_validation_warnings: `%d`\n", validation.Warnings)
	fmt.Fprintf(b, "- soul_required_files: `%d`\n", validation.RequiredFiles)
	fmt.Fprintf(b, "- soul_required_files_present: `%d`\n", validation.PresentRequiredFiles)
	fmt.Fprintf(b, "- soul_required_files_missing: `%d`\n", validation.MissingRequiredFiles)
	fmt.Fprintf(b, "- soul_memory_notes: `%d`\n", validation.MemoryNotes)
	fmt.Fprintf(b, "- soul_noncanonical_memory_notes: `%d`\n", validation.NoncanonicalMemoryNotes)
}

func writeSoulValidationFindings(b *strings.Builder, validation SoulValidationReport) {
	if len(validation.Findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range validation.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func (r *SoulValidationReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, SoulValidationFinding{
		Severity: severity,
		Code:     code,
		Path:     path,
		Detail:   detail,
	})
	switch severity {
	case "error":
		r.Errors++
	case "warning":
		r.Warnings++
	}
}
