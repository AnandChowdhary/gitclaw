package gitclaw

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	maxSecretScanFileBytes = 128000
	maxSecretFindings      = 20
	maxSecretReferences    = 20
)

type SecretAuditReport struct {
	Status             string
	FilesScanned       int
	FilesSkipped       int
	FindingsTotal      int
	FindingsReturned   int
	SecretReferences   int
	ReferencesReturned int
	RawValuesIncluded  bool
	RawLinesIncluded   bool
	Findings           []SecretFinding
	References         []SecretReference
}

type SecretFinding struct {
	Code     string
	Kind     string
	Severity string
	Path     string
	Line     int
	ValueSHA string
	LineSHA  string
}

type SecretReference struct {
	Syntax  string
	Path    string
	Line    int
	NameSHA string
	LineSHA string
}

type secretPattern struct {
	Code       string
	Kind       string
	Severity   string
	Expression *regexp.Regexp
	ValueGroup int
}

var secretPatterns = []secretPattern{
	{
		Code:       "github_token",
		Kind:       "known-token",
		Severity:   "high",
		Expression: regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9_]{20,})\b`),
		ValueGroup: 1,
	},
	{
		Code:       "github_pat",
		Kind:       "known-token",
		Severity:   "high",
		Expression: regexp.MustCompile(`\b(github_pat_[A-Za-z0-9_]{20,})\b`),
		ValueGroup: 1,
	},
	{
		Code:       "openai_key",
		Kind:       "known-token",
		Severity:   "high",
		Expression: regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{20,})\b`),
		ValueGroup: 1,
	},
	{
		Code:       "slack_token",
		Kind:       "known-token",
		Severity:   "high",
		Expression: regexp.MustCompile(`\b(xox[baprs]-[A-Za-z0-9-]{20,})\b`),
		ValueGroup: 1,
	},
	{
		Code:       "telegram_bot_token",
		Kind:       "known-token",
		Severity:   "high",
		Expression: regexp.MustCompile(`\b([0-9]{6,}:[A-Za-z0-9_-]{20,})\b`),
		ValueGroup: 1,
	},
	{
		Code:       "sensitive_assignment",
		Kind:       "plaintext-assignment",
		Severity:   "medium",
		Expression: regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|auth[_-]?token|token|secret|password|credential|authorization)\b\s*[:=]\s*["']?([A-Za-z0-9_./+=:-]{12,})`),
		ValueGroup: 2,
	},
}

