package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const workspacePolicyPath = ".gitclaw/WORKSPACE.md"
const workspaceSpecsDir = ".gitclaw/workspaces"

type workspaceSurface struct {
	Policy                 configSurfaceFile
	Specs                  []workspaceSpecCard
	Git                    workspaceGitSurface
	Workflows              []workspaceWorkflowCard
	RepoFilesListed        int
	RepoFileListLimit      int
	RepoFileListError      string
	ContextDocumentsLoaded int
	ContextAllowlist       int
}

type workspaceSpecCard struct {
	Name             string
	Path             string
	Present          bool
	Bytes            int
	Lines            int
	SHA              string
	Frontmatter      bool
	Kind             string
	Runtime          string
	Storage          string
	Mode             string
	Root             string
	Isolation        string
	DurableState     string
	RequiresApproval bool
}

type workspaceSpecFrontmatter struct {
	Name             string `yaml:"name"`
	Kind             string `yaml:"kind"`
	Runtime          string `yaml:"runtime"`
	Storage          string `yaml:"storage"`
	Mode             string `yaml:"mode"`
	Root             string `yaml:"root"`
	Isolation        string `yaml:"isolation"`
	DurableState     string `yaml:"durable_state"`
	RequiresApproval bool   `yaml:"requires_approval"`
}

type workspaceGitSurface struct {
	GitAvailable  bool
	GitRepository bool
	Root          string
	Branch        string
	HeadShortSHA  string
	ErrorReason   string
}

type workspaceWorkflowCard struct {
	Path                 string
	Present              bool
	Bytes                int
	Lines                int
	SHA                  string
	CheckoutActions      []string
	CheckoutSteps        int
	SetupGoActions       []string
	SetupGoSteps         int
	FetchDepthConfigured bool
}

type workspaceFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsWorkspaceReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/workspace" || command == "/workdir" || command == "/repo"
}

func RenderWorkspaceReport(ev Event, cfg Config) string {
	return renderWorkspaceReport(ev, cfg, true)
}

func RenderWorkspaceCLIReport(cfg Config) string {
	return renderWorkspaceReport(Event{}, cfg, false)
}

func renderWorkspaceReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectWorkspaceSurface(cfg.Workdir)
	findings := workspaceFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Workspace Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- workspace_status: `%s`\n", workspaceStatus(findings))
	fmt.Fprintf(&b, "- workspace_policy_path: `%s`\n", workspacePolicyPath)
	fmt.Fprintf(&b, "- workspace_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- workspace_policy_loaded_for_model: `%t`\n", workspacePolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- workspace_specs_dir: `%s`\n", workspaceSpecsDir)
	fmt.Fprintf(&b, "- workspace_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- workspace_specs_with_frontmatter: `%d`\n", workspaceSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- workspace_specs_requiring_approval: `%d`\n", workspaceSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- git_available: `%t`\n", surface.Git.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", surface.Git.GitRepository)
	fmt.Fprintf(&b, "- worktree_root: `%s`\n", inlineCode(surface.Git.Root))
	fmt.Fprintf(&b, "- branch: `%s`\n", inlineCode(surface.Git.Branch))
	fmt.Fprintf(&b, "- head_commit: `%s`\n", surface.Git.HeadShortSHA)
	fmt.Fprintf(&b, "- repo_files_listed: `%d`\n", surface.RepoFilesListed)
	fmt.Fprintf(&b, "- repo_file_list_limit: `%d`\n", surface.RepoFileListLimit)
	fmt.Fprintf(&b, "- context_documents_loaded: `%d`\n", surface.ContextDocumentsLoaded)
	fmt.Fprintf(&b, "- context_allowlist_entries: `%d`\n", surface.ContextAllowlist)
	fmt.Fprintf(&b, "- workspace_context_policy_loaded: `%t`\n", workspacePolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- workflow_files_present: `%d`\n", workspaceWorkflowsPresent(surface.Workflows))
	fmt.Fprintf(&b, "- checkout_workflows: `%d`\n", workspaceCheckoutWorkflows(surface.Workflows))
	fmt.Fprintf(&b, "- checkout_steps: `%d`\n", workspaceCheckoutSteps(surface.Workflows))
	fmt.Fprintf(&b, "- checkout_action_versions: `%s`\n", inlineCode(joinOrNone(workspaceCheckoutVersions(surface.Workflows))))
	fmt.Fprintf(&b, "- setup_go_action_versions: `%s`\n", inlineCode(joinOrNone(workspaceSetupGoVersions(surface.Workflows))))
	fmt.Fprintf(&b, "- fetch_depth_configured: `%t`\n", workspaceFetchDepthConfigured(surface.Workflows))
	fmt.Fprintf(&b, "- sandbox_backend: `%s`\n", "github-actions")
	fmt.Fprintf(&b, "- durable_state_backend: `%s`\n", "git-tracked-files-and-backup-branch")
	fmt.Fprintf(&b, "- private_workspace_memory_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- external_workspace_mount_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- workspace_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if surface.Git.ErrorReason != "" {
		fmt.Fprintf(&b, "- git_error_reason: `%s`\n", surface.Git.ErrorReason)
	}
	if surface.RepoFileListError != "" {
		fmt.Fprintf(&b, "- repo_file_list_error: `%s`\n", surface.RepoFileListError)
	}
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("The workspace is reported as metadata only. File bodies, issue/comment bodies, prompts, tool outputs, backup payloads, workflow bodies, and secrets are not printed by this report.\n\n")

	b.WriteString("### Workspace Policy\n")
	writeConfigSurfaceFile(&b, surface.Policy)
	if workspacePolicyPathInContext() {
		b.WriteString("- `.gitclaw/WORKSPACE.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/WORKSPACE.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Workspace Specs\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, spec := range surface.Specs {
			fmt.Fprintf(
				&b,
				"- name=`%s` path=`%s` bytes=`%d` lines=`%d` frontmatter=`%t` kind=`%s` runtime=`%s` storage=`%s` mode=`%s` root=`%s` isolation=`%s` durable_state=`%s` requires_approval=`%t` sha256_12=`%s`\n",
				inlineCode(spec.Name),
				spec.Path,
				spec.Bytes,
				spec.Lines,
				spec.Frontmatter,
				inlineCode(spec.Kind),
				inlineCode(spec.Runtime),
				inlineCode(spec.Storage),
				inlineCode(spec.Mode),
				inlineCode(spec.Root),
				inlineCode(spec.Isolation),
				inlineCode(spec.DurableState),
				spec.RequiresApproval,
				spec.SHA,
			)
		}
	}

	b.WriteString("\n### Workflow Workspace Setup\n")
	if len(surface.Workflows) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, workflow := range surface.Workflows {
			fmt.Fprintf(
				&b,
				"- path=`%s` present=`%t` bytes=`%d` lines=`%d` checkout_actions=`%s` setup_go_actions=`%s` fetch_depth_configured=`%t` sha256_12=`%s`\n",
				workflow.Path,
				workflow.Present,
				workflow.Bytes,
				workflow.Lines,
				inlineCode(joinOrNone(workflow.CheckoutActions)),
				inlineCode(joinOrNone(workflow.SetupGoActions)),
				workflow.FetchDepthConfigured,
				workflow.SHA,
			)
		}
	}

	b.WriteString("\n### Repository Inventory\n")
	fmt.Fprintf(&b, "- repo_files_listed=`%d` limit=`%d` raw_paths_included=`%t`\n", surface.RepoFilesListed, surface.RepoFileListLimit, false)
	fmt.Fprintf(&b, "- context_documents_loaded=`%d` allowlist_entries=`%d` raw_context_bodies_included=`%t`\n", surface.ContextDocumentsLoaded, surface.ContextAllowlist, false)

	b.WriteString("\n### Runtime Boundary\n")
	b.WriteString("- `/workspace` is inspect-only and never writes, deletes, cleans, checks out, commits, dispatches, or mounts workspace paths\n")
	b.WriteString("- GitHub Actions runner state is ephemeral; durable state stays in reviewed git files, backups, or explicit artifacts\n")
	b.WriteString("- future private workspace memory or external mounts require reviewed specs, explicit permissions, and live GitHub Models E2E coverage\n")

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

func inspectWorkspaceSurface(root string) workspaceSurface {
	if root == "" {
		root = "."
	}
	surface := workspaceSurface{
		Policy:            inspectConfigSurfaceFile(root, workspacePolicyPath),
		RepoFileListLimit: maxRepoFilesListed,
		ContextAllowlist:  len(contextDocumentPaths),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		surface.Git = workspaceGitSurface{Root: ".", ErrorReason: "workdir_abs_failed"}
		surface.RepoFileListError = "workdir_abs_failed"
		return surface
	}
	surface.Specs = inspectWorkspaceSpecs(absRoot)
	surface.Workflows = inspectWorkspaceWorkflows(absRoot)
	surface.Git = inspectWorkspaceGit(absRoot)
	files, err := listRepoFiles(absRoot)
	if err != nil {
		surface.RepoFileListError = "repo_file_walk_failed"
	} else {
		surface.RepoFilesListed = len(files)
	}
	surface.ContextDocumentsLoaded = len(loadContextDocuments(absRoot, contextDocumentPaths))
	return surface
}

func inspectWorkspaceSpecs(absRoot string) []workspaceSpecCard {
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "workspaces", "*.md"))
	sort.Strings(matches)
	specs := make([]workspaceSpecCard, 0, len(matches))
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
		spec := workspaceSpecCard{
			Name:    strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)),
			Path:    relPath,
			Present: true,
			Bytes:   len(body),
			Lines:   lineCount(text),
			SHA:     shortDocumentHash(text),
		}
		if meta, ok := parseWorkspaceFrontmatter(text); ok {
			spec.Frontmatter = true
			if strings.TrimSpace(meta.Name) != "" {
				spec.Name = strings.TrimSpace(meta.Name)
			}
			spec.Kind = strings.TrimSpace(meta.Kind)
			spec.Runtime = strings.TrimSpace(meta.Runtime)
			spec.Storage = strings.TrimSpace(meta.Storage)
			spec.Mode = strings.TrimSpace(meta.Mode)
			spec.Root = strings.TrimSpace(meta.Root)
			spec.Isolation = strings.TrimSpace(meta.Isolation)
			spec.DurableState = strings.TrimSpace(meta.DurableState)
			spec.RequiresApproval = meta.RequiresApproval
		}
		specs = append(specs, spec)
	}
	return specs
}

