package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const diffPolicyPath = ".gitclaw/DIFFS.md"
const diffSpecsDir = ".gitclaw/diffs"
const maxDiffFilesReturned = 200

type diffSurface struct {
	Policy configSurfaceFile
	Specs  []diffSpecCard
	Git    diffGitSurface
}

type diffSpecCard struct {
	Name             string
	Path             string
	Present          bool
	Bytes            int
	Lines            int
	SHA              string
	Frontmatter      bool
	Kind             string
	Source           string
	Mode             string
	MaxFiles         int
	RawPatchAllowed  bool
	RequiresApproval bool
}

type diffSpecFrontmatter struct {
	Name             string `yaml:"name"`
	Kind             string `yaml:"kind"`
	Source           string `yaml:"source"`
	Mode             string `yaml:"mode"`
	MaxFiles         int    `yaml:"max_files"`
	RawPatchAllowed  bool   `yaml:"raw_patch_allowed"`
	RequiresApproval bool   `yaml:"requires_approval"`
}

type diffGitSurface struct {
	Status                string
	GitAvailable          bool
	GitRepository         bool
	Root                  string
	Branch                string
	HeadShortSHA          string
	WorktreeClean         bool
	ChangedFiles          int
	StagedFiles           int
	UnstagedFiles         int
	UntrackedFiles        int
	RenamedFiles          int
	DeletedFiles          int
	StagedInsertions      int
	StagedDeletions       int
	UnstagedInsertions    int
	UnstagedDeletions     int
	BinaryDiffFiles       int
	FilesReturned         int
	RawDiffsIncluded      bool
	RawFileBodiesIncluded bool
	ErrorReason           string
	Files                 []diffFileCard
}

type diffFileCard struct {
	Path               string
	Status             string
	Staged             bool
	Unstaged           bool
	Untracked          bool
	Renamed            bool
	Deleted            bool
	StagedInsertions   int
	StagedDeletions    int
	UnstagedInsertions int
	UnstagedDeletions  int
	Binary             bool
	PathSHA            string
}

type diffFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsDiffReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/diffs" || command == "/diff" || command == "/changes"
}

func RenderDiffReport(ev Event, cfg Config) string {
	return renderDiffReport(ev, cfg, true)
}

func RenderDiffCLIReport(cfg Config) string {
	return renderDiffReport(Event{}, cfg, false)
}

func renderDiffReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectDiffSurface(cfg.Workdir)
	findings := diffFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Diffs Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- diff_status: `%s`\n", diffStatus(surface, findings))
	fmt.Fprintf(&b, "- diff_policy_path: `%s`\n", diffPolicyPath)
	fmt.Fprintf(&b, "- diff_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- diff_policy_loaded_for_model: `%t`\n", diffPolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- diff_specs_dir: `%s`\n", diffSpecsDir)
	fmt.Fprintf(&b, "- diff_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- diff_specs_with_frontmatter: `%d`\n", diffSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- diff_specs_requiring_approval: `%d`\n", diffSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- diff_specs_disallowing_raw_patch: `%d`\n", diffSpecsDisallowingRawPatch(surface.Specs))
	fmt.Fprintf(&b, "- git_available: `%t`\n", surface.Git.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", surface.Git.GitRepository)
	fmt.Fprintf(&b, "- worktree_root: `%s`\n", inlineCode(surface.Git.Root))
	fmt.Fprintf(&b, "- branch: `%s`\n", inlineCode(surface.Git.Branch))
	fmt.Fprintf(&b, "- head_commit: `%s`\n", surface.Git.HeadShortSHA)
	fmt.Fprintf(&b, "- worktree_clean: `%t`\n", surface.Git.WorktreeClean)
	fmt.Fprintf(&b, "- changed_files: `%d`\n", surface.Git.ChangedFiles)
	fmt.Fprintf(&b, "- staged_files: `%d`\n", surface.Git.StagedFiles)
	fmt.Fprintf(&b, "- unstaged_files: `%d`\n", surface.Git.UnstagedFiles)
	fmt.Fprintf(&b, "- untracked_files: `%d`\n", surface.Git.UntrackedFiles)
	fmt.Fprintf(&b, "- renamed_files: `%d`\n", surface.Git.RenamedFiles)
	fmt.Fprintf(&b, "- deleted_files: `%d`\n", surface.Git.DeletedFiles)
	fmt.Fprintf(&b, "- staged_insertions: `%d`\n", surface.Git.StagedInsertions)
	fmt.Fprintf(&b, "- staged_deletions: `%d`\n", surface.Git.StagedDeletions)
	fmt.Fprintf(&b, "- unstaged_insertions: `%d`\n", surface.Git.UnstagedInsertions)
	fmt.Fprintf(&b, "- unstaged_deletions: `%d`\n", surface.Git.UnstagedDeletions)
	fmt.Fprintf(&b, "- binary_diff_files: `%d`\n", surface.Git.BinaryDiffFiles)
	fmt.Fprintf(&b, "- diff_file_limit: `%d`\n", maxDiffFilesReturned)
	fmt.Fprintf(&b, "- diff_files_returned: `%d`\n", surface.Git.FilesReturned)
	fmt.Fprintf(&b, "- raw_diffs_included: `%t`\n", surface.Git.RawDiffsIncluded)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", surface.Git.RawFileBodiesIncluded)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if surface.Git.ErrorReason != "" {
		fmt.Fprintf(&b, "- error_reason: `%s`\n", surface.Git.ErrorReason)
	}
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Diffs are reported as git metadata only. Raw patch hunks, file bodies, issue/comment bodies, prompts, tool outputs, backup payloads, and secrets are not printed by this report.\n\n")

	b.WriteString("### Diff Policy\n")
	writeConfigSurfaceFile(&b, surface.Policy)
	if diffPolicyPathInContext() {
		b.WriteString("- `.gitclaw/DIFFS.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/DIFFS.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Diff Specs\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, spec := range surface.Specs {
			fmt.Fprintf(
				&b,
				"- name=`%s` path=`%s` bytes=`%d` lines=`%d` frontmatter=`%t` kind=`%s` source=`%s` mode=`%s` max_files=`%d` raw_patch_allowed=`%t` requires_approval=`%t` sha256_12=`%s`\n",
				inlineCode(spec.Name),
				spec.Path,
				spec.Bytes,
				spec.Lines,
				spec.Frontmatter,
				inlineCode(spec.Kind),
				inlineCode(spec.Source),
				inlineCode(spec.Mode),
				spec.MaxFiles,
				spec.RawPatchAllowed,
				spec.RequiresApproval,
				spec.SHA,
			)
		}
	}

	b.WriteString("\n### Changed Files\n")
	if len(surface.Git.Files) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, file := range surface.Git.Files {
			fmt.Fprintf(
				&b,
				"- path=`%s` status=`%s` staged=`%t` unstaged=`%t` untracked=`%t` renamed=`%t` deleted=`%t` staged_insertions=`%d` staged_deletions=`%d` unstaged_insertions=`%d` unstaged_deletions=`%d` binary=`%t` path_sha256_12=`%s`\n",
				inlineCode(file.Path),
				inlineCode(file.Status),
				file.Staged,
				file.Unstaged,
				file.Untracked,
				file.Renamed,
				file.Deleted,
				file.StagedInsertions,
				file.StagedDeletions,
				file.UnstagedInsertions,
				file.UnstagedDeletions,
				file.Binary,
				file.PathSHA,
			)
		}
	}

	b.WriteString("\n### Runtime Boundary\n")
	b.WriteString("- `/diffs` is inspect-only and never stages, resets, restores, commits, or opens pull requests\n")
	b.WriteString("- raw patches belong in explicit artifacts, pull requests, or local export commands, not issue-visible status reports\n")
	b.WriteString("- future diff rendering needs reviewed workflows, body-free audit cards, retention/redaction decisions, and live GitHub Models E2E coverage\n")

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

func inspectDiffSurface(root string) diffSurface {
	if root == "" {
		root = "."
	}
	surface := diffSurface{
		Policy: inspectConfigSurfaceFile(root, diffPolicyPath),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		surface.Git = diffGitSurface{Status: "unavailable", Root: ".", ErrorReason: "workdir_abs_failed"}
		return surface
	}
	surface.Specs = inspectDiffSpecs(absRoot)
	surface.Git = inspectDiffGit(absRoot)
	return surface
}

