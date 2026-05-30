package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type MemoryRiskFinding struct {
	Severity string
	Code     string
	Category string
	Path     string
	Line     int
	LineSHA  string
}

type MemoryRiskFile struct {
	Path              string
	Kind              string
	Source            string
	Present           bool
	Canonical         bool
	Latest            bool
	LoadedForThisTurn bool
	Bytes             int
	Lines             int
	SHA               string
	Findings          []MemoryRiskFinding
}

type MemoryRiskReport struct {
	Status                        string
	MemoryFiles                   int
	ScannedMemoryFiles            int
	MemoryFilesWithRiskFindings   int
	LongTermMemoryPresent         bool
	LongTermMemoryLoaded          bool
	DatedMemoryNotes              int
	CanonicalDatedMemoryNotes     int
	NoncanonicalDatedMemoryNotes  int
	LoadedMemoryNotes             int
	Findings                      []MemoryRiskFinding
	Files                         []MemoryRiskFile
	HighRiskFindings              int
	WarningRiskFindings           int
	InfoRiskFindings              int
	MemoryWritesAllowed           bool
	ExternalProviderVerification  string
	BackgroundPromotionReview     string
	RawBodiesIncluded             bool
	LLME2ERequiredAfterRiskChange bool
}

type memoryRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var memoryRiskRules = []memoryRiskRule{
	{
		Severity: "high",
		Code:     "prompt_boundary_override",
		Category: "prompt-boundary",
		Any: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"disregard previous instructions",
			"override system instructions",
			"reveal the system prompt",
			"show the system prompt",
			"developer message",
		},
	},
	{
		Severity: "high",
		Code:     "credential_material_in_memory",
		Category: "credential-handling",
		Any: []string{
			"github_token=",
			"github_pat_",
			"ghp_",
			"gho_",
			"ghu_",
			"ghs_",
			"api_key=",
			"private_key=",
			"telegram_bot_token=",
			"slack_bot_token=",
			"slack_app_token=",
			"xoxb-",
			"xapp-",
			"-----begin private key-----",
			"-----begin openssh private key-----",
		},
	},
	{
		Severity: "high",
		Code:     "hidden_persistence_instruction",
		Category: "persistent-state",
		Any: []string{
			"silently persist",
			"persist without review",
			"remember this secretly",
			"write to memory without review",
			"edit memory without review",
			"modify memory without review",
			"append to memory without review",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_automation_instruction",
		Category: "automation-amplification",
		Any: []string{
			"retry forever",
			"loop forever",
			"infinite loop",
			"while true",
			"never stop",
			"continue indefinitely",
			"every 2 minutes",
		},
	},
	{
		Severity: "warning",
		Code:     "unreviewed_host_execution",
		Category: "host-execution",
		Any: []string{
			"rm -rf",
			"bash -c",
			"python -c",
			"curl http",
			"wget http",
			"execute shell command",
		},
	},
	{
		Severity: "info",
		Code:     "credential_handling_note",
		Category: "credential-handling",
		Any: []string{
			"api key",
			"private key",
			"github token",
			"github_token",
		},
		All: []string{
			"memory",
		},
	},
}

func BuildMemoryRiskReport(root string, repoContext RepoContext) MemoryRiskReport {
	surface := inspectMemorySurface(root, repoContext)
	files := memorySearchFiles(surface)
	report := MemoryRiskReport{
		Status:                        "ok",
		MemoryFiles:                   len(files),
		LongTermMemoryPresent:         surface.LongTerm.Present,
		LongTermMemoryLoaded:          surface.LoadedLongTerm,
		DatedMemoryNotes:              len(surface.DatedNotes),
		CanonicalDatedMemoryNotes:     countCanonicalMemoryNotes(surface.DatedNotes),
		NoncanonicalDatedMemoryNotes:  countNoncanonicalMemoryNotes(surface.DatedNotes),
		LoadedMemoryNotes:             len(surface.LoadedNotePaths),
		MemoryWritesAllowed:           false,
		ExternalProviderVerification:  "not_configured",
		BackgroundPromotionReview:     "git_review_required",
		RawBodiesIncluded:             false,
		LLME2ERequiredAfterRiskChange: true,
	}
	loaded := loadedMemoryPathSet(surface)
	latest := latestMemoryNotePath(surface.DatedNotes)
	for _, file := range files {
		card := MemoryRiskFile{
			Path:              file.Path,
			Kind:              memoryFileKind(file.Path),
			Source:            memoryTrustSource(file.Path),
			Present:           file.Present,
			Canonical:         memoryFileCanonical(file.Path),
			Latest:            file.Path == latest,
			LoadedForThisTurn: loaded[file.Path],
			Bytes:             file.Bytes,
			Lines:             file.Lines,
			SHA:               file.SHA,
		}
		if !file.Present {
			report.Files = append(report.Files, card)
			continue
		}
		report.ScannedMemoryFiles++
		body, err := readRepoTextFile(rootOrDot(root), file.Path, maxContextDocumentBytes+1)
		if err != nil {
			card.Findings = append(card.Findings, MemoryRiskFinding{
				Severity: "warning",
				Code:     "memory_file_unreadable",
				Category: "memory-integrity",
				Path:     file.Path,
				Line:     0,
				LineSHA:  shortDocumentHash(err.Error()),
			})
		} else {
			card.Findings = append(card.Findings, scanMemoryRiskFindings(file.Path, body)...)
		}
		sortMemoryRiskFindings(card.Findings)
		if len(card.Findings) > 0 {
			report.MemoryFilesWithRiskFindings++
			report.Findings = append(report.Findings, card.Findings...)
		}
		report.Files = append(report.Files, card)
	}
	sortMemoryRiskFindings(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case report.HighRiskFindings > 0:
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "warn"
	default:
		report.Status = "ok"
	}
	return report
}