func inspectWorkspaceGit(absRoot string) workspaceGitSurface {
	report := workspaceGitSurface{Root: "."}
	if info, err := os.Stat(absRoot); err != nil || !info.IsDir() {
		report.ErrorReason = "workdir_not_directory"
		return report
	}
	if _, err := exec.LookPath("git"); err != nil {
		report.ErrorReason = "git_not_found"
		return report
	}
	report.GitAvailable = true
	inside, err := runDiffGit(absRoot, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		report.ErrorReason = "not_git_repository"
		return report
	}
	report.GitRepository = true
	if root, err := runDiffGit(absRoot, "rev-parse", "--show-toplevel"); err == nil && strings.TrimSpace(root) != "" {
		baseRoot := absRoot
		if resolved, resolveErr := filepath.EvalSymlinks(absRoot); resolveErr == nil {
			baseRoot = resolved
		}
		topRoot := strings.TrimSpace(root)
		if resolved, resolveErr := filepath.EvalSymlinks(topRoot); resolveErr == nil {
			topRoot = resolved
		}
		if rel, relErr := filepath.Rel(baseRoot, topRoot); relErr == nil {
			report.Root = filepath.ToSlash(rel)
		}
	}
	if report.Root == "" {
		report.Root = "."
	}
	if branch, err := runDiffGit(absRoot, "branch", "--show-current"); err == nil && strings.TrimSpace(branch) != "" {
		report.Branch = strings.TrimSpace(branch)
	} else {
		report.Branch = "(detached)"
	}
	if short, err := runDiffGit(absRoot, "rev-parse", "--short=12", "HEAD"); err == nil {
		report.HeadShortSHA = strings.TrimSpace(short)
	}
	return report
}

