package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ProactiveInitOptions struct {
	Root         string
	Name         string
	Cron         string
	PromptPath   string
	PromptBody   string
	WorkflowPath string
	Skills       []string
	Force        bool
	DryRun       bool
}

type ProactiveInitResult struct {
	Root              string
	Name              string
	Cron              string
	PromptPath        string
	WorkflowPath      string
	PromptWritten     bool
	WorkflowWritten   bool
	PromptExisted     bool
	WorkflowExisted   bool
	Force             bool
	DryRun            bool
	Skills            []string
	PromptBodySHA     string
	WorkflowBodySHA   string
	PromptBodyBytes   int
	WorkflowBodyBytes int
}

func RunProactiveInit(opts ProactiveInitOptions) (ProactiveInitResult, error) {
	opts = normalizeProactiveInitOptions(opts)
	if err := validateProactiveInitOptions(opts); err != nil {
		return ProactiveInitResult{}, err
	}

	promptBody := proactivePromptBody(opts)
	workflowBody := RenderProactiveWorkflow(opts)
	result := ProactiveInitResult{
		Root:              opts.Root,
		Name:              opts.Name,
		Cron:              opts.Cron,
		PromptPath:        opts.PromptPath,
		WorkflowPath:      opts.WorkflowPath,
		Force:             opts.Force,
		DryRun:            opts.DryRun,
		Skills:            append([]string(nil), opts.Skills...),
		PromptBodySHA:     shortDocumentHash(promptBody),
		WorkflowBodySHA:   shortDocumentHash(workflowBody),
		PromptBodyBytes:   len([]byte(promptBody)),
		WorkflowBodyBytes: len([]byte(workflowBody)),
	}

	promptAbs, err := safeRepoPath(opts.Root, opts.PromptPath)
	if err != nil {
		return ProactiveInitResult{}, err
	}
	workflowAbs, err := safeRepoPath(opts.Root, opts.WorkflowPath)
	if err != nil {
		return ProactiveInitResult{}, err
	}

	if opts.DryRun {
		result.PromptExisted = fileExists(promptAbs)
		result.WorkflowExisted = fileExists(workflowAbs)
		return result, nil
	}

	wrote, existed, err := writeGeneratedFile(promptAbs, []byte(promptBody), opts.Force)
	if err != nil {
		return ProactiveInitResult{}, fmt.Errorf("write proactive prompt: %w", err)
	}
	result.PromptWritten = wrote
	result.PromptExisted = existed

	wrote, existed, err = writeGeneratedFile(workflowAbs, []byte(workflowBody), opts.Force)
	if err != nil {
		return ProactiveInitResult{}, fmt.Errorf("write proactive workflow: %w", err)
	}
	result.WorkflowWritten = wrote
	result.WorkflowExisted = existed
	return result, nil
}

func normalizeProactiveInitOptions(opts ProactiveInitOptions) ProactiveInitOptions {
	opts.Root = strings.TrimSpace(opts.Root)
	if opts.Root == "" {
		opts.Root = "."
	}
	opts.Name = normalizeProactiveName(opts.Name)
	opts.Cron = strings.TrimSpace(opts.Cron)
	opts.PromptPath = cleanRepoRelPath(opts.PromptPath)
	opts.WorkflowPath = cleanRepoRelPath(opts.WorkflowPath)
	opts.PromptBody = strings.TrimSpace(opts.PromptBody)
	opts.Skills = normalizeProactiveSkillHints(opts.Skills)
	if opts.Name != "" {
		if opts.PromptPath == "" {
			opts.PromptPath = ".gitclaw/proactive/" + opts.Name + ".md"
		}
		if opts.WorkflowPath == "" {
			opts.WorkflowPath = ".github/workflows/gitclaw-proactive-" + opts.Name + ".yml"
		}
	}
	return opts
}

func validateProactiveInitOptions(opts ProactiveInitOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("missing proactive name")
	}
	if err := validateProactiveCron(opts.Cron); err != nil {
		return err
	}
	if err := validateProactiveGeneratedPath(opts.PromptPath, ".md", "prompt"); err != nil {
		return err
	}
	if err := validateProactiveGeneratedPath(opts.WorkflowPath, ".yml", "workflow"); err != nil {
		return err
	}
	if _, err := safeRepoPath(opts.Root, opts.PromptPath); err != nil {
		return err
	}
	if _, err := safeRepoPath(opts.Root, opts.WorkflowPath); err != nil {
		return err
	}
	return nil
}

func validateProactiveCron(cron string) error {
	if cron == "" {
		return fmt.Errorf("missing proactive cron")
	}
	if strings.ContainsAny(cron, "\r\n'") {
		return fmt.Errorf("invalid proactive cron %q", cron)
	}
	if fields := strings.Fields(cron); len(fields) != 5 {
		return fmt.Errorf("invalid proactive cron %q: expected five fields", cron)
	}
	return nil
}

func validateProactiveGeneratedPath(path, suffix, name string) error {
	if path == "" {
		return fmt.Errorf("missing proactive %s path", name)
	}
	if !strings.HasSuffix(path, suffix) {
		return fmt.Errorf("proactive %s path must end with %s: %s", name, suffix, path)
	}
	return nil
}