func scanMemoryRiskFindings(path, body string) []MemoryRiskFinding {
	var findings []MemoryRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range memoryRiskRules {
			if !memoryRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, MemoryRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Path:     path,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortMemoryRiskFindings(findings)
	return findings
}

func memoryRiskRuleMatches(lowerLine string, rule memoryRiskRule) bool {
	for _, required := range rule.All {
		if !strings.Contains(lowerLine, required) {
			return false
		}
	}
	if len(rule.Any) == 0 {
		return true
	}
	for _, phrase := range rule.Any {
		if strings.Contains(lowerLine, phrase) {
			return true
		}
	}
	return false
}

func RenderMemoryRiskReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderMemoryRiskReport(ev, cfg, repoContext, ev.Repo != "" || ev.Issue.Number != 0)
}

func RenderMemoryRiskCLIReport(cfg Config, repoContext RepoContext) string {
	return renderMemoryRiskReport(Event{}, cfg, repoContext, false)
}

func renderMemoryRiskReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildMemoryRiskReport(cfg.Workdir, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeMemoryRiskSummary(&b, report)
	b.WriteByte('\n')
	b.WriteString("This report scans repo-local memory files for body-free durable-state risk categories inspired by OpenClaw/Hermes memory and profile safety boundaries. It reports only paths, counts, categories, finding codes, and line hashes; raw memory bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Memory Risk Cards\n")
	if len(report.Files) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, file := range report.Files {
			writeMemoryRiskCard(&b, file)
		}
	}

	b.WriteString("\n### Risk Findings\n")
	writeMemoryRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeMemoryRiskSummary(b *strings.Builder, report MemoryRiskReport) {
	fmt.Fprintf(b, "- memory_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- memory_files: `%d`\n", report.MemoryFiles)
	fmt.Fprintf(b, "- scanned_memory_files: `%d`\n", report.ScannedMemoryFiles)
	fmt.Fprintf(b, "- memory_files_with_risk_findings: `%d`\n", report.MemoryFilesWithRiskFindings)
	fmt.Fprintf(b, "- long_term_memory_present: `%t`\n", report.LongTermMemoryPresent)
	fmt.Fprintf(b, "- long_term_memory_loaded: `%t`\n", report.LongTermMemoryLoaded)
	fmt.Fprintf(b, "- dated_memory_notes: `%d`\n", report.DatedMemoryNotes)
	fmt.Fprintf(b, "- canonical_dated_memory_notes: `%d`\n", report.CanonicalDatedMemoryNotes)
	fmt.Fprintf(b, "- noncanonical_dated_memory_notes: `%d`\n", report.NoncanonicalDatedMemoryNotes)
	fmt.Fprintf(b, "- loaded_memory_notes: `%d`\n", report.LoadedMemoryNotes)
	fmt.Fprintf(b, "- memory_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- memory_writes_allowed: `%t`\n", report.MemoryWritesAllowed)
	fmt.Fprintf(b, "- external_provider_verification: `%s`\n", report.ExternalProviderVerification)
	fmt.Fprintf(b, "- background_promotion_review: `%s`\n", report.BackgroundPromotionReview)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_memory_risk_change: `%t`\n", report.LLME2ERequiredAfterRiskChange)
}

func writeMemoryRiskCard(b *strings.Builder, file MemoryRiskFile) {
	fmt.Fprintf(
		b,
		"- kind=`%s` path=`%s` source=`%s` present=`%t` canonical=`%t` latest=`%t` loaded_for_this_turn=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		file.Kind,
		file.Path,
		file.Source,
		file.Present,
		file.Canonical,
		file.Latest,
		file.LoadedForThisTurn,
		file.Bytes,
		file.Lines,
		file.SHA,
		len(file.Findings),
		memoryRiskMaxSeverity(file.Findings),
		inlineListOrNone(memoryRiskCodes(file.Findings)),
		inlineListOrNone(memoryRiskLineHashes(file.Findings)),
	)
}

func writeMemoryRiskFindings(b *strings.Builder, findings []MemoryRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` path=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Path,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func memoryRiskCodes(findings []MemoryRiskFinding) []string {
	seen := map[string]bool{}
	var codes []string
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	sort.Strings(codes)
	return codes
}

func memoryRiskLineHashes(findings []MemoryRiskFinding) []string {
	seen := map[string]bool{}
	var hashes []string
	for _, finding := range findings {
		if finding.LineSHA == "" || seen[finding.LineSHA] {
			continue
		}
		seen[finding.LineSHA] = true
		hashes = append(hashes, finding.LineSHA)
	}
	sort.Strings(hashes)
	return hashes
}

func memoryRiskMaxSeverity(findings []MemoryRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if memoryRiskSeverityRank(finding.Severity) > memoryRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func memoryRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func sortMemoryRiskFindings(findings []MemoryRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if memoryRiskSeverityRank(findings[i].Severity) != memoryRiskSeverityRank(findings[j].Severity) {
			return memoryRiskSeverityRank(findings[i].Severity) > memoryRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Code < findings[j].Code
	})
}