func inspectWorkspaceWorkflows(absRoot string) []workspaceWorkflowCard {
	workflows := make([]workspaceWorkflowCard, 0, len(configWorkflowPaths))
	for _, path := range configWorkflowPaths {
		card := workspaceWorkflowCard{Path: path}
		body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(path)))
		if err != nil {
			workflows = append(workflows, card)
			continue
		}
		text := string(body)
		card.Present = true
		card.Bytes = len(body)
		card.Lines = lineCount(text)
		card.SHA = shortDocumentHash(text)
		checkoutMatches := workspaceActionMatches(text, "actions/checkout")
		setupGoMatches := workspaceActionMatches(text, "actions/setup-go")
		card.CheckoutActions = uniqueSortedStrings(checkoutMatches)
		card.CheckoutSteps = len(checkoutMatches)
		card.SetupGoActions = uniqueSortedStrings(setupGoMatches)
		card.SetupGoSteps = len(setupGoMatches)
		card.FetchDepthConfigured = strings.Contains(text, "fetch-depth:")
		workflows = append(workflows, card)
	}
	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].Path < workflows[j].Path
	})
	return workflows
}

func parseWorkspaceFrontmatter(text string) (workspaceSpecFrontmatter, bool) {
	var meta workspaceSpecFrontmatter
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
		return workspaceSpecFrontmatter{}, false
	}
	return meta, true
}

func workspaceFindings(surface workspaceSurface) []workspaceFinding {
	var findings []workspaceFinding
	if !surface.Policy.Present {
		findings = append(findings, workspaceFinding{"info", "workspace_policy_not_configured", workspacePolicyPath, "no workspace policy file is configured"})
	}
	if surface.Policy.Present && !workspacePolicyPathInContext() {
		findings = append(findings, workspaceFinding{"error", "workspace_policy_not_loaded", workspacePolicyPath, "workspace policy file is not in the model context allowlist"})
	}
	if len(surface.Specs) == 0 {
		findings = append(findings, workspaceFinding{"info", "workspace_specs_not_configured", workspaceSpecsDir, "no workspace specs are configured"})
	}
	for _, spec := range surface.Specs {
		if !spec.Frontmatter {
			findings = append(findings, workspaceFinding{"warning", "workspace_frontmatter_missing", spec.Path, "workspace spec should start with YAML frontmatter"})
		}
		if !strings.EqualFold(spec.Kind, "git-workspace") {
			findings = append(findings, workspaceFinding{"warning", "workspace_kind_not_git_workspace", spec.Path, "GitClaw v1 workspace specs should declare kind git-workspace"})
		}
		if !strings.EqualFold(spec.Runtime, "github-actions") {
			findings = append(findings, workspaceFinding{"warning", "workspace_runtime_not_github_actions", spec.Path, "GitClaw v1 workspaces should run in GitHub Actions"})
		}
		if !strings.EqualFold(spec.Storage, "repository-checkout") {
			findings = append(findings, workspaceFinding{"warning", "workspace_storage_not_checkout", spec.Path, "GitClaw v1 workspaces should use the repository checkout"})
		}
		if !strings.EqualFold(spec.Mode, "metadata-only") {
			findings = append(findings, workspaceFinding{"warning", "workspace_mode_not_metadata_only", spec.Path, "workspace reports must stay metadata-only"})
		}
		if strings.TrimSpace(spec.Root) != "." {
			findings = append(findings, workspaceFinding{"warning", "workspace_root_not_repo_root", spec.Path, "GitClaw v1 workspace root should be the checked-out repository root"})
		}
		if !strings.EqualFold(spec.Isolation, "ephemeral-actions-runner") {
			findings = append(findings, workspaceFinding{"warning", "workspace_isolation_not_ephemeral_runner", spec.Path, "GitClaw v1 relies on ephemeral GitHub-hosted runner isolation"})
		}
		if !strings.EqualFold(spec.DurableState, "git-tracked-files-and-backup-branch") {
			findings = append(findings, workspaceFinding{"warning", "workspace_durable_state_unreviewed", spec.Path, "durable state should stay in git-tracked files and the backup branch"})
		}
		if !spec.RequiresApproval {
			findings = append(findings, workspaceFinding{"warning", "workspace_approval_gate_missing", spec.Path, "workspace mutation or externalization must require reviewed approval"})
		}
	}
	if surface.Git.ErrorReason != "" {
		findings = append(findings, workspaceFinding{"warning", "workspace_git_unavailable", "git", "git workspace metadata could not be inspected"})
	}
	if surface.RepoFileListError != "" {
		findings = append(findings, workspaceFinding{"warning", "workspace_repo_files_unavailable", "repository", "repository file inventory could not be listed"})
	}
	if workspaceWorkflowsPresent(surface.Workflows) == 0 {
		findings = append(findings, workspaceFinding{"warning", "workspace_workflows_missing", ".github/workflows", "no configured GitClaw workflow files are present"})
	}
	if workspaceCheckoutWorkflows(surface.Workflows) == 0 {
		findings = append(findings, workspaceFinding{"warning", "workspace_checkout_missing", ".github/workflows", "no configured GitClaw workflow uses actions/checkout"})
	}
	return findings
}

