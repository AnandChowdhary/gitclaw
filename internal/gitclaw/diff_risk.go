package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type DiffRiskFinding struct {
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

type DiffRiskReport struct {
	Status                            string
	VerificationScope                 string
	DiffPolicyPresent                 bool
	DiffPolicyLoadedForModel          bool
	DiffSpecs                         int
	ScannedDiffSpecs                  int
	DiffSpecsRequiringApproval        int
	DiffSpecsDisallowingRawPatch      int
	DiffMaxFilesDeclared              int
	GitAvailable                      bool
	GitRepository                     bool
	WorktreeClean                     bool
	ChangedFiles                      int
	StagedFiles                       int
	UnstagedFiles                     int
	UntrackedFiles                    int
	RenamedFiles                      int
	DeletedFiles                      int
	BinaryDiffFiles                   int
	DiffFileLimit                     int
	DiffFilesReturned                 int
	SurfacesWithRiskFindings          int
	Findings                          []DiffRiskFinding
	HighRiskFindings                  int
	WarningRiskFindings               int
	InfoRiskFindings                  int
	RawDiffsIncluded                  bool
	RawFileBodiesIncluded             bool
	RawIssueBodiesIncluded            bool
	RawCommentBodiesIncluded          bool
	CredentialValuesIncluded          bool
	PatchApplicationAllowed           bool
	RepositoryMutationAllowed         bool
	DiffAsHiddenStateAllowed          bool
	ExternalDiffStorageAllowed        bool
	LLME2ERequiredAfterDiffRiskChange bool
}

type diffRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var diffTextRiskRules = []diffRiskRule{
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
		Code:     "credential_material_in_diff",
		Category: "credential-handling",
		Any: []string{
			"github_token=",
			"github_pat_",
			"ghp_",
			"gho_",
			"ghu_",
			"ghs_",
			"telegram_bot_token=",
			"slack_bot_token=",
			"slack_app_token=",
			"xoxb-",
			"xapp-",
			"api_key=",
			"private_key=",
			"-----begin private key-----",
			"-----begin openssh private key-----",
		},
	},
	{
		Severity: "high",
		Code:     "raw_patch_leakage_enabled",
		Category: "body-leakage",
		Any: []string{
			"raw_patch_allowed: true",
			"print raw diff",
			"print raw patch",
			"include unified patch",
			"include raw patch",
			"include file bodies",
			"include full file body",
			"git diff --patch",
			"git diff --",
			"git show --patch",
		},
		IgnoreAny: []string{
			"do not print raw",
			"keep raw unified patches out",
			"raw_patch_allowed: false",
			"must not print",
		},
	},
	{
		Severity: "high",
		Code:     "destructive_diff_action",
		Category: "write-authority",
		Any: []string{
			"git reset --hard",
			"git checkout --",
			"git restore --source",
			"git clean -fd",
			"git apply",
			"git commit",
			"git push",
			"gh pr create",
			"apply patch automatically",
			"restore files automatically",
		},
		IgnoreAny: []string{
			"do not git reset",
			"do not git checkout",
			"do not git restore",
			"do not git clean",
			"do not apply",
			"do not commit",
			"do not push",
		},
	},
	{
		Severity: "high",
		Code:     "diff_hidden_state",
		Category: "state-boundary",
		Any: []string{
			"use diffs as hidden state",
			"use diff as hidden state",
			"diffs are durable memory",
			"diff artifact is durable memory",
			"load prior diff as context",
			"load diff next turn",
			"diff transcript is source of truth",
			"hidden state channel",
		},
		IgnoreAny: []string{
			"do not turn",
			"not a hidden state channel",
		},
	},
	{
		Severity: "high",
		Code:     "untracked_file_body_context",
		Category: "context-boundary",
		Any: []string{
			"treat untracked file contents as prompt-visible",
			"read all untracked files into the prompt",
			"include untracked file bodies",
			"dump untracked file contents",
		},
		IgnoreAny: []string{
			"do not treat untracked",
			"must not treat untracked",
		},
	},
	{
		Severity: "warning",
		Code:     "external_diff_storage",
		Category: "storage-boundary",
		Any: []string{
			"storage: s3",
			"storage: gcs",
			"storage: supabase",
			"s3://",
			"gs://",
			"aws s3 cp",
			"curl -t ",
			"curl --upload-file",
			"scp ",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_diff_collection",
		Category: "runtime-amplification",
		Any: []string{
			"max_files: 0",
			"max_files: -1",
			"while true",
			"retry forever",
			"loop forever",
			"sleep infinity",
			"never stop",
			"continue indefinitely",
		},
	},
}

func renderDiffRiskReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectDiffSurface(cfg.Workdir)
	report := BuildDiffRiskReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Diff Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeDiffRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans diff policy, diff specs, and git worktree metadata for prompt-boundary, credential, raw patch leakage, destructive git action, hidden-state, untracked-file body, external storage, approval, and unbounded-collection risks. It reports metadata, paths, risk codes, severities, and hashes only; raw patches, file bodies, issue bodies, comments, prompts, tool outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Diff Policy Risk Card\n")
	writeDiffPolicyRiskCard(&b, cfg.Workdir, surface.Policy)

	b.WriteString("\n### Diff Spec Risk Cards\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- kind=`diff-spec` none\n")
	} else {
		for _, spec := range surface.Specs {
			writeDiffSpecRiskCard(&b, cfg.Workdir, spec)
		}
	}

	b.WriteString("\n### Git Worktree Risk Card\n")
	writeDiffGitRiskCard(&b, surface.Git)

	b.WriteString("\n### Changed File Risk Cards\n")
	if len(surface.Git.Files) == 0 {
		b.WriteString("- kind=`changed-file` none\n")
	} else {
		for _, file := range surface.Git.Files {
			writeDiffFileRiskCard(&b, file)
		}
	}

	b.WriteString("\n### Current Diff Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-diff-request` current_issue_diff_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-diff-request` scope=`local-cli` current_issue_diff_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeDiffRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildDiffRiskReport(cfg Config) DiffRiskReport {
	surface := inspectDiffSurface(cfg.Workdir)
	report := DiffRiskReport{
		Status:                            "ok",
		VerificationScope:                 "git_worktree_diff_metadata",
		DiffPolicyPresent:                 surface.Policy.Present,
		DiffPolicyLoadedForModel:          diffPolicyLoadedForModel(surface),
		DiffSpecs:                         len(surface.Specs),
		DiffSpecsRequiringApproval:        diffSpecsRequiringApproval(surface.Specs),
		DiffSpecsDisallowingRawPatch:      diffSpecsDisallowingRawPatch(surface.Specs),
		DiffMaxFilesDeclared:              diffMaxFilesDeclared(surface.Specs),
		GitAvailable:                      surface.Git.GitAvailable,
		GitRepository:                     surface.Git.GitRepository,
		WorktreeClean:                     surface.Git.WorktreeClean,
		ChangedFiles:                      surface.Git.ChangedFiles,
		StagedFiles:                       surface.Git.StagedFiles,
		UnstagedFiles:                     surface.Git.UnstagedFiles,
		UntrackedFiles:                    surface.Git.UntrackedFiles,
		RenamedFiles:                      surface.Git.RenamedFiles,
		DeletedFiles:                      surface.Git.DeletedFiles,
		BinaryDiffFiles:                   surface.Git.BinaryDiffFiles,
		DiffFileLimit:                     maxDiffFilesReturned,
		DiffFilesReturned:                 surface.Git.FilesReturned,
		RawDiffsIncluded:                  false,
		RawFileBodiesIncluded:             false,
		RawIssueBodiesIncluded:            false,
		RawCommentBodiesIncluded:          false,
		CredentialValuesIncluded:          false,
		PatchApplicationAllowed:           false,
		RepositoryMutationAllowed:         false,
		DiffAsHiddenStateAllowed:          false,
		ExternalDiffStorageAllowed:        false,
		LLME2ERequiredAfterDiffRiskChange: true,
	}
	report.Findings = append(report.Findings, scanDiffPolicyRiskFindings(cfg.Workdir, surface.Policy)...)
	for _, spec := range surface.Specs {
		report.ScannedDiffSpecs++
		report.Findings = append(report.Findings, scanDiffSpecRiskFindings(cfg.Workdir, spec)...)
	}
	report.Findings = append(report.Findings, scanDiffGitRiskFindings(surface.Git)...)
	sortDiffRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = diffRiskSurfaceCount(report.Findings)
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

func writeDiffRiskSummary(b *strings.Builder, report DiffRiskReport) {
	fmt.Fprintf(b, "- diff_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- diff_policy_present: `%t`\n", report.DiffPolicyPresent)
	fmt.Fprintf(b, "- diff_policy_loaded_for_model: `%t`\n", report.DiffPolicyLoadedForModel)
	fmt.Fprintf(b, "- diff_specs: `%d`\n", report.DiffSpecs)
	fmt.Fprintf(b, "- scanned_diff_specs: `%d`\n", report.ScannedDiffSpecs)
	fmt.Fprintf(b, "- diff_specs_requiring_approval: `%d`\n", report.DiffSpecsRequiringApproval)
	fmt.Fprintf(b, "- diff_specs_disallowing_raw_patch: `%d`\n", report.DiffSpecsDisallowingRawPatch)
	fmt.Fprintf(b, "- diff_max_files_declared: `%d`\n", report.DiffMaxFilesDeclared)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_repository: `%t`\n", report.GitRepository)
	fmt.Fprintf(b, "- worktree_clean: `%t`\n", report.WorktreeClean)
	fmt.Fprintf(b, "- changed_files: `%d`\n", report.ChangedFiles)
	fmt.Fprintf(b, "- staged_files: `%d`\n", report.StagedFiles)
	fmt.Fprintf(b, "- unstaged_files: `%d`\n", report.UnstagedFiles)
	fmt.Fprintf(b, "- untracked_files: `%d`\n", report.UntrackedFiles)
	fmt.Fprintf(b, "- renamed_files: `%d`\n", report.RenamedFiles)
	fmt.Fprintf(b, "- deleted_files: `%d`\n", report.DeletedFiles)
	fmt.Fprintf(b, "- binary_diff_files: `%d`\n", report.BinaryDiffFiles)
	fmt.Fprintf(b, "- diff_file_limit: `%d`\n", report.DiffFileLimit)
	fmt.Fprintf(b, "- diff_files_returned: `%d`\n", report.DiffFilesReturned)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- diff_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- raw_diffs_included: `%t`\n", report.RawDiffsIncluded)
	fmt.Fprintf(b, "- raw_file_bodies_included: `%t`\n", report.RawFileBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- patch_application_allowed: `%t`\n", report.PatchApplicationAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- diff_as_hidden_state_allowed: `%t`\n", report.DiffAsHiddenStateAllowed)
	fmt.Fprintf(b, "- external_diff_storage_allowed: `%t`\n", report.ExternalDiffStorageAllowed)
	fmt.Fprintf(b, "- llm_e2e_required_after_diff_risk_change: `%t`\n", report.LLME2ERequiredAfterDiffRiskChange)
}

func writeDiffPolicyRiskCard(b *strings.Builder, root string, policy configSurfaceFile) {
	findings := scanDiffPolicyRiskFindings(root, policy)
	if !policy.Present {
		fmt.Fprintf(
			b,
			"- kind=`diff-policy` path=`%s` present=`false` loaded_for_model=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			policy.Path,
			len(findings),
			diffRiskMaxSeverity(findings),
			inlineListOrNone(diffRiskCodes(findings)),
			inlineListOrNone(diffRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`diff-policy` path=`%s` present=`true` loaded_for_model=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		policy.Path,
		diffPolicyPathInContext(),
		policy.Bytes,
		policy.Lines,
		policy.SHA,
		len(findings),
		diffRiskMaxSeverity(findings),
		inlineListOrNone(diffRiskCodes(findings)),
		inlineListOrNone(diffRiskLineHashes(findings)),
	)
}

func writeDiffSpecRiskCard(b *strings.Builder, root string, spec diffSpecCard) {
	findings := scanDiffSpecRiskFindings(root, spec)
	fmt.Fprintf(
		b,
		"- kind=`diff-spec` name=`%s` path=`%s` frontmatter=`%t` diff_kind=`%s` source=`%s` mode=`%s` max_files=`%d` raw_patch_allowed=`%t` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(spec.Name),
		spec.Path,
		spec.Frontmatter,
		inlineCode(spec.Kind),
		inlineCode(spec.Source),
		inlineCode(spec.Mode),
		spec.MaxFiles,
		spec.RawPatchAllowed,
		spec.RequiresApproval,
		spec.Bytes,
		spec.Lines,
		spec.SHA,
		len(findings),
		diffRiskMaxSeverity(findings),
		inlineListOrNone(diffRiskCodes(findings)),
		inlineListOrNone(diffRiskLineHashes(findings)),
	)
}

func writeDiffGitRiskCard(b *strings.Builder, git diffGitSurface) {
	findings := scanDiffGitRiskFindings(git)
	fmt.Fprintf(
		b,
		"- kind=`git-worktree` root=`%s` git_available=`%t` git_repository=`%t` branch=`%s` head_commit=`%s` worktree_clean=`%t` changed_files=`%d` staged_files=`%d` unstaged_files=`%d` untracked_files=`%d` renamed_files=`%d` deleted_files=`%d` binary_diff_files=`%d` diff_file_limit=`%d` diff_files_returned=`%d` raw_diffs_included=`%t` raw_file_bodies_included=`%t` error_reason=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(git.Root),
		git.GitAvailable,
		git.GitRepository,
		inlineCode(git.Branch),
		git.HeadShortSHA,
		git.WorktreeClean,
		git.ChangedFiles,
		git.StagedFiles,
		git.UnstagedFiles,
		git.UntrackedFiles,
		git.RenamedFiles,
		git.DeletedFiles,
		git.BinaryDiffFiles,
		maxDiffFilesReturned,
		git.FilesReturned,
		git.RawDiffsIncluded,
		git.RawFileBodiesIncluded,
		inlineCode(git.ErrorReason),
		len(findings),
		diffRiskMaxSeverity(findings),
		inlineListOrNone(diffRiskCodes(findings)),
		inlineListOrNone(diffRiskLineHashes(findings)),
	)
}

func writeDiffFileRiskCard(b *strings.Builder, file diffFileCard) {
	fmt.Fprintf(
		b,
		"- kind=`changed-file` path=`%s` status=`%s` staged=`%t` unstaged=`%t` untracked=`%t` renamed=`%t` deleted=`%t` binary=`%t` path_sha256_12=`%s` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n",
		inlineCode(file.Path),
		inlineCode(file.Status),
		file.Staged,
		file.Unstaged,
		file.Untracked,
		file.Renamed,
		file.Deleted,
		file.Binary,
		file.PathSHA,
	)
}

func scanDiffPolicyRiskFindings(root string, policy configSurfaceFile) []DiffRiskFinding {
	var findings []DiffRiskFinding
	if !policy.Present {
		findings = append(findings, DiffRiskFinding{
			Severity: "info",
			Code:     "diff_policy_not_configured",
			Category: "policy",
			Kind:     "diff-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "present",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":present"),
		})
		return findings
	}
	if !diffPolicyPathInContext() {
		findings = append(findings, DiffRiskFinding{
			Severity: "high",
			Code:     "diff_policy_not_loaded",
			Category: "context",
			Kind:     "diff-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "loaded_for_model",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":loaded_for_model"),
		})
	}
	findings = append(findings, scanDiffRiskText("diff-policy", "policy", policy.Path, "body", readDiffRiskBody(root, policy.Path))...)
	sortDiffRiskFindings(findings)
	return findings
}

func scanDiffSpecRiskFindings(root string, spec diffSpecCard) []DiffRiskFinding {
	var findings []DiffRiskFinding
	if !spec.Frontmatter {
		findings = append(findings, diffSpecMetadataRiskFinding("warning", "diff_frontmatter_missing", "metadata", spec, "frontmatter"))
	}
	if strings.TrimSpace(spec.Kind) == "" {
		findings = append(findings, diffSpecMetadataRiskFinding("warning", "diff_kind_missing", "metadata", spec, "kind"))
	}
	if !strings.EqualFold(spec.Source, "git-worktree") {
		findings = append(findings, diffSpecMetadataRiskFinding("warning", "diff_source_not_git_worktree", "source-boundary", spec, "source"))
	}
	if !strings.EqualFold(spec.Mode, "metadata-only") {
		findings = append(findings, diffSpecMetadataRiskFinding("high", "diff_mode_not_metadata_only", "body-leakage", spec, "mode"))
	}
	if spec.MaxFiles <= 0 {
		findings = append(findings, diffSpecMetadataRiskFinding("warning", "diff_max_files_missing", "runtime-amplification", spec, "max_files"))
	} else if spec.MaxFiles > maxDiffFilesReturned {
		findings = append(findings, diffSpecMetadataRiskFinding("warning", "diff_max_files_exceeds_report_limit", "runtime-amplification", spec, "max_files"))
	}
	if spec.RawPatchAllowed {
		findings = append(findings, diffSpecMetadataRiskFinding("high", "diff_raw_patch_allowed", "body-leakage", spec, "raw_patch_allowed"))
	}
	if !spec.RequiresApproval {
		findings = append(findings, diffSpecMetadataRiskFinding("warning", "diff_approval_gate_missing", "approval", spec, "requires_approval"))
	}
	findings = append(findings, scanDiffRiskText("diff-spec", spec.Name, spec.Path, "body", readDiffRiskBody(root, spec.Path))...)
	sortDiffRiskFindings(findings)
	return findings
}

func scanDiffGitRiskFindings(git diffGitSurface) []DiffRiskFinding {
	var findings []DiffRiskFinding
	if !git.GitAvailable {
		findings = append(findings, diffGitMetadataRiskFinding("warning", "diff_git_unavailable", "git", git, "git_available"))
	}
	if git.GitAvailable && !git.GitRepository {
		findings = append(findings, diffGitMetadataRiskFinding("warning", "diff_git_repository_missing", "git", git, "git_repository"))
	}
	if git.FilesReturned >= maxDiffFilesReturned && git.ChangedFiles > git.FilesReturned {
		findings = append(findings, diffGitMetadataRiskFinding("warning", "diff_changed_file_limit_reached", "runtime-amplification", git, "diff_files_returned"))
	}
	sortDiffRiskFindings(findings)
	return findings
}

func diffSpecMetadataRiskFinding(severity, code, category string, spec diffSpecCard, field string) DiffRiskFinding {
	return DiffRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "diff-spec",
		Name:     spec.Name,
		Path:     spec.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(spec.Path + ":" + field),
	}
}

func diffGitMetadataRiskFinding(severity, code, category string, git diffGitSurface, field string) DiffRiskFinding {
	return DiffRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "git-worktree",
		Name:     "git",
		Path:     git.Root,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash("git:" + field + ":" + git.ErrorReason),
	}
}

func scanDiffRiskText(kind, name, path, field, body string) []DiffRiskFinding {
	var findings []DiffRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range diffTextRiskRules {
			if !diffRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, DiffRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Kind:     kind,
				Name:     name,
				Path:     path,
				Field:    field,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortDiffRiskFindings(findings)
	return findings
}

func diffRiskRuleMatches(lowerLine string, rule diffRiskRule) bool {
	for _, ignored := range rule.IgnoreAny {
		if strings.Contains(lowerLine, ignored) {
			return false
		}
	}
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

func readDiffRiskBody(root, relPath string) string {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(relPath)))
	if err != nil {
		return ""
	}
	return string(body)
}

func writeDiffRiskFindings(b *strings.Builder, findings []DiffRiskFinding) {
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

func diffRiskSurfaceCount(findings []DiffRiskFinding) int {
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

func diffRiskCodes(findings []DiffRiskFinding) []string {
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

func diffRiskLineHashes(findings []DiffRiskFinding) []string {
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

func diffRiskMaxSeverity(findings []DiffRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if diffRiskSeverityRank(finding.Severity) > diffRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func diffRiskSeverityRank(severity string) int {
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

func sortDiffRiskFindings(findings []DiffRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a := findings[i]
		b := findings[j]
		if rankA, rankB := diffRiskSeverityRank(a.Severity), diffRiskSeverityRank(b.Severity); rankA != rankB {
			return rankA > rankB
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		return a.LineSHA < b.LineSHA
	})
}
