package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const artifactPolicyPath = ".gitclaw/ARTIFACTS.md"
const artifactSpecsDir = ".gitclaw/artifacts"

type artifactSurface struct {
	Policy    configSurfaceFile
	Specs     []artifactSpecCard
	Workflows []artifactWorkflowCard
}

type artifactSpecCard struct {
	Name              string
	Path              string
	Present           bool
	Bytes             int
	Lines             int
	SHA               string
	Frontmatter       bool
	Kind              string
	Storage           string
	Filename          string
	Workflow          string
	Label             string
	RetentionDays     int
	RedactionRequired bool
	RequiresApproval  bool
}

type artifactSpecFrontmatter struct {
	Name              string `yaml:"name"`
	Kind              string `yaml:"kind"`
	Storage           string `yaml:"storage"`
	Filename          string `yaml:"filename"`
	Workflow          string `yaml:"workflow"`
	Label             string `yaml:"label"`
	RetentionDays     int    `yaml:"retention_days"`
	RedactionRequired bool   `yaml:"redaction_required"`
	RequiresApproval  bool   `yaml:"requires_approval"`
}

type artifactWorkflowCard struct {
	Path                    string
	Present                 bool
	Bytes                   int
	Lines                   int
	SHA                     string
	UploadArtifactActions   []string
	RetentionDays           []int
	IfNoFilesFoundError     bool
	PromptArtifactLabelGate bool
	PromptArtifactPathEnv   bool
}

type artifactFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsArtifactReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/artifacts" || command == "/artifact"
}

func RenderArtifactReport(ev Event, cfg Config) string {
	return renderArtifactReport(ev, cfg, true)
}

func RenderArtifactCLIReport(cfg Config) string {
	return renderArtifactReport(Event{}, cfg, false)
}

func renderArtifactReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectArtifactSurface(cfg.Workdir)
	findings := artifactFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Artifacts Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- artifacts_status: `%s`\n", artifactStatus(surface, findings))
	fmt.Fprintf(&b, "- artifact_policy_path: `%s`\n", artifactPolicyPath)
	fmt.Fprintf(&b, "- artifact_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- artifact_policy_loaded_for_model: `%t`\n", artifactPolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- artifact_specs_dir: `%s`\n", artifactSpecsDir)
	fmt.Fprintf(&b, "- artifact_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- artifact_specs_with_frontmatter: `%d`\n", artifactSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- artifact_specs_requiring_approval: `%d`\n", artifactSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- artifact_specs_requiring_redaction: `%d`\n", artifactSpecsRequiringRedaction(surface.Specs))
	fmt.Fprintf(&b, "- artifact_retention_days_declared: `%d`\n", artifactRetentionDaysDeclared(surface.Specs))
	fmt.Fprintf(&b, "- github_actions_artifact_uploaders: `%d`\n", len(surface.Workflows))
	fmt.Fprintf(&b, "- upload_artifact_versions: `%s`\n", inlineCode(strings.Join(artifactUploadVersions(surface.Workflows), ", ")))
	fmt.Fprintf(&b, "- prompt_artifact_default_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- prompt_artifact_label: `%s`\n", "gitclaw:e2e-prompt-artifact")
	fmt.Fprintf(&b, "- prompt_artifact_env_path_configured: `%t`\n", strings.TrimSpace(os.Getenv("GITCLAW_PROMPT_ARTIFACT_PATH")) != "")
	fmt.Fprintf(&b, "- artifact_storage_backend: `%s`\n", "github-actions-artifacts")
	fmt.Fprintf(&b, "- durable_backup_backend: `%s`\n", "git-backup-branch")
	fmt.Fprintf(&b, "- artifact_body_printing_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_artifact_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Artifacts describe reviewed evidence bundles. GitClaw treats artifact specs and workflow upload steps as metadata only: prompt dumps, tool outputs, transcripts, backups, channel payloads, and artifact bodies are not printed by this report.\n\n")

	b.WriteString("### Artifact Policy\n")
	writeConfigSurfaceFile(&b, surface.Policy)
	if artifactPolicyPathInContext() {
		b.WriteString("- `.gitclaw/ARTIFACTS.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/ARTIFACTS.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Artifact Specs\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, spec := range surface.Specs {
			fmt.Fprintf(
				&b,
				"- name=`%s` path=`%s` bytes=`%d` lines=`%d` frontmatter=`%t` kind=`%s` storage=`%s` filename=`%s` workflow=`%s` label=`%s` retention_days=`%d` redaction_required=`%t` requires_approval=`%t` sha256_12=`%s`\n",
				inlineCode(spec.Name),
				spec.Path,
				spec.Bytes,
				spec.Lines,
				spec.Frontmatter,
				inlineCode(spec.Kind),
				inlineCode(spec.Storage),
				inlineCode(spec.Filename),
				inlineCode(spec.Workflow),
				inlineCode(spec.Label),
				spec.RetentionDays,
				spec.RedactionRequired,
				spec.RequiresApproval,
				spec.SHA,
			)
		}
	}

	b.WriteString("\n### Workflow Artifact Uploads\n")
	if len(surface.Workflows) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, workflow := range surface.Workflows {
			fmt.Fprintf(
				&b,
				"- path=`%s` bytes=`%d` lines=`%d` upload_artifact_actions=`%s` retention_days=`%s` if_no_files_found_error=`%t` prompt_artifact_label_gate=`%t` prompt_artifact_path_env=`%t` sha256_12=`%s`\n",
				workflow.Path,
				workflow.Bytes,
				workflow.Lines,
				inlineCode(strings.Join(workflow.UploadArtifactActions, ", ")),
				inlineCode(joinInts(workflow.RetentionDays)),
				workflow.IfNoFilesFoundError,
				workflow.PromptArtifactLabelGate,
				workflow.PromptArtifactPathEnv,
				workflow.SHA,
			)
		}
	}

	b.WriteString("\n### Runtime Boundary\n")
	b.WriteString("- GitHub Actions artifacts are short-lived evidence bundles, not durable memory\n")
	b.WriteString("- the git backup branch is the durable transcript backup path\n")
	b.WriteString("- artifact specs do not enable uploads by themselves; reviewed workflows and gates do\n")
	b.WriteString("- future artifact types require body-free audit cards, redaction rules when needed, explicit retention, and live GitHub Models E2E coverage\n")

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

func inspectArtifactSurface(root string) artifactSurface {
	if root == "" {
		root = "."
	}
	surface := artifactSurface{
		Policy: inspectConfigSurfaceFile(root, artifactPolicyPath),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	surface.Specs = inspectArtifactSpecs(absRoot)
	surface.Workflows = inspectArtifactWorkflows(absRoot)
	return surface
}

func inspectArtifactSpecs(absRoot string) []artifactSpecCard {
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "artifacts", "*.md"))
	sort.Strings(matches)
	specs := make([]artifactSpecCard, 0, len(matches))
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
		spec := artifactSpecCard{
			Name:    strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)),
			Path:    relPath,
			Present: true,
			Bytes:   len(body),
			Lines:   lineCount(text),
			SHA:     shortDocumentHash(text),
		}
		if meta, ok := parseArtifactFrontmatter(text); ok {
			spec.Frontmatter = true
			if strings.TrimSpace(meta.Name) != "" {
				spec.Name = strings.TrimSpace(meta.Name)
			}
			spec.Kind = strings.TrimSpace(meta.Kind)
			spec.Storage = strings.TrimSpace(meta.Storage)
			spec.Filename = strings.TrimSpace(meta.Filename)
			spec.Workflow = strings.TrimSpace(meta.Workflow)
			spec.Label = strings.TrimSpace(meta.Label)
			spec.RetentionDays = meta.RetentionDays
			spec.RedactionRequired = meta.RedactionRequired
			spec.RequiresApproval = meta.RequiresApproval
		}
		specs = append(specs, spec)
	}
	return specs
}