func RenderProactiveInitReport(result ProactiveInitResult) string {
	mode := "apply"
	if result.DryRun {
		mode = "dry-run"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Proactive Init Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- mode: `%s`\n", mode)
	fmt.Fprintf(&b, "- name: `%s`\n", result.Name)
	fmt.Fprintf(&b, "- cron: `%s`\n", result.Cron)
	fmt.Fprintf(&b, "- prompt_file: `%s`\n", result.PromptPath)
	fmt.Fprintf(&b, "- workflow_file: `%s`\n", result.WorkflowPath)
	fmt.Fprintf(&b, "- force: `%t`\n", result.Force)
	fmt.Fprintf(&b, "- skill_hints: `%d`\n", len(result.Skills))
	fmt.Fprintf(&b, "- skill_hint_names: `%s`\n", inlineList(result.Skills))
	fmt.Fprintf(&b, "- prompt_written: `%t`\n", result.PromptWritten)
	fmt.Fprintf(&b, "- workflow_written: `%t`\n", result.WorkflowWritten)
	fmt.Fprintf(&b, "- prompt_existed: `%t`\n", result.PromptExisted)
	fmt.Fprintf(&b, "- workflow_existed: `%t`\n", result.WorkflowExisted)
	fmt.Fprintf(&b, "- prompt_body_bytes: `%d`\n", result.PromptBodyBytes)
	fmt.Fprintf(&b, "- workflow_body_bytes: `%d`\n", result.WorkflowBodyBytes)
	fmt.Fprintf(&b, "- prompt_body_sha256_12: `%s`\n", result.PromptBodySHA)
	fmt.Fprintf(&b, "- workflow_body_sha256_12: `%s`\n\n", result.WorkflowBodySHA)
	b.WriteString("The generated workflow creates or reuses a proactive issue, then dispatches the normal GitClaw issue workflow. Prompt bodies are not included in this report.\n\n")
	b.WriteString("### Generated Files\n")
	fmt.Fprintf(&b, "- `%s`\n", result.PromptPath)
	fmt.Fprintf(&b, "- `%s`\n", result.WorkflowPath)
	return strings.TrimSpace(b.String())
}

func RenderProactiveWorkflow(opts ProactiveInitOptions) string {
	title := proactiveDisplayName(opts.Name)
	return fmt.Sprintf(`name: GitClaw Proactive %s

on:
  workflow_dispatch:
    inputs:
      slot:
        description: Optional idempotency slot, defaults to current UTC date
        required: false
      not_before:
        description: Optional RFC3339 or YYYY-MM-DD due gate for reminder-style jobs
        required: false
  schedule:
    - cron: '%s'

concurrency:
  group: gitclaw-proactive-%s
  cancel-in-progress: false

jobs:
  enqueue:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions:
      actions: write
      contents: read
      issues: write
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 1

      - uses: actions/setup-go@v6
        with:
          go-version: stable

      - id: enqueue
        run: |
          set -euo pipefail
          slot="${GITCLAW_PROACTIVE_SLOT:-$(date -u +%%Y-%%m-%%d)}"
          go run ./cmd/gitclaw proactive enqueue \
            --name %s \
            --slot "$slot" \
            --prompt-file %s
        env:
          GH_TOKEN: ${{ github.token }}
          GITHUB_TOKEN: ${{ github.token }}
          GITCLAW_PROACTIVE_SLOT: ${{ github.event.inputs.slot }}
          GITCLAW_PROACTIVE_NOT_BEFORE: ${{ github.event.inputs.not_before }}

      - if: ${{ steps.enqueue.outputs.issue_number != '' && steps.enqueue.outputs.issue_number != '0' }}
        run: |
          set -euo pipefail
          gh workflow run .github/workflows/gitclaw.yml \
            --repo "$GITHUB_REPOSITORY" \
            -f issue_number="${GITCLAW_ISSUE_NUMBER}" \
            -f dispatch_id="proactive-%s-${GITCLAW_PROACTIVE_SLOT}" \
            -f reason="proactive:%s"
        env:
          GH_TOKEN: ${{ github.token }}
          GITHUB_TOKEN: ${{ github.token }}
          GITCLAW_ISSUE_NUMBER: ${{ steps.enqueue.outputs.issue_number }}
          GITCLAW_PROACTIVE_SLOT: ${{ steps.enqueue.outputs.slot }}
`, title, opts.Cron, opts.Name, shellQuote(opts.Name), shellQuote(opts.PromptPath), opts.Name, opts.Name)
}

func proactivePromptBody(opts ProactiveInitOptions) string {
	skillBlock := proactiveSkillHintBlock(opts.Skills)
	if opts.PromptBody != "" {
		if skillBlock == "" {
			return opts.PromptBody + "\n"
		}
		return skillBlock + strings.TrimSpace(opts.PromptBody) + "\n"
	}
	return fmt.Sprintf(`# GitClaw Proactive %s

%sReview the repository for useful proactive work. Open a concise issue thread only when there is something actionable to report.
`, proactiveDisplayName(opts.Name), skillBlock)
}

func proactiveSkillHintBlock(skills []string) string {
	skills = normalizeProactiveSkillHints(skills)
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:proactive-skills %s -->\n\n", strings.Join(skills, ", "))
	b.WriteString("Suggested GitClaw skills for this scheduled job:\n")
	for _, skill := range skills {
		fmt.Fprintf(&b, "- %s\n", skill)
	}
	b.WriteByte('\n')
	return b.String()
}

func normalizeProactiveSkillHints(skills []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range skills {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\t'
		}) {
			skill := strings.ToLower(cleanSkillLookupName(part))
			if skill == "" {
				continue
			}
			key := strings.ToLower(skill)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, skill)
		}
	}
	sort.Strings(out)
	return out
}

func proactiveDisplayName(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func cleanRepoRelPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeGeneratedFile(path string, body []byte, force bool) (bool, bool, error) {
	existed := false
	existing, err := os.ReadFile(path)
	if err == nil {
		existed = true
		if string(existing) == string(body) {
			return false, true, nil
		}
		if !force {
			return false, true, fmt.Errorf("%s already exists with different content; use --force to overwrite", path)
		}
	} else if !os.IsNotExist(err) {
		return false, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, existed, err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return false, existed, err
	}
	return true, existed, nil
}
