package gitclaw

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type MemoryValidationReport struct {
	Status                  string
	Errors                  int
	Warnings                int
	LongTermPresent         bool
	LongTermLoaded          bool
	DatedNotes              int
	CanonicalDatedNotes     int
	NoncanonicalDatedNotes  int
	LoadedMemoryNotes       int
	EmptyMemoryFiles        int
	MemoryFilesAtLimit      int
	PotentialSecretFindings int
	Findings                []MemoryValidationFinding
}

type MemoryValidationFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

var memorySecretPatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{name: "private_key", pattern: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{name: "github_token", pattern: regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr|github_pat)_[A-Za-z0-9_]{20,}\b`)},
	{name: "openai_key", pattern: regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`)},
	{name: "slack_token", pattern: regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{20,}\b`)},
	{name: "aws_access_key", pattern: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{name: "assigned_secret", pattern: regexp.MustCompile(`(?i)\b(?:api[_-]?key|password|secret[_-]?key|access[_-]?token)\s*[:=]\s*['"]?[A-Za-z0-9_./+=-]{12,}`)},
}

func ValidateMemory(root string, repoContext RepoContext) MemoryValidationReport {
	surface := inspectMemorySurface(root, repoContext)
	report := MemoryValidationReport{
		Status:                 "ok",
		LongTermPresent:        surface.LongTerm.Present,
		LongTermLoaded:         surface.LoadedLongTerm,
		DatedNotes:             len(surface.DatedNotes),
		CanonicalDatedNotes:    countCanonicalMemoryNotes(surface.DatedNotes),
		NoncanonicalDatedNotes: countNoncanonicalMemoryNotes(surface.DatedNotes),
		LoadedMemoryNotes:      len(surface.LoadedNotePaths),
	}
	if !surface.LongTerm.Present {
		report.addFinding("error", "missing_long_term_memory", longTermMemoryPath, "expected .gitclaw/MEMORY.md to exist")
	} else {
		validateMemoryFile(root, surface.LongTerm, &report)
		if !surface.LoadedLongTerm {
			report.addFinding("warning", "long_term_memory_not_loaded", longTermMemoryPath, "long-term memory exists but was not loaded into repo context")
		}
	}
	for _, file := range surface.DatedNotes {
		if !datedMemoryNotePattern.MatchString(file.Path) {
			report.addFinding("warning", "noncanonical_memory_note", file.Path, "memory note should use .gitclaw/memory/YYYY-MM-DD.md")
		}
		validateMemoryFile(root, file, &report)
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

func validateMemoryFile(root string, file configSurfaceFile, report *MemoryValidationReport) {
	if !file.Present {
		return
	}
	if file.Lines == 0 || file.Bytes == 0 {
		report.EmptyMemoryFiles++
		report.addFinding("error", "empty_memory_file", file.Path, "memory file is empty")
		return
	}
	if file.Bytes >= maxContextDocumentBytes {
		report.MemoryFilesAtLimit++
		report.addFinding("warning", "memory_file_at_limit", file.Path, fmt.Sprintf("memory file is at or above the %d byte context limit", maxContextDocumentBytes))
	}
	body, err := readRepoTextFile(root, file.Path, maxContextDocumentBytes+1)
	if err != nil {
		report.addFinding("warning", "memory_file_unreadable", file.Path, "memory file could not be scanned as bounded text")
		return
	}
	for _, secretPattern := range memorySecretPatterns {
		if !secretPattern.pattern.MatchString(body) {
			continue
		}
		report.PotentialSecretFindings++
		report.addFinding("error", "potential_secret", file.Path, "memory file matched secret-like pattern "+secretPattern.name)
	}
}

func RenderMemoryValidationReport(ev Event, cfg Config, repoContext RepoContext) string {
	validation := ValidateMemory(cfg.Workdir, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Validate Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if ev.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	}
	if ev.Issue.Number != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeMemoryValidationSummary(&b, validation)
	b.WriteString("\nThis report validates git-backed memory hygiene without dumping memory bodies, issue bodies, comments, prompts, or secret values.\n\n")
	b.WriteString("### Findings\n")
	writeMemoryValidationFindings(&b, validation)
	return strings.TrimSpace(b.String())
}

func writeMemoryValidationSummary(b *strings.Builder, validation MemoryValidationReport) {
	fmt.Fprintf(b, "- memory_validation_status: `%s`\n", validation.Status)
	fmt.Fprintf(b, "- memory_validation_errors: `%d`\n", validation.Errors)
	fmt.Fprintf(b, "- memory_validation_warnings: `%d`\n", validation.Warnings)
	fmt.Fprintf(b, "- long_term_memory_present: `%t`\n", validation.LongTermPresent)
	fmt.Fprintf(b, "- long_term_memory_loaded: `%t`\n", validation.LongTermLoaded)
	fmt.Fprintf(b, "- dated_memory_notes: `%d`\n", validation.DatedNotes)
	fmt.Fprintf(b, "- canonical_dated_memory_notes: `%d`\n", validation.CanonicalDatedNotes)
	fmt.Fprintf(b, "- noncanonical_dated_memory_notes: `%d`\n", validation.NoncanonicalDatedNotes)
	fmt.Fprintf(b, "- loaded_memory_notes: `%d`\n", validation.LoadedMemoryNotes)
	fmt.Fprintf(b, "- max_loaded_memory_notes: `%d`\n", maxMemoryDocuments)
	fmt.Fprintf(b, "- empty_memory_files: `%d`\n", validation.EmptyMemoryFiles)
	fmt.Fprintf(b, "- memory_files_at_limit: `%d`\n", validation.MemoryFilesAtLimit)
	fmt.Fprintf(b, "- potential_secret_findings: `%d`\n", validation.PotentialSecretFindings)
}

func writeMemoryValidationFindings(b *strings.Builder, validation MemoryValidationReport) {
	if len(validation.Findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range validation.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func (r *MemoryValidationReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, MemoryValidationFinding{
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