var githubActionsSecretRefPattern = regexp.MustCompile(`\$\{\{\s*secrets\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

func IsSecretsReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/secrets" || command == "/secret"
}

func BuildSecretAuditReport(root string) (SecretAuditReport, error) {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return SecretAuditReport{}, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return SecretAuditReport{}, err
	}
	if !info.IsDir() {
		return SecretAuditReport{}, fmt.Errorf("workdir is not a directory: %s", root)
	}

	report := SecretAuditReport{
		Status:            "clean",
		RawValuesIncluded: false,
		RawLinesIncluded:  false,
	}
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			report.FilesSkipped++
			return nil
		}
		name := entry.Name()
		if entry.IsDir() && shouldSkipSecretScanDir(name) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if shouldSkipFile(name) {
			report.FilesSkipped++
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			report.FilesSkipped++
			return nil
		}
		rel = filepath.ToSlash(rel)
		body, err := readSecretScanTextFile(absRoot, rel)
		if err != nil {
			report.FilesSkipped++
			return nil
		}
		report.FilesScanned++
		scanSecretFile(&report, rel, body)
		return nil
	})
	if err != nil {
		return report, err
	}
	sortSecretAuditReport(&report)
	report.FindingsTotal = len(report.Findings)
	report.SecretReferences = len(report.References)
	if len(report.Findings) > maxSecretFindings {
		report.Findings = report.Findings[:maxSecretFindings]
	}
	if len(report.References) > maxSecretReferences {
		report.References = report.References[:maxSecretReferences]
	}
	report.FindingsReturned = len(report.Findings)
	report.ReferencesReturned = len(report.References)
	if report.FindingsTotal > 0 {
		report.Status = "findings"
	}
	return report, nil
}

func RenderSecretsReport(ev Event, report SecretAuditReport) string {
	return renderSecretsReport(ev, report, true)
}

func RenderSecretsCLIReport(report SecretAuditReport) string {
	return renderSecretsReport(Event{}, report, false)
}

func renderSecretsReport(ev Event, report SecretAuditReport, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Secrets Audit Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- secrets_audit_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- files_scanned: `%d`\n", report.FilesScanned)
	fmt.Fprintf(&b, "- files_skipped: `%d`\n", report.FilesSkipped)
	fmt.Fprintf(&b, "- findings_total: `%d`\n", report.FindingsTotal)
	fmt.Fprintf(&b, "- findings_returned: `%d`\n", report.FindingsReturned)
	fmt.Fprintf(&b, "- secret_references: `%d`\n", report.SecretReferences)
	fmt.Fprintf(&b, "- references_returned: `%d`\n", report.ReferencesReturned)
	fmt.Fprintf(&b, "- raw_values_included: `%t`\n", report.RawValuesIncluded)
	fmt.Fprintf(&b, "- raw_lines_included: `%t`\n", report.RawLinesIncluded)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report is a heuristic, read-only repo scan for plaintext secret residues and checked-in secret references. It never prints matched values, source lines, issue bodies, comments, prompts, or environment values.\n\n")

	b.WriteString("### Findings\n")
	if len(report.Findings) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, finding := range report.Findings {
			fmt.Fprintf(&b, "- code=`%s` kind=`%s` severity=`%s` path=`%s` line=`%d` value_sha256_12=`%s` line_sha256_12=`%s`\n",
				finding.Code,
				finding.Kind,
				finding.Severity,
				finding.Path,
				finding.Line,
				finding.ValueSHA,
				finding.LineSHA,
			)
		}
	}
	if report.FindingsTotal > report.FindingsReturned {
		fmt.Fprintf(&b, "- omitted_findings=`%d`\n", report.FindingsTotal-report.FindingsReturned)
	}

	b.WriteString("\n### Secret References\n")
	if len(report.References) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, ref := range report.References {
			fmt.Fprintf(&b, "- syntax=`%s` path=`%s` line=`%d` name_sha256_12=`%s` line_sha256_12=`%s`\n",
				ref.Syntax,
				ref.Path,
				ref.Line,
				ref.NameSHA,
				ref.LineSHA,
			)
		}
	}
	if report.SecretReferences > report.ReferencesReturned {
		fmt.Fprintf(&b, "- omitted_references=`%d`\n", report.SecretReferences-report.ReferencesReturned)
	}
	return strings.TrimSpace(b.String())
}

func scanSecretFile(report *SecretAuditReport, path, body string) {
	lines := strings.Split(body, "\n")
	seenFindings := map[string]bool{}
	seenRefs := map[string]bool{}
	for i, line := range lines {
		lineNumber := i + 1
		for _, pattern := range secretPatterns {
			matches := pattern.Expression.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if pattern.ValueGroup >= len(match) {
					continue
				}
				value := strings.TrimSpace(match[pattern.ValueGroup])
				if value == "" || ignoredSecretLikeValue(value) {
					continue
				}
				key := fmt.Sprintf("%s:%d:%s:%s", path, lineNumber, pattern.Code, shortDocumentHash(value))
				if seenFindings[key] {
					continue
				}
				seenFindings[key] = true
				report.Findings = append(report.Findings, SecretFinding{
					Code:     pattern.Code,
					Kind:     pattern.Kind,
					Severity: pattern.Severity,
					Path:     path,
					Line:     lineNumber,
					ValueSHA: shortDocumentHash(value),
					LineSHA:  shortDocumentHash(line),
				})
			}
		}
		refMatches := githubActionsSecretRefPattern.FindAllStringSubmatch(line, -1)
		for _, match := range refMatches {
			if len(match) < 2 {
				continue
			}
			name := strings.TrimSpace(match[1])
			key := fmt.Sprintf("%s:%d:%s", path, lineNumber, strings.ToLower(name))
			if seenRefs[key] {
				continue
			}
			seenRefs[key] = true
			report.References = append(report.References, SecretReference{
				Syntax:  "github-actions",
				Path:    path,
				Line:    lineNumber,
				NameSHA: shortDocumentHash(name),
				LineSHA: shortDocumentHash(line),
			})
		}
	}
}

func sortSecretAuditReport(report *SecretAuditReport) {
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Path != report.Findings[j].Path {
			return report.Findings[i].Path < report.Findings[j].Path
		}
		if report.Findings[i].Line != report.Findings[j].Line {
			return report.Findings[i].Line < report.Findings[j].Line
		}
		return report.Findings[i].Code < report.Findings[j].Code
	})
	sort.SliceStable(report.References, func(i, j int) bool {
		if report.References[i].Path != report.References[j].Path {
			return report.References[i].Path < report.References[j].Path
		}
		if report.References[i].Line != report.References[j].Line {
			return report.References[i].Line < report.References[j].Line
		}
		return report.References[i].NameSHA < report.References[j].NameSHA
	})
}

func ignoredSecretLikeValue(value string) bool {
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "${{") || strings.HasPrefix(value, "$(") || strings.HasPrefix(value, "${") {
		return true
	}
	lower := strings.ToLower(value)
	switch lower {
	case "true", "false", "null", "none", "example", "placeholder", "redacted":
		return true
	default:
		return false
	}
}

func readSecretScanTextFile(root, rel string) (string, error) {
	path, err := safeRepoPath(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("not a regular file: %s", rel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > maxSecretScanFileBytes {
		data = data[:maxSecretScanFileBytes]
	}
	if strings.ContainsRune(string(data), '\x00') {
		return "", fmt.Errorf("binary file: %s", rel)
	}
	return string(data), nil
}

func shouldSkipSecretScanDir(name string) bool {
	if shouldSkipDir(name) {
		return true
	}
	switch name {
	case ".idea", ".vscode":
		return true
	default:
		return false
	}
}