func inspectDiffSpecs(absRoot string) []diffSpecCard {
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "diffs", "*.md"))
	sort.Strings(matches)
	specs := make([]diffSpecCard, 0, len(matches))
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
		spec := diffSpecCard{
			Name:    strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)),
			Path:    relPath,
			Present: true,
			Bytes:   len(body),
			Lines:   lineCount(text),
			SHA:     shortDocumentHash(text),
		}
		if meta, ok := parseDiffFrontmatter(text); ok {
			spec.Frontmatter = true
			if strings.TrimSpace(meta.Name) != "" {
				spec.Name = strings.TrimSpace(meta.Name)
			}
			spec.Kind = strings.TrimSpace(meta.Kind)
			spec.Source = strings.TrimSpace(meta.Source)
			spec.Mode = strings.TrimSpace(meta.Mode)
			spec.MaxFiles = meta.MaxFiles
			spec.RawPatchAllowed = meta.RawPatchAllowed
			spec.RequiresApproval = meta.RequiresApproval
		}
		specs = append(specs, spec)
	}
	return specs
}

func inspectDiffGit(absRoot string) diffGitSurface {
	report := diffGitSurface{
		Status:                "unavailable",
		Root:                  ".",
		RawDiffsIncluded:      false,
		RawFileBodiesIncluded: false,
	}
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
	if branch, err := runDiffGit(absRoot, "branch", "--show-current"); err == nil && strings.TrimSpace(branch) != "" {
		report.Branch = strings.TrimSpace(branch)
	} else {
		report.Branch = "(detached)"
	}
	if short, err := runDiffGit(absRoot, "rev-parse", "--short=12", "HEAD"); err == nil {
		report.HeadShortSHA = strings.TrimSpace(short)
	}

	status, err := runDiffGit(absRoot, "status", "--porcelain=v1")
	if err != nil {
		report.ErrorReason = "git_status_failed"
		return report
	}
	files := parseDiffStatus(status)
	applyDiffNumstat(files, "staged", mustRunDiffGit(absRoot, "diff", "--cached", "--numstat", "--"))
	applyDiffNumstat(files, "unstaged", mustRunDiffGit(absRoot, "diff", "--numstat", "--"))
	report.Files = sortedDiffFiles(files, maxDiffFilesReturned)
	report.FilesReturned = len(report.Files)
	report.ChangedFiles = len(files)
	for _, file := range files {
		if file.Staged {
			report.StagedFiles++
		}
		if file.Unstaged {
			report.UnstagedFiles++
		}
		if file.Untracked {
			report.UntrackedFiles++
		}
		if file.Renamed {
			report.RenamedFiles++
		}
		if file.Deleted {
			report.DeletedFiles++
		}
		if file.Binary {
			report.BinaryDiffFiles++
		}
		report.StagedInsertions += file.StagedInsertions
		report.StagedDeletions += file.StagedDeletions
		report.UnstagedInsertions += file.UnstagedInsertions
		report.UnstagedDeletions += file.UnstagedDeletions
	}
	report.WorktreeClean = report.ChangedFiles == 0
	report.Status = "clean"
	if !report.WorktreeClean {
		report.Status = "dirty"
	}
	return report
}

func parseDiffFrontmatter(text string) (diffSpecFrontmatter, bool) {
	var meta diffSpecFrontmatter
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
		return diffSpecFrontmatter{}, false
	}
	return meta, true
}

func diffFindings(surface diffSurface) []diffFinding {
	var findings []diffFinding
	if !surface.Policy.Present {
		findings = append(findings, diffFinding{"info", "diff_policy_not_configured", diffPolicyPath, "no diff policy file is configured"})
	}
	if surface.Policy.Present && !diffPolicyPathInContext() {
		findings = append(findings, diffFinding{"error", "diff_policy_not_loaded", diffPolicyPath, "diff policy file is not in the model context allowlist"})
	}
	for _, spec := range surface.Specs {
		if !spec.Frontmatter {
			findings = append(findings, diffFinding{"warning", "diff_frontmatter_missing", spec.Path, "diff spec should start with YAML frontmatter"})
		}
		if strings.TrimSpace(spec.Kind) == "" {
			findings = append(findings, diffFinding{"warning", "diff_kind_missing", spec.Path, "diff spec should declare a kind such as git-diff"})
		}
		if !strings.EqualFold(spec.Source, "git-worktree") {
			findings = append(findings, diffFinding{"warning", "diff_source_not_git_worktree", spec.Path, "GitClaw v1 diff specs should use the checked-out git worktree"})
		}
		if !strings.EqualFold(spec.Mode, "metadata-only") {
			findings = append(findings, diffFinding{"warning", "diff_mode_not_metadata_only", spec.Path, "GitClaw v1 diff reports must stay metadata-only"})
		}
		if spec.MaxFiles <= 0 {
			findings = append(findings, diffFinding{"warning", "diff_max_files_missing", spec.Path, "diff spec should declare a positive max_files value"})
		}
		if spec.RawPatchAllowed {
			findings = append(findings, diffFinding{"warning", "diff_raw_patch_allowed", spec.Path, "issue-visible diff reports should not allow raw patches"})
		}
		if !spec.RequiresApproval {
			findings = append(findings, diffFinding{"warning", "diff_approval_gate_missing", spec.Path, "diff rendering or patch export should require reviewed approval"})
		}
	}
	if surface.Git.ErrorReason != "" {
		findings = append(findings, diffFinding{"warning", "diff_git_unavailable", "git", "git metadata could not be inspected"})
	}
	return findings
}