func inspectArtifactWorkflows(absRoot string) []artifactWorkflowCard {
	workflows := make([]artifactWorkflowCard, 0, len(configWorkflowPaths))
	for _, path := range configWorkflowPaths {
		body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(path)))
		if err != nil {
			continue
		}
		text := string(body)
		actions := uploadArtifactActions(text)
		if len(actions) == 0 {
			continue
		}
		workflows = append(workflows, artifactWorkflowCard{
			Path:                    path,
			Present:                 true,
			Bytes:                   len(body),
			Lines:                   lineCount(text),
			SHA:                     shortDocumentHash(text),
			UploadArtifactActions:   actions,
			RetentionDays:           artifactWorkflowRetentionDays(text),
			IfNoFilesFoundError:     strings.Contains(text, "if-no-files-found: error"),
			PromptArtifactLabelGate: strings.Contains(text, "gitclaw:e2e-prompt-artifact"),
			PromptArtifactPathEnv:   strings.Contains(text, "GITCLAW_PROMPT_ARTIFACT_PATH"),
		})
	}
	return workflows
}

func parseArtifactFrontmatter(text string) (artifactSpecFrontmatter, bool) {
	var meta artifactSpecFrontmatter
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
		return artifactSpecFrontmatter{}, false
	}
	return meta, true
}

func artifactFindings(surface artifactSurface) []artifactFinding {
	var findings []artifactFinding
	if !surface.Policy.Present {
		findings = append(findings, artifactFinding{"info", "artifact_policy_not_configured", artifactPolicyPath, "no artifact policy file is configured"})
	}
	if surface.Policy.Present && !artifactPolicyPathInContext() {
		findings = append(findings, artifactFinding{"error", "artifact_policy_not_loaded", artifactPolicyPath, "artifact policy file is not in the model context allowlist"})
	}
	for _, spec := range surface.Specs {
		if !spec.Frontmatter {
			findings = append(findings, artifactFinding{"warning", "artifact_frontmatter_missing", spec.Path, "artifact spec should start with YAML frontmatter"})
		}
		if strings.TrimSpace(spec.Kind) == "" {
			findings = append(findings, artifactFinding{"warning", "artifact_kind_missing", spec.Path, "artifact spec should declare a kind such as prompt"})
		}
		if !strings.EqualFold(spec.Storage, "github-actions-artifact") {
			findings = append(findings, artifactFinding{"warning", "artifact_storage_not_actions", spec.Path, "GitClaw v1 artifact specs should use GitHub Actions artifact storage"})
		}
		if strings.TrimSpace(spec.Filename) == "" {
			findings = append(findings, artifactFinding{"warning", "artifact_filename_missing", spec.Path, "artifact spec should declare the uploaded filename"})
		}
		if strings.TrimSpace(spec.Workflow) == "" {
			findings = append(findings, artifactFinding{"warning", "artifact_workflow_missing", spec.Path, "artifact spec should declare the workflow that uploads it"})
		} else if !artifactWorkflowUploads(surface.Workflows, spec.Workflow) {
			findings = append(findings, artifactFinding{"warning", "artifact_workflow_upload_missing", spec.Path, "declared workflow is not currently detected as uploading GitHub Actions artifacts"})
		}
		if strings.TrimSpace(spec.Label) == "" {
			findings = append(findings, artifactFinding{"warning", "artifact_label_missing", spec.Path, "artifact spec should declare the label or gate that enables it"})
		}
		if spec.RetentionDays <= 0 {
			findings = append(findings, artifactFinding{"warning", "artifact_retention_missing", spec.Path, "artifact spec should declare a positive retention_days value"})
		}
		if !spec.RedactionRequired {
			findings = append(findings, artifactFinding{"warning", "artifact_redaction_not_required", spec.Path, "prompt or diagnostic artifacts should require redaction before upload"})
		}
		if !spec.RequiresApproval {
			findings = append(findings, artifactFinding{"warning", "artifact_approval_gate_missing", spec.Path, "artifact specs should require a reviewed approval or label gate"})
		}
	}
	for _, workflow := range surface.Workflows {
		if !artifactWorkflowUsesUploadV6(workflow) {
			findings = append(findings, artifactFinding{"warning", "artifact_upload_action_not_v6", workflow.Path, "workflow should use actions/upload-artifact@v6"})
		}
		if len(workflow.RetentionDays) == 0 {
			findings = append(findings, artifactFinding{"warning", "artifact_workflow_retention_missing", workflow.Path, "artifact upload step should declare retention-days"})
		}
		if !workflow.IfNoFilesFoundError {
			findings = append(findings, artifactFinding{"warning", "artifact_workflow_missing_if_no_files_found_error", workflow.Path, "artifact upload step should fail when expected files are missing"})
		}
	}
	return findings
}

