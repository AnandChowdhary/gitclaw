package gitclaw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const backupRiskLargePayloadBytes = 1_000_000

type BackupRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Name     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type BackupRiskReport struct {
	Status                    string
	Repo                      string
	Root                      string
	RepoDir                   string
	IndexPath                 string
	ReadmePath                string
	SchemaVersion             int
	IndexGeneratedAt          string
	BackupVerifyStatus        string
	VerificationFailures      int
	IndexedIssues             int
	IssuesScanned             int
	IssuePayloadsScanned      int
	CommentsScanned           int
	TranscriptMessagesScanned int
	SurfacesWithRiskFindings  int
	Findings                  []BackupRiskFinding
	HighRiskFindings          int
	WarningRiskFindings       int
	InfoRiskFindings          int
	RawBackupPayloadsScanned  bool
	RawBodiesIncluded         bool
	LLME2ERequiredAfterChange bool
}

type backupRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
}

var backupRiskRules = []backupRiskRule{
	{
		Severity: "high",
		Code:     "credential_material_exposed",
		Category: "credential-handling",
		Any: []string{
			"github_token=",
			"github_pat_",
			"ghp_",
			"gho_",
			"ghu_",
			"ghs_",
			"sk-",
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
		Severity: "warning",
		Code:     "prompt_boundary_text",
		Category: "prompt-boundary",
		Any: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"disregard previous instructions",
			"override system instructions",
			"reveal the system prompt",
			"show the system prompt",
		},
	},
	{
		Severity: "warning",
		Code:     "restore_side_effect_instruction",
		Category: "restore-safety",
		Any: []string{
			"restore by posting raw",
			"replay secrets",
			"post all raw comments",
			"commit backup contents to main",
			"push backup contents to main",
		},
	},
}

func BuildBackupRisk(root, repo string) (BackupRiskReport, error) {
	if err := validateRepoName(repo); err != nil {
		return BackupRiskReport{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	repoDir := backupRepoDir(root, repo)
	report := BackupRiskReport{
		Status:                    "ok",
		Repo:                      repo,
		Root:                      filepath.ToSlash(root),
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		BackupVerifyStatus:        "ok",
		RawBackupPayloadsScanned:  true,
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}

	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupRiskReport{}, err
	}
	report.VerificationFailures = len(verify.VerificationFailures)
	if !verify.OK() {
		report.BackupVerifyStatus = "warn"
	}
	for _, failure := range verify.VerificationFailures {
		report.Findings = append(report.Findings, BackupRiskFinding{
			Severity: "high",
			Code:     "backup_verification_failure",
			Category: "integrity",
			Kind:     "verification",
			Name:     "backup-tree",
			Field:    "verify",
			Line:     0,
			LineSHA:  shortDocumentHash(failure),
		})
	}

	indexData, err := os.ReadFile(filepath.Join(repoDir, "index.json"))
	if err != nil {
		report.Findings = append(report.Findings, backupRiskMetadataFinding("high", "backup_index_unreadable", "integrity", "index", filepath.ToSlash(filepath.Join(repoDir, "index.json")), err.Error()))
		return finalizeBackupRiskReport(report), nil
	}
	var index BackupIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		report.Findings = append(report.Findings, backupRiskMetadataFinding("high", "backup_index_invalid_json", "integrity", "index", filepath.ToSlash(filepath.Join(repoDir, "index.json")), err.Error()))
		return finalizeBackupRiskReport(report), nil
	}
	report.SchemaVersion = index.Version
	report.IndexGeneratedAt = index.GeneratedAt
	report.IndexedIssues = len(index.Issues)

	for _, issue := range index.Issues {
		report.IssuesScanned++
		report.scanBackupPayload(repoDir, issue)
	}
	return finalizeBackupRiskReport(report), nil
}

func (r *BackupRiskReport) scanBackupPayload(repoDir string, issue BackupIndexIssue) {
	absPath, err := safeBackupPayloadPath(repoDir, issue.Path)
	if err != nil {
		r.Findings = append(r.Findings, backupRiskMetadataFinding("high", "backup_payload_path_unsafe", "path-safety", fmt.Sprintf("#%d", issue.Number), issue.Path, err.Error()))
		return
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		r.Findings = append(r.Findings, backupRiskMetadataFinding("high", "backup_payload_unreadable", "integrity", fmt.Sprintf("#%d", issue.Number), issue.Path, err.Error()))
		return
	}
	if len(data) > backupRiskLargePayloadBytes {
		r.Findings = append(r.Findings, BackupRiskFinding{
			Severity: "warning",
			Code:     "backup_payload_oversized",
			Category: "retention",
			Kind:     "payload",
			Name:     fmt.Sprintf("#%d", issue.Number),
			Path:     filepath.ToSlash(issue.Path),
			Field:    "bytes",
			Line:     0,
			LineSHA:  shortDocumentHash(fmt.Sprintf("%s:%d", issue.Path, len(data))),
		})
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		r.Findings = append(r.Findings, backupRiskMetadataFinding("high", "backup_payload_invalid_json", "integrity", fmt.Sprintf("#%d", issue.Number), issue.Path, err.Error()))
		return
	}
	r.IssuePayloadsScanned++
	name := fmt.Sprintf("#%d", backup.Issue.Number)
	r.Findings = append(r.Findings, scanBackupRiskText("payload", name, issue.Path, "issue.title", backup.Issue.Title)...)
	r.Findings = append(r.Findings, scanBackupRiskText("payload", name, issue.Path, "issue.body", backup.Issue.Body)...)
	for _, comment := range backup.Comments {
		r.CommentsScanned++
		commentName := fmt.Sprintf("#%d/comment:%d", backup.Issue.Number, comment.ID)
		r.Findings = append(r.Findings, scanBackupRiskText("comment", commentName, issue.Path, "body", comment.Body)...)
	}
	for i, message := range backup.Transcript {
		r.TranscriptMessagesScanned++
		messageName := fmt.Sprintf("#%d/transcript:%d", backup.Issue.Number, i+1)
		r.Findings = append(r.Findings, scanBackupRiskText("transcript", messageName, issue.Path, "body", message.Body)...)
	}
}

