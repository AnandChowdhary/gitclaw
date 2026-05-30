package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type WorkspaceRiskFinding struct {
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

type WorkspaceRiskReport struct {
	Status                                 string
	VerificationScope                      string
	WorkspacePolicyPresent                 bool
	WorkspacePolicyLoadedForModel          bool
	WorkspaceSpecs                         int
	ScannedWorkspaceSpecs                  int
	WorkspaceSpecsRequiringApproval        int
	GitAvailable                           bool
	GitRepository                          bool
	RepoFilesListed                        int
	RepoFileListLimit                      int
	ContextDocumentsLoaded                 int
	ContextAllowlist                       int
	WorkflowFilesPresent                   int
	CheckoutWorkflows                      int
	CheckoutSteps                          int
	SetupGoSteps                           int
	FetchDepthConfigured                   bool
	CheckoutActionVersions                 string
	SetupGoActionVersions                  string
	SandboxBackend                         string
	DurableStateBackend                    string
	SurfacesWithRiskFindings               int
	Findings                               []WorkspaceRiskFinding
	HighRiskFindings                       int
	WarningRiskFindings                    int
	InfoRiskFindings                       int
	PrivateWorkspaceMemorySupported        bool
	ExternalWorkspaceMountSupported        bool
	WorkspaceMutationAllowed               bool
	WorkspaceDaemonSupported               bool
	LongRunningSocketSupported             bool
	RawWorkspaceBodiesIncluded             bool
	RawFileBodiesIncluded                  bool
	RawWorkflowBodiesIncluded              bool
	RawIssueBodiesIncluded                 bool
	RawCommentBodiesIncluded               bool
	CredentialValuesIncluded               bool
	RepositoryMutationAllowed              bool
	LLME2ERequiredAfterWorkspaceRiskChange bool
}

type workspaceRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var workspaceTextRiskRules = []workspaceRiskRule{
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
		Code:     "credential_material_in_workspace",
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
		Code:     "private_workspace_memory_enabled",
		Category: "state-boundary",
		Any: []string{
			"private workspace memory",
			"runner-local durable memory",
			"workspace memory is durable",
			"store memory in runner",
			"persist memory in workspace",
			"hidden workspace state",
			"workspace state is source of truth",
		},
		IgnoreAny: []string{
			"not private durable memory",
			"private workspace memory supported: false",
			"private workspace memory, external workspace mounts",
			"private workspace memory or external mounts require",
			"does not grant",
			"outside v1",
		},
	},
	{
		Severity: "high",
		Code:     "external_workspace_mount_enabled",
		Category: "storage-boundary",
		Any: []string{
			"external workspace mount",
			"mount external paths",
			"mount host path",
			"bind mount",
			"/var/run/docker.sock",
			"docker.sock",
			"sshfs",
			"nfs mount",
			"storage: s3",
			"storage: gcs",
			"s3://",
			"gs://",
		},
		IgnoreAny: []string{
			"must not",
			"external workspace mount supported: false",
			"external workspace mounts, long-running sockets",
			"external mounts require",
			"does not grant",
			"outside v1",
		},
	},
	{
		Severity: "high",
		Code:     "destructive_workspace_mutation",
		Category: "write-authority",
		Any: []string{
			"rm -rf",
			"git reset --hard",
			"git checkout --",
			"git clean -fd",
			"git restore --source",
			"stage files automatically",
			"commit directly",
			"push directly",
			"write workspace state",
			"delete workspace files",
			"clean directories",
			"switch refs",
		},
		IgnoreAny: []string{
			"must not",
			"never writes",
			"does not grant",
			"not change",
		},
	},
	{
		Severity: "high",
		Code:     "long_running_workspace_service",
		Category: "runtime-boundary",
		Any: []string{
			"long-running sockets",
			"long running sockets",
			"websocket server",
			"start daemon",
			"background daemon",
			"listen on 0.0.0.0",
			"serve forever",
			"sleep infinity",
		},
		IgnoreAny: []string{
			"must not",
			"long-running sockets, and mutable",
			"outside v1",
			"does not grant",
		},
	},
	{
		Severity: "high",
		Code:     "raw_workspace_body_leakage",
		Category: "body-leakage",
		Any: []string{
			"print raw file bodies",
			"print workflow bodies",
			"print all file contents",
			"dump repository files",
			"include full file body",
			"include raw workflow",
			"cat .github/workflows",
			"cat $github_event_path",
		},
		IgnoreAny: []string{
			"must not print",
			"not printed",
			"without printing",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_workspace_inventory",
		Category: "runtime-amplification",
		Any: []string{
			"repo_file_list_limit: 0",
			"repo_file_list_limit: -1",
			"max_files: 0",
			"max_files: -1",
			"walk entire filesystem",
			"scan /",
			"while true",
			"retry forever",
			"continue indefinitely",
		},
	},
}

func renderWorkspaceRiskReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectWorkspaceSurface(cfg.Workdir)
	report := BuildWorkspaceRiskReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Workspace Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeWorkspaceRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans workspace policy, workspace specs, and workflow checkout metadata for prompt-boundary, credential, private memory, external mount, destructive mutation, long-running service, raw body leakage, checkout, setup, approval, and unbounded-inventory risks. It reports metadata, paths, risk codes, severities, and hashes only; workspace bodies, file bodies, workflow bodies, issue bodies, comments, prompts, tool outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Workspace Policy Risk Card\n")
	writeWorkspacePolicyRiskCard(&b, cfg.Workdir, surface.Policy)

	b.WriteString("\n### Workspace Spec Risk Cards\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- kind=`workspace-spec` none\n")
	} else {
		for _, spec := range surface.Specs {
			writeWorkspaceSpecRiskCard(&b, cfg.Workdir, spec)
		}
	}

	b.WriteString("\n### Workflow Workspace Risk Cards\n")
	if len(surface.Workflows) == 0 {
		b.WriteString("- kind=`workspace-workflow` none\n")
	} else {
		for _, workflow := range surface.Workflows {
			writeWorkspaceWorkflowRiskCard(&b, workflow)
		}
	}

	b.WriteString("\n### Git Workspace Risk Card\n")
	writeWorkspaceGitRiskCard(&b, surface.Git)

	b.WriteString("\n### Repository Inventory Risk Card\n")
	fmt.Fprintf(&b, "- kind=`repository-inventory` repo_files_listed=`%d` repo_file_list_limit=`%d` context_documents_loaded=`%d` context_allowlist_entries=`%d` raw_paths_included=`false` raw_context_bodies_included=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n", surface.RepoFilesListed, surface.RepoFileListLimit, surface.ContextDocumentsLoaded, surface.ContextAllowlist)

	b.WriteString("\n### Current Workspace Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-workspace-request` current_issue_workspace_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-workspace-request` scope=`local-cli` current_issue_workspace_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeWorkspaceRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildWorkspaceRiskReport(cfg Config) WorkspaceRiskReport {
	surface := inspectWorkspaceSurface(cfg.Workdir)
	report := WorkspaceRiskReport{
		Status:                                 "ok",
		VerificationScope:                      "github_actions_workspace_metadata",
		WorkspacePolicyPresent:                 surface.Policy.Present,
		WorkspacePolicyLoadedForModel:          workspacePolicyLoadedForModel(surface),
		WorkspaceSpecs:                         len(surface.Specs),
		WorkspaceSpecsRequiringApproval:        workspaceSpecsRequiringApproval(surface.Specs),
		GitAvailable:                           surface.Git.GitAvailable,
		GitRepository:                          surface.Git.GitRepository,
		RepoFilesListed:                        surface.RepoFilesListed,
		RepoFileListLimit:                      surface.RepoFileListLimit,
		ContextDocumentsLoaded:                 surface.ContextDocumentsLoaded,
		ContextAllowlist:                       surface.ContextAllowlist,
		WorkflowFilesPresent:                   workspaceWorkflowsPresent(surface.Workflows),
		CheckoutWorkflows:                      workspaceCheckoutWorkflows(surface.Workflows),
		CheckoutSteps:                          workspaceCheckoutSteps(surface.Workflows),
		SetupGoSteps:                           workspaceSetupGoSteps(surface.Workflows),
		FetchDepthConfigured:                   workspaceFetchDepthConfigured(surface.Workflows),
		CheckoutActionVersions:                 joinOrNone(workspaceCheckoutVersions(surface.Workflows)),
		SetupGoActionVersions:                  joinOrNone(workspaceSetupGoVersions(surface.Workflows)),
		SandboxBackend:                         "github-actions",
		DurableStateBackend:                    "git-tracked-files-and-backup-branch",
		PrivateWorkspaceMemorySupported:        false,
		ExternalWorkspaceMountSupported:        false,
		WorkspaceMutationAllowed:               false,
		WorkspaceDaemonSupported:               false,
		LongRunningSocketSupported:             false,
		RawWorkspaceBodiesIncluded:             false,
		RawFileBodiesIncluded:                  false,
		RawWorkflowBodiesIncluded:              false,
		RawIssueBodiesIncluded:                 false,
		RawCommentBodiesIncluded:               false,
		CredentialValuesIncluded:               false,
		RepositoryMutationAllowed:              false,
		LLME2ERequiredAfterWorkspaceRiskChange: true,
	}
	report.Findings = append(report.Findings, scanWorkspacePolicyRiskFindings(cfg.Workdir, surface.Policy)...)
	for _, spec := range surface.Specs {
		report.ScannedWorkspaceSpecs++
		report.Findings = append(report.Findings, scanWorkspaceSpecRiskFindings(cfg.Workdir, spec)...)
	}
	for _, workflow := range surface.Workflows {
		report.Findings = append(report.Findings, scanWorkspaceWorkflowRiskFindings(workflow)...)
	}
	if report.WorkflowFilesPresent == 0 {
		report.Findings = append(report.Findings, WorkspaceRiskFinding{Severity: "warning", Code: "workspace_workflows_missing", Category: "workflow", Kind: "workspace-workflows", Name: "workflows", Path: ".github/workflows", Field: "present", LineSHA: shortDocumentHash(".github/workflows:present")})
	}
	if report.WorkflowFilesPresent > 0 && report.CheckoutWorkflows == 0 {
		report.Findings = append(report.Findings, WorkspaceRiskFinding{Severity: "high", Code: "workspace_checkout_missing", Category: "workflow", Kind: "workspace-workflows", Name: "workflows", Path: ".github/workflows", Field: "checkout_actions", LineSHA: shortDocumentHash(".github/workflows:checkout_actions")})
	}
	report.Findings = append(report.Findings, scanWorkspaceGitRiskFindings(surface.Git)...)
	if surface.RepoFileListError != "" {
		report.Findings = append(report.Findings, WorkspaceRiskFinding{
			Severity: "warning",
			Code:     "workspace_repo_files_unavailable",
			Category: "inventory",
			Kind:     "repository-inventory",
			Name:     "repository",
			Path:     ".",
			Field:    "repo_files_listed",
			LineSHA:  shortDocumentHash("repository:" + surface.RepoFileListError),
		})
	}
	sortWorkspaceRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = workspaceRiskSurfaceCount(report.Findings)
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

func writeWorkspaceRiskSummary(b *strings.Builder, report WorkspaceRiskReport) {
	fmt.Fprintf(b, "- workspace_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- workspace_policy_present: `%t`\n", report.WorkspacePolicyPresent)
	fmt.Fprintf(b, "- workspace_policy_loaded_for_model: `%t`\n", report.WorkspacePolicyLoadedForModel)
	fmt.Fprintf(b, "- workspace_specs: `%d`\n", report.WorkspaceSpecs)
	fmt.Fprintf(b, "- scanned_workspace_specs: `%d`\n", report.ScannedWorkspaceSpecs)
	fmt.Fprintf(b, "- workspace_specs_requiring_approval: `%d`\n", report.WorkspaceSpecsRequiringApproval)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_repository: `%t`\n", report.GitRepository)
	fmt.Fprintf(b, "- repo_files_listed: `%d`\n", report.RepoFilesListed)
	fmt.Fprintf(b, "- repo_file_list_limit: `%d`\n", report.RepoFileListLimit)
	fmt.Fprintf(b, "- context_documents_loaded: `%d`\n", report.ContextDocumentsLoaded)
	fmt.Fprintf(b, "- context_allowlist_entries: `%d`\n", report.ContextAllowlist)
	fmt.Fprintf(b, "- workflow_files_present: `%d`\n", report.WorkflowFilesPresent)
	fmt.Fprintf(b, "- checkout_workflows: `%d`\n", report.CheckoutWorkflows)
	fmt.Fprintf(b, "- checkout_steps: `%d`\n", report.CheckoutSteps)
	fmt.Fprintf(b, "- setup_go_steps: `%d`\n", report.SetupGoSteps)
	fmt.Fprintf(b, "- checkout_action_versions: `%s`\n", inlineCode(report.CheckoutActionVersions))
	fmt.Fprintf(b, "- setup_go_action_versions: `%s`\n", inlineCode(report.SetupGoActionVersions))
	fmt.Fprintf(b, "- fetch_depth_configured: `%t`\n", report.FetchDepthConfigured)
	fmt.Fprintf(b, "- sandbox_backend: `%s`\n", report.SandboxBackend)
	fmt.Fprintf(b, "- durable_state_backend: `%s`\n", report.DurableStateBackend)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- workspace_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- private_workspace_memory_supported: `%t`\n", report.PrivateWorkspaceMemorySupported)
	fmt.Fprintf(b, "- external_workspace_mount_supported: `%t`\n", report.ExternalWorkspaceMountSupported)
	fmt.Fprintf(b, "- workspace_mutation_allowed: `%t`\n", report.WorkspaceMutationAllowed)
	fmt.Fprintf(b, "- workspace_daemon_supported: `%t`\n", report.WorkspaceDaemonSupported)
	fmt.Fprintf(b, "- long_running_socket_supported: `%t`\n", report.LongRunningSocketSupported)
	fmt.Fprintf(b, "- raw_workspace_bodies_included: `%t`\n", report.RawWorkspaceBodiesIncluded)
	fmt.Fprintf(b, "- raw_file_bodies_included: `%t`\n", report.RawFileBodiesIncluded)
	fmt.Fprintf(b, "- raw_workflow_bodies_included: `%t`\n", report.RawWorkflowBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- llm_e2e_required_after_workspace_risk_change: `%t`\n", report.LLME2ERequiredAfterWorkspaceRiskChange)
}

func writeWorkspacePolicyRiskCard(b *strings.Builder, root string, policy configSurfaceFile) {
	findings := scanWorkspacePolicyRiskFindings(root, policy)
	if !policy.Present {
		fmt.Fprintf(b, "- kind=`workspace-policy` path=`%s` present=`false` loaded_for_model=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n", policy.Path, len(findings), workspaceRiskMaxSeverity(findings), inlineListOrNone(workspaceRiskCodes(findings)), inlineListOrNone(workspaceRiskLineHashes(findings)))
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`workspace-policy` path=`%s` present=`true` loaded_for_model=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		policy.Path,
		workspacePolicyPathInContext(),
		policy.Bytes,
		policy.Lines,
		policy.SHA,
		len(findings),
		workspaceRiskMaxSeverity(findings),
		inlineListOrNone(workspaceRiskCodes(findings)),
		inlineListOrNone(workspaceRiskLineHashes(findings)),
	)
}

func writeWorkspaceSpecRiskCard(b *strings.Builder, root string, spec workspaceSpecCard) {
	findings := scanWorkspaceSpecRiskFindings(root, spec)
	fmt.Fprintf(
		b,
		"- kind=`workspace-spec` name=`%s` path=`%s` frontmatter=`%t` workspace_kind=`%s` runtime=`%s` storage=`%s` mode=`%s` root=`%s` isolation=`%s` durable_state=`%s` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(spec.Name),
		spec.Path,
		spec.Frontmatter,
		inlineCode(spec.Kind),
		inlineCode(spec.Runtime),
		inlineCode(spec.Storage),
		inlineCode(spec.Mode),
		inlineCode(spec.Root),
		inlineCode(spec.Isolation),
		inlineCode(spec.DurableState),
		spec.RequiresApproval,
		spec.Bytes,
		spec.Lines,
		spec.SHA,
		len(findings),
		workspaceRiskMaxSeverity(findings),
		inlineListOrNone(workspaceRiskCodes(findings)),
		inlineListOrNone(workspaceRiskLineHashes(findings)),
	)
}

func writeWorkspaceWorkflowRiskCard(b *strings.Builder, workflow workspaceWorkflowCard) {
	findings := scanWorkspaceWorkflowRiskFindings(workflow)
	fmt.Fprintf(
		b,
		"- kind=`workspace-workflow` path=`%s` present=`%t` checkout_actions=`%s` checkout_steps=`%d` setup_go_actions=`%s` setup_go_steps=`%d` fetch_depth_configured=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		workflow.Path,
		workflow.Present,
		inlineCode(joinOrNone(workflow.CheckoutActions)),
		workflow.CheckoutSteps,
		inlineCode(joinOrNone(workflow.SetupGoActions)),
		workflow.SetupGoSteps,
		workflow.FetchDepthConfigured,
		workflow.Bytes,
		workflow.Lines,
		workflow.SHA,
		len(findings),
		workspaceRiskMaxSeverity(findings),
		inlineListOrNone(workspaceRiskCodes(findings)),
		inlineListOrNone(workspaceRiskLineHashes(findings)),
	)
}

func writeWorkspaceGitRiskCard(b *strings.Builder, git workspaceGitSurface) {
	findings := scanWorkspaceGitRiskFindings(git)
	fmt.Fprintf(
		b,
		"- kind=`git-workspace` root=`%s` git_available=`%t` git_repository=`%t` branch=`%s` head_commit=`%s` error_reason=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(git.Root),
		git.GitAvailable,
		git.GitRepository,
		inlineCode(git.Branch),
		git.HeadShortSHA,
		inlineCode(git.ErrorReason),
		len(findings),
		workspaceRiskMaxSeverity(findings),
		inlineListOrNone(workspaceRiskCodes(findings)),
		inlineListOrNone(workspaceRiskLineHashes(findings)),
	)
}

func scanWorkspacePolicyRiskFindings(root string, policy configSurfaceFile) []WorkspaceRiskFinding {
	var findings []WorkspaceRiskFinding
	if !policy.Present {
		findings = append(findings, WorkspaceRiskFinding{Severity: "info", Code: "workspace_policy_not_configured", Category: "policy", Kind: "workspace-policy", Name: "policy", Path: policy.Path, Field: "present", LineSHA: shortDocumentHash(policy.Path + ":present")})
		return findings
	}
	if !workspacePolicyPathInContext() {
		findings = append(findings, WorkspaceRiskFinding{Severity: "high", Code: "workspace_policy_not_loaded", Category: "context", Kind: "workspace-policy", Name: "policy", Path: policy.Path, Field: "loaded_for_model", LineSHA: shortDocumentHash(policy.Path + ":loaded_for_model")})
	}
	findings = append(findings, scanWorkspaceRiskText("workspace-policy", "policy", policy.Path, "body", readWorkspaceRiskBody(root, policy.Path))...)
	sortWorkspaceRiskFindings(findings)
	return findings
}

func scanWorkspaceSpecRiskFindings(root string, spec workspaceSpecCard) []WorkspaceRiskFinding {
	var findings []WorkspaceRiskFinding
	if !spec.Frontmatter {
		findings = append(findings, workspaceSpecMetadataRiskFinding("warning", "workspace_frontmatter_missing", "metadata", spec, "frontmatter"))
	}
	if !strings.EqualFold(spec.Kind, "git-workspace") {
		findings = append(findings, workspaceSpecMetadataRiskFinding("warning", "workspace_kind_not_git_workspace", "metadata", spec, "kind"))
	}
	if !strings.EqualFold(spec.Runtime, "github-actions") {
		findings = append(findings, workspaceSpecMetadataRiskFinding("high", "workspace_runtime_not_github_actions", "runtime-boundary", spec, "runtime"))
	}
	if !strings.EqualFold(spec.Storage, "repository-checkout") {
		findings = append(findings, workspaceSpecMetadataRiskFinding("warning", "workspace_storage_not_checkout", "storage-boundary", spec, "storage"))
	}
	if !strings.EqualFold(spec.Mode, "metadata-only") {
		findings = append(findings, workspaceSpecMetadataRiskFinding("high", "workspace_mode_not_metadata_only", "body-leakage", spec, "mode"))
	}
	if strings.TrimSpace(spec.Root) != "." {
		findings = append(findings, workspaceSpecMetadataRiskFinding("high", "workspace_root_not_repo_root", "workspace-boundary", spec, "root"))
	}
	if !strings.EqualFold(spec.Isolation, "ephemeral-actions-runner") {
		findings = append(findings, workspaceSpecMetadataRiskFinding("warning", "workspace_isolation_not_ephemeral_runner", "runtime-boundary", spec, "isolation"))
	}
	if !strings.EqualFold(spec.DurableState, "git-tracked-files-and-backup-branch") {
		findings = append(findings, workspaceSpecMetadataRiskFinding("warning", "workspace_durable_state_unreviewed", "state-boundary", spec, "durable_state"))
	}
	if !spec.RequiresApproval {
		findings = append(findings, workspaceSpecMetadataRiskFinding("warning", "workspace_approval_gate_missing", "approval", spec, "requires_approval"))
	}
	findings = append(findings, scanWorkspaceRiskText("workspace-spec", spec.Name, spec.Path, "body", readWorkspaceRiskBody(root, spec.Path))...)
	sortWorkspaceRiskFindings(findings)
	return findings
}

func scanWorkspaceWorkflowRiskFindings(workflow workspaceWorkflowCard) []WorkspaceRiskFinding {
	var findings []WorkspaceRiskFinding
	if !workflow.Present {
		return findings
	}
	if len(workflow.CheckoutActions) == 0 {
		findings = append(findings, workspaceWorkflowMetadataRiskFinding("high", "workspace_checkout_missing", "workflow", workflow, "checkout_actions"))
	} else if !workspaceWorkflowUsesActionVersion(workflow.CheckoutActions, "actions/checkout@v5") {
		findings = append(findings, workspaceWorkflowMetadataRiskFinding("warning", "workspace_checkout_action_not_v5", "workflow", workflow, "checkout_actions"))
	}
	if !workspaceWorkflowUsesActionVersion(workflow.SetupGoActions, "actions/setup-go@v6") {
		findings = append(findings, workspaceWorkflowMetadataRiskFinding("warning", "workspace_setup_go_action_not_v6", "workflow", workflow, "setup_go_actions"))
	}
	if !workflow.FetchDepthConfigured {
		findings = append(findings, workspaceWorkflowMetadataRiskFinding("warning", "workspace_fetch_depth_missing", "workflow", workflow, "fetch_depth"))
	}
	sortWorkspaceRiskFindings(findings)
	return findings
}

func scanWorkspaceGitRiskFindings(git workspaceGitSurface) []WorkspaceRiskFinding {
	var findings []WorkspaceRiskFinding
	if !git.GitAvailable {
		findings = append(findings, workspaceGitMetadataRiskFinding("warning", "workspace_git_unavailable", "git", git, "git_available"))
	}
	if git.GitAvailable && !git.GitRepository {
		findings = append(findings, workspaceGitMetadataRiskFinding("warning", "workspace_git_repository_missing", "git", git, "git_repository"))
	}
	sortWorkspaceRiskFindings(findings)
	return findings
}

func workspaceSpecMetadataRiskFinding(severity, code, category string, spec workspaceSpecCard, field string) WorkspaceRiskFinding {
	return WorkspaceRiskFinding{Severity: severity, Code: code, Category: category, Kind: "workspace-spec", Name: spec.Name, Path: spec.Path, Field: field, LineSHA: shortDocumentHash(spec.Path + ":" + field)}
}

func workspaceWorkflowMetadataRiskFinding(severity, code, category string, workflow workspaceWorkflowCard, field string) WorkspaceRiskFinding {
	return WorkspaceRiskFinding{Severity: severity, Code: code, Category: category, Kind: "workspace-workflow", Name: "workflow", Path: workflow.Path, Field: field, LineSHA: shortDocumentHash(workflow.Path + ":" + field)}
}

func workspaceGitMetadataRiskFinding(severity, code, category string, git workspaceGitSurface, field string) WorkspaceRiskFinding {
	return WorkspaceRiskFinding{Severity: severity, Code: code, Category: category, Kind: "git-workspace", Name: "git", Path: git.Root, Field: field, LineSHA: shortDocumentHash("git:" + field + ":" + git.ErrorReason)}
}

func workspaceWorkflowUsesActionVersion(actions []string, want string) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

func scanWorkspaceRiskText(kind, name, path, field, body string) []WorkspaceRiskFinding {
	var findings []WorkspaceRiskFinding
	lines := strings.Split(body, "\n")
	for lineNumber, line := range lines {
		lower := strings.ToLower(line)
		contextLower := strings.ToLower(workspaceRiskLineContext(lines, lineNumber))
		for _, rule := range workspaceTextRiskRules {
			if !workspaceRiskRuleMatches(lower, contextLower, rule) {
				continue
			}
			findings = append(findings, WorkspaceRiskFinding{Severity: rule.Severity, Code: rule.Code, Category: rule.Category, Kind: kind, Name: name, Path: path, Field: field, Line: lineNumber + 1, LineSHA: shortDocumentHash(line)})
		}
	}
	sortWorkspaceRiskFindings(findings)
	return findings
}

func workspaceRiskLineContext(lines []string, lineNumber int) string {
	var context []string
	start := lineNumber - 2
	if start < 0 {
		start = 0
	}
	end := lineNumber + 2
	if end >= len(lines) {
		end = len(lines) - 1
	}
	for i := start; i <= end; i++ {
		context = append(context, lines[i])
	}
	return strings.Join(context, " ")
}

func workspaceRiskRuleMatches(lowerLine, lowerContext string, rule workspaceRiskRule) bool {
	for _, ignored := range rule.IgnoreAny {
		if strings.Contains(lowerContext, ignored) {
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

func readWorkspaceRiskBody(root, relPath string) string {
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

func writeWorkspaceRiskFindings(b *strings.Builder, findings []WorkspaceRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` kind=`%s` name=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.Name, finding.Path, finding.Field, finding.Line, finding.LineSHA)
	}
}

func workspaceRiskSurfaceCount(findings []WorkspaceRiskFinding) int {
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

func workspaceRiskCodes(findings []WorkspaceRiskFinding) []string {
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

func workspaceRiskLineHashes(findings []WorkspaceRiskFinding) []string {
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

func workspaceRiskMaxSeverity(findings []WorkspaceRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if workspaceRiskSeverityRank(finding.Severity) > workspaceRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func workspaceRiskSeverityRank(severity string) int {
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

func sortWorkspaceRiskFindings(findings []WorkspaceRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a := findings[i]
		b := findings[j]
		if rankA, rankB := workspaceRiskSeverityRank(a.Severity), workspaceRiskSeverityRank(b.Severity); rankA != rankB {
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