func artifactStatus(surface artifactSurface, findings []artifactFinding) string {
	if !surface.Policy.Present && len(surface.Specs) == 0 && len(surface.Workflows) == 0 {
		return "not_configured"
	}
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

func artifactPolicyLoadedForModel(surface artifactSurface) bool {
	return surface.Policy.Present && artifactPolicyPathInContext()
}

func artifactPolicyPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == artifactPolicyPath {
			return true
		}
	}
	return false
}

func artifactSpecsWithFrontmatter(specs []artifactSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.Frontmatter {
			count++
		}
	}
	return count
}

func artifactSpecsRequiringApproval(specs []artifactSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RequiresApproval {
			count++
		}
	}
	return count
}

func artifactSpecsRequiringRedaction(specs []artifactSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RedactionRequired {
			count++
		}
	}
	return count
}

func artifactRetentionDaysDeclared(specs []artifactSpecCard) int {
	total := 0
	for _, spec := range specs {
		if spec.RetentionDays > 0 {
			total += spec.RetentionDays
		}
	}
	return total
}

func artifactUploadVersions(workflows []artifactWorkflowCard) []string {
	seen := map[string]bool{}
	var versions []string
	for _, workflow := range workflows {
		for _, action := range workflow.UploadArtifactActions {
			if seen[action] {
				continue
			}
			seen[action] = true
			versions = append(versions, action)
		}
	}
	sort.Strings(versions)
	if len(versions) == 0 {
		return []string{"none"}
	}
	return versions
}

func artifactWorkflowUploads(workflows []artifactWorkflowCard, path string) bool {
	for _, workflow := range workflows {
		if workflow.Path == path && len(workflow.UploadArtifactActions) > 0 {
			return true
		}
	}
	return false
}

func artifactWorkflowUsesUploadV6(workflow artifactWorkflowCard) bool {
	for _, action := range workflow.UploadArtifactActions {
		if action == "actions/upload-artifact@v6" {
			return true
		}
	}
	return false
}

func uploadArtifactActions(text string) []string {
	seen := map[string]bool{}
	var actions []string
	for _, line := range strings.Split(text, "\n") {
		idx := strings.Index(line, "actions/upload-artifact@")
		if idx < 0 {
			continue
		}
		action := strings.TrimSpace(line[idx:])
		action = strings.Trim(action, `"'`)
		if cut := strings.IndexAny(action, " \t#"); cut >= 0 {
			action = action[:cut]
		}
		action = strings.TrimRight(action, `"'`)
		if action == "" || seen[action] {
			continue
		}
		seen[action] = true
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return actions
}

func artifactWorkflowRetentionDays(text string) []int {
	seen := map[int]bool{}
	var days []int
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "retention-days:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "retention-days:"))
		value = strings.Trim(value, `"'`)
		day, err := strconv.Atoi(value)
		if err != nil || seen[day] {
			continue
		}
		seen[day] = true
		days = append(days, day)
	}
	sort.Ints(days)
	return days
}

func joinInts(values []int) string {
	if len(values) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}