func workspaceStatus(findings []workspaceFinding) string {
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

func workspacePolicyLoadedForModel(surface workspaceSurface) bool {
	return surface.Policy.Present && workspacePolicyPathInContext()
}

func workspacePolicyPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == workspacePolicyPath {
			return true
		}
	}
	return false
}

func workspaceSpecsWithFrontmatter(specs []workspaceSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.Frontmatter {
			count++
		}
	}
	return count
}

func workspaceSpecsRequiringApproval(specs []workspaceSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RequiresApproval {
			count++
		}
	}
	return count
}

func workspaceWorkflowsPresent(workflows []workspaceWorkflowCard) int {
	count := 0
	for _, workflow := range workflows {
		if workflow.Present {
			count++
		}
	}
	return count
}

func workspaceCheckoutWorkflows(workflows []workspaceWorkflowCard) int {
	count := 0
	for _, workflow := range workflows {
		if len(workflow.CheckoutActions) > 0 {
			count++
		}
	}
	return count
}

func workspaceCheckoutSteps(workflows []workspaceWorkflowCard) int {
	count := 0
	for _, workflow := range workflows {
		count += workflow.CheckoutSteps
	}
	return count
}

func workspaceCheckoutVersions(workflows []workspaceWorkflowCard) []string {
	var values []string
	for _, workflow := range workflows {
		values = append(values, workflow.CheckoutActions...)
	}
	return uniqueSortedStrings(values)
}

func workspaceSetupGoVersions(workflows []workspaceWorkflowCard) []string {
	var values []string
	for _, workflow := range workflows {
		values = append(values, workflow.SetupGoActions...)
	}
	return uniqueSortedStrings(values)
}

func workspaceFetchDepthConfigured(workflows []workspaceWorkflowCard) bool {
	for _, workflow := range workflows {
		if workflow.FetchDepthConfigured {
			return true
		}
	}
	return false
}

func workspaceActionMatches(text, action string) []string {
	pattern := regexp.MustCompile(regexp.QuoteMeta(action) + `@[-A-Za-z0-9_.]+`)
	return pattern.FindAllString(text, -1)
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