func diffStatus(surface diffSurface, findings []diffFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "error"
		}
	}
	if surface.Git.Status != "" {
		return surface.Git.Status
	}
	return "unavailable"
}

func diffPolicyLoadedForModel(surface diffSurface) bool {
	return surface.Policy.Present && diffPolicyPathInContext()
}

func diffPolicyPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == diffPolicyPath {
			return true
		}
	}
	return false
}

func diffSpecsWithFrontmatter(specs []diffSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.Frontmatter {
			count++
		}
	}
	return count
}

func diffSpecsRequiringApproval(specs []diffSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RequiresApproval {
			count++
		}
	}
	return count
}

func diffSpecsDisallowingRawPatch(specs []diffSpecCard) int {
	count := 0
	for _, spec := range specs {
		if !spec.RawPatchAllowed {
			count++
		}
	}
	return count
}

func parseDiffStatus(status string) map[string]*diffFileCard {
	files := map[string]*diffFileCard{}
	for _, line := range strings.Split(strings.TrimRight(status, "\n"), "\n") {
		if len(line) < 3 {
			continue
		}
		code := line[:2]
		path := strings.TrimSpace(line[3:])
		if path == "" {
			continue
		}
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			path = parts[len(parts)-1]
		}
		file := diffFile(files, path)
		file.Status = code
		if code == "??" {
			file.Untracked = true
			continue
		}
		if code[0] != ' ' {
			file.Staged = true
		}
		if code[1] != ' ' {
			file.Unstaged = true
		}
		if strings.ContainsAny(code, "R") {
			file.Renamed = true
		}
		if strings.ContainsAny(code, "D") {
			file.Deleted = true
		}
	}
	return files
}

func applyDiffNumstat(files map[string]*diffFileCard, scope string, numstat string) {
	for _, line := range strings.Split(strings.TrimSpace(numstat), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		path := parts[2]
		if len(parts) >= 4 {
			path = parts[len(parts)-1]
		}
		file := diffFile(files, path)
		if parts[0] == "-" || parts[1] == "-" {
			file.Binary = true
			continue
		}
		insertions, _ := strconv.Atoi(parts[0])
		deletions, _ := strconv.Atoi(parts[1])
		if scope == "staged" {
			file.Staged = true
			file.StagedInsertions += insertions
			file.StagedDeletions += deletions
		} else {
			file.Unstaged = true
			file.UnstagedInsertions += insertions
			file.UnstagedDeletions += deletions
		}
	}
}

func diffFile(files map[string]*diffFileCard, path string) *diffFileCard {
	if file, ok := files[path]; ok {
		return file
	}
	file := &diffFileCard{Path: path, PathSHA: shortDocumentHash(path)}
	files[path] = file
	return file
}

func sortedDiffFiles(files map[string]*diffFileCard, limit int) []diffFileCard {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if limit > 0 && len(paths) > limit {
		paths = paths[:limit]
	}
	out := make([]diffFileCard, 0, len(paths))
	for _, path := range paths {
		out = append(out, *files[path])
	}
	return out
}

func runDiffGit(root string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", cmdArgs...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(data)))
	}
	return string(data), nil
}

func mustRunDiffGit(root string, args ...string) string {
	out, err := runDiffGit(root, args...)
	if err != nil {
		return ""
	}
	return out
}
