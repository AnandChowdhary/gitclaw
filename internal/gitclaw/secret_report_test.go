package gitclaw

import (
	"strings"
	"testing"
)

func TestBuildSecretAuditReportFindsSecretsWithoutValues(t *testing.T) {
	root := t.TempDir()
	githubToken := "ghp_abcdefghijklmnopqrstuvwxyz123456"
	openAIKey := "sk-abcdefghijklmnopqrstuvwxyz123456"
	writeTestFile(t, root, ".gitclaw/config.yml", "trigger:\n  label: gitclaw\n")
	writeTestFile(t, root, ".github/workflows/example.yml", "env:\n  API_TOKEN: ${{ secrets.MY_API_TOKEN }}\n")
	writeTestFile(t, root, "config.env", "GITHUB_TOKEN="+githubToken+"\nOPENAI_API_KEY="+openAIKey+"\n")

	report, err := BuildSecretAuditReport(root)
	if err != nil {
		t.Fatalf("BuildSecretAuditReport returned error: %v", err)
	}
	if report.Status != "findings" {
		t.Fatalf("status = %q, want findings: %#v", report.Status, report)
	}
	if report.FilesScanned != 3 {
		t.Fatalf("files scanned = %d, want 3: %#v", report.FilesScanned, report)
	}
	if report.FindingsTotal < 2 || report.FindingsReturned < 2 {
		t.Fatalf("expected at least two findings: %#v", report)
	}
	if report.SecretReferences != 1 || report.ReferencesReturned != 1 {
		t.Fatalf("expected one GitHub Actions secret reference: %#v", report)
	}
	if !hasSecretFinding(report.Findings, "github_token", "config.env") {
		t.Fatalf("missing github_token finding: %#v", report.Findings)
	}
	if !hasSecretFinding(report.Findings, "openai_key", "config.env") {
		t.Fatalf("missing openai_key finding: %#v", report.Findings)
	}

	rendered := RenderSecretsCLIReport(report)
	for _, want := range []string{
		"GitClaw Secrets Audit Report",
		"scope: `local-cli`",
		"secrets_audit_status: `findings`",
		"raw_values_included: `false`",
		"raw_lines_included: `false`",
		"code=`github_token`",
		"code=`openai_key`",
		"syntax=`github-actions`",
		"value_sha256_12=",
		"name_sha256_12=",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("report missing %q:\n%s", want, rendered)
		}
	}
	for _, notWant := range []string{githubToken, openAIKey, "MY_API_TOKEN", "GITHUB_TOKEN=", "OPENAI_API_KEY="} {
		if strings.Contains(rendered, notWant) {
			t.Fatalf("report leaked %q:\n%s", notWant, rendered)
		}
	}
}

func TestBuildSecretAuditReportCleanRepo(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", "trigger:\n  label: gitclaw\n")
	writeTestFile(t, root, "README.md", "No secrets here.\n")

	report, err := BuildSecretAuditReport(root)
	if err != nil {
		t.Fatalf("BuildSecretAuditReport returned error: %v", err)
	}
	if report.Status != "clean" || report.FindingsTotal != 0 || report.SecretReferences != 0 {
		t.Fatalf("unexpected clean report: %#v", report)
	}
}

func hasSecretFinding(findings []SecretFinding, code, path string) bool {
	for _, finding := range findings {
		if finding.Code == code && finding.Path == path {
			return true
		}
	}
	return false
}