func backupRiskMetadataFinding(severity, code, category, name, path, detail string) BackupRiskFinding {
	return BackupRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "metadata",
		Name:     name,
		Path:     filepath.ToSlash(path),
		Field:    "metadata",
		Line:     0,
		LineSHA:  shortDocumentHash(detail),
	}
}

func scanBackupRiskText(kind, name, path, field, body string) []BackupRiskFinding {
	var findings []BackupRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range backupRiskRules {
			if !backupRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, BackupRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Kind:     kind,
				Name:     name,
				Path:     filepath.ToSlash(path),
				Field:    field,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortBackupRiskFindings(findings)
	return findings
}

func backupRiskRuleMatches(lowerLine string, rule backupRiskRule) bool {
	for _, phrase := range rule.Any {
		if strings.Contains(lowerLine, phrase) {
			return true
		}
	}
	return false
}

func finalizeBackupRiskReport(report BackupRiskReport) BackupRiskReport {
	sortBackupRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = backupRiskSurfaceCount(report.Findings)
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

func RenderBackupRiskReport(report BackupRiskReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeBackupRiskSummary(&b, report)
	b.WriteByte('\n')
	b.WriteString("This report scans a fetched `gitclaw-backups` tree for backup integrity and recovery risks. It reads raw backup payloads locally to compute findings, but reports only paths, counts, codes, severities, and hashes; raw issue bodies, comments, transcript messages, prompts, credentials, and secret values are not included.\n\n")
	b.WriteString("### Risk Findings\n")
	writeBackupRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeBackupRiskSummary(b *strings.Builder, report BackupRiskReport) {
	fmt.Fprintf(b, "- repository: `%s`\n", report.Repo)
	fmt.Fprintf(b, "- backup_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- backup_verify_status: `%s`\n", report.BackupVerifyStatus)
	fmt.Fprintf(b, "- verification_failures: `%d`\n", report.VerificationFailures)
	fmt.Fprintf(b, "- backup_root: `%s`\n", report.Root)
	fmt.Fprintf(b, "- repo_backup_dir: `%s`\n", report.RepoDir)
	fmt.Fprintf(b, "- index_path: `%s`\n", report.IndexPath)
	fmt.Fprintf(b, "- readme_path: `%s`\n", report.ReadmePath)
	fmt.Fprintf(b, "- backup_schema_version: `%d`\n", report.SchemaVersion)
	indexGeneratedAt := strings.TrimSpace(report.IndexGeneratedAt)
	if indexGeneratedAt == "" {
		indexGeneratedAt = "none"
	}
	fmt.Fprintf(b, "- index_generated_at: `%s`\n", indexGeneratedAt)
	fmt.Fprintf(b, "- indexed_issues: `%d`\n", report.IndexedIssues)
	fmt.Fprintf(b, "- issues_scanned: `%d`\n", report.IssuesScanned)
	fmt.Fprintf(b, "- issue_payloads_scanned: `%d`\n", report.IssuePayloadsScanned)
	fmt.Fprintf(b, "- comments_scanned: `%d`\n", report.CommentsScanned)
	fmt.Fprintf(b, "- transcript_messages_scanned: `%d`\n", report.TranscriptMessagesScanned)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- backup_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- raw_backup_payloads_scanned: `%t`\n", report.RawBackupPayloadsScanned)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_backup_risk_change: `%t`\n", report.LLME2ERequiredAfterChange)
}

func writeBackupRiskFindings(b *strings.Builder, findings []BackupRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` kind=`%s` name=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Kind,
			finding.Name,
			finding.Path,
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func backupRiskSurfaceCount(findings []BackupRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.Name + "\x00" + finding.Path
		if key == "\x00\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func backupRiskSeverityRank(severity string) int {
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

func sortBackupRiskFindings(findings []BackupRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if backupRiskSeverityRank(findings[i].Severity) != backupRiskSeverityRank(findings[j].Severity) {
			return backupRiskSeverityRank(findings[i].Severity) > backupRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Name != findings[j].Name {
			return findings[i].Name < findings[j].Name
		}
		if findings[i].Field != findings[j].Field {
			return findings[i].Field < findings[j].Field
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Code < findings[j].Code
	})
}
