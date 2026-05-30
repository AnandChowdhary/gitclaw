package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProactiveEnqueueCreatesIssueAndLabelsIt(t *testing.T) {
	github := &FakeGitHub{}
	result, err := RunProactiveEnqueue(context.Background(), DefaultConfig(), github, ProactiveEnqueueOptions{
		Repo:   "owner/repo",
		Name:   "Email Triage",
		Slot:   "2026-05-29",
		Prompt: "Summarize inbox and include PROACTIVE_TOKEN.",
	}, time.Time{})
	if err != nil {
		t.Fatalf("RunProactiveEnqueue returned error: %v", err)
	}
	if !result.Created || result.IssueNumber == 0 || result.Name != "email-triage" {
		t.Fatalf("unexpected enqueue result: %#v", result)
	}
	issue := github.Issues[0]
	if !HasProactiveRunMarker(issue.Body) || !strings.Contains(issue.Body, "PROACTIVE_TOKEN") {
		t.Fatalf("created issue missing proactive marker or prompt: %#v", issue)
	}
	labels := github.IssueLabels[result.IssueNumber]
	if !hasLabel(labels, "gitclaw") || !hasLabel(labels, "gitclaw:proactive") {
		t.Fatalf("proactive labels missing: %#v", labels)
	}
}

func TestProactiveEnqueueReusesSameSlotIssue(t *testing.T) {
	cfg := DefaultConfig()
	body := RenderProactiveRunBody(ProactiveEnqueueOptions{
		Name:   "email-triage",
		Slot:   "2026-05-29",
		Prompt: "existing prompt",
	})
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 9,
			Title:  "GitClaw proactive email-triage 2026-05-29",
			Body:   body,
			Labels: []string{cfg.ProactiveLabel},
		}},
	}
	result, err := RunProactiveEnqueue(context.Background(), cfg, github, ProactiveEnqueueOptions{
		Repo:   "owner/repo",
		Name:   "email-triage",
		Slot:   "2026-05-29",
		Prompt: "new prompt",
	}, time.Time{})
	if err != nil {
		t.Fatalf("RunProactiveEnqueue returned error: %v", err)
	}
	if result.Created || result.IssueNumber != 9 {
		t.Fatalf("same slot should reuse issue 9: %#v", result)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate proactive issue created: %#v", github.Issues)
	}
}

func TestProactiveEnqueueDefaultsSlotToUTCDate(t *testing.T) {
	opts := normalizeProactiveOptions(ProactiveEnqueueOptions{
		Repo:   "owner/repo",
		Name:   "Daily Check",
		Prompt: "Check daily state.",
	}, time.Date(2026, 5, 29, 23, 0, 0, 0, time.FixedZone("CEST", 2*60*60)))
	if opts.Slot != "2026-05-29" || opts.Name != "daily-check" {
		t.Fatalf("unexpected normalized opts: %#v", opts)
	}
}

func TestProactiveEnqueueSkipsBeforeNotBefore(t *testing.T) {
	github := &FakeGitHub{}
	result, err := RunProactiveEnqueue(context.Background(), DefaultConfig(), github, ProactiveEnqueueOptions{
		Repo:       "owner/repo",
		Name:       "Reminder",
		Slot:       "due-2026-05-30",
		PromptFile: filepath.Join(t.TempDir(), "missing.md"),
		NotBefore:  "2026-05-30T10:00:00Z",
	}, time.Date(2026, 5, 30, 9, 59, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("RunProactiveEnqueue returned error before due gate: %v", err)
	}
	if !result.Skipped || result.Due || result.IssueNumber != 0 || result.Name != "reminder" {
		t.Fatalf("unexpected skipped result: %#v", result)
	}
	if len(github.Issues) != 0 {
		t.Fatalf("not-before skip should not touch GitHub: %#v", github.Issues)
	}
}

func TestProactiveEnqueueCreatesAfterNotBefore(t *testing.T) {
	github := &FakeGitHub{}
	result, err := RunProactiveEnqueue(context.Background(), DefaultConfig(), github, ProactiveEnqueueOptions{
		Repo:      "owner/repo",
		Name:      "Reminder",
		Slot:      "due-2026-05-30",
		Prompt:    "Remind me to review the release checklist.",
		NotBefore: "2026-05-30",
	}, time.Date(2026, 5, 30, 0, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatalf("RunProactiveEnqueue returned error after due gate: %v", err)
	}
	if !result.Due || result.Skipped || !result.Created || result.IssueNumber == 0 {
		t.Fatalf("unexpected due result: %#v", result)
	}
	if len(github.Issues) != 1 || !strings.Contains(github.Issues[0].Body, "Remind me to review") {
		t.Fatalf("due enqueue did not create reminder issue: %#v", github.Issues)
	}
}

func TestProactiveEnqueueRejectsInvalidNotBefore(t *testing.T) {
	_, err := RunProactiveEnqueue(context.Background(), DefaultConfig(), &FakeGitHub{}, ProactiveEnqueueOptions{
		Repo:      "owner/repo",
		Name:      "Reminder",
		Slot:      "due-2026-05-30",
		Prompt:    "Review release checklist.",
		NotBefore: "tomorrow morning",
	}, time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "invalid proactive not-before time") {
		t.Fatalf("expected invalid not-before error, got %v", err)
	}
}

func TestProactiveInitWritesWorkflowAndPrompt(t *testing.T) {
	dir := t.TempDir()
	result, err := RunProactiveInit(ProactiveInitOptions{
		Root:       dir,
		Name:       "Email Triage",
		Cron:       "17 8 * * 1-5",
		PromptBody: "Summarize inbox state without leaking private data.",
		Skills:     []string{"repo-reader", "repo-reader"},
	})
	if err != nil {
		t.Fatalf("RunProactiveInit returned error: %v", err)
	}
	if result.Name != "email-triage" || !result.PromptWritten || !result.WorkflowWritten {
		t.Fatalf("unexpected init result: %#v", result)
	}

	prompt := readTestFile(t, filepath.Join(dir, ".gitclaw", "proactive", "email-triage.md"))
	for _, want := range []string{"gitclaw:proactive-skills repo-reader", "Suggested GitClaw skills", "- repo-reader", "Summarize inbox state without leaking private data."} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("generated prompt missing %q:\n%s", want, prompt)
		}
	}
	workflow := readTestFile(t, filepath.Join(dir, ".github", "workflows", "gitclaw-proactive-email-triage.yml"))
	for _, want := range []string{
		"name: GitClaw Proactive Email Triage",
		"workflow_dispatch:",
		"- cron: '17 8 * * 1-5'",
		"not_before:",
		"actions/checkout@v5",
		"actions/setup-go@v6",
		"go run ./cmd/gitclaw proactive enqueue",
		"--name 'email-triage'",
		"--prompt-file '.gitclaw/proactive/email-triage.md'",
		"steps.enqueue.outputs.issue_number != '' && steps.enqueue.outputs.issue_number != '0'",
		"GITCLAW_PROACTIVE_NOT_BEFORE",
		"gh workflow run .github/workflows/gitclaw.yml",
		`dispatch_id="proactive-email-triage-${GITCLAW_PROACTIVE_SLOT}"`,
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("generated workflow missing %q:\n%s", want, workflow)
		}
	}
	report := RenderProactiveInitReport(result)
	for _, want := range []string{
		"GitClaw Proactive Init Report",
		"mode: `apply`",
		"name: `email-triage`",
		"skill_hints: `1`",
		"skill_hint_names: `repo-reader`",
		"prompt_written: `true`",
		"workflow_written: `true`",
		"prompt_body_sha256_12:",
		"workflow_body_sha256_12:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("init report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "Summarize inbox state") {
		t.Fatalf("init report leaked prompt body:\n%s", report)
	}
}

func TestProactiveInitDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	result, err := RunProactiveInit(ProactiveInitOptions{
		Root:   dir,
		Name:   "Repo Watch",
		Cron:   "23 8 * * 1",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("RunProactiveInit returned error: %v", err)
	}
	if !result.DryRun || result.PromptWritten || result.WorkflowWritten {
		t.Fatalf("dry run should not write files: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "gitclaw-proactive-repo-watch.yml")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote workflow file or returned unexpected stat error: %v", err)
	}
}

func TestParseProactiveSkillHints(t *testing.T) {
	hints := parseProactiveSkillHints("<!-- gitclaw:proactive-skills repo-reader, deploy-watch -->\nbody")
	if got := strings.Join(hints, ","); got != "deploy-watch,repo-reader" {
		t.Fatalf("parseProactiveSkillHints() = %q", got)
	}
}

func TestProactiveInitRefusesDifferentExistingFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitclaw", "proactive")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "repo-watch.md"), []byte("custom prompt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RunProactiveInit(ProactiveInitOptions{
		Root: dir,
		Name: "Repo Watch",
		Cron: "23 8 * * 1",
	})
	if err == nil || !strings.Contains(err.Error(), "already exists with different content") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
}

func TestProactiveInitRejectsUnsafePathAndCron(t *testing.T) {
	for _, opts := range []ProactiveInitOptions{
		{Name: "Repo Watch", Cron: "23 8 * *", PromptPath: ".gitclaw/proactive/repo-watch.md"},
		{Name: "Repo Watch", Cron: "23 8 * * 1", PromptPath: "../repo-watch.md"},
		{Name: "Repo Watch", Cron: "23 8 * * 1", WorkflowPath: ".github/workflows/repo-watch.yaml"},
	} {
		if _, err := RunProactiveInit(opts); err == nil {
			t.Fatalf("RunProactiveInit allowed invalid options: %#v", opts)
		}
	}
}

func TestActiveSlashCommandFindsTriggeredBodyLine(t *testing.T) {
	ev := Event{
		Issue: Issue{
			Title: "GitClaw proactive repo-watch 2026-05-29",
			Body:  "Proactive instruction:\n\n@gitclaw /proactive\n\nHidden token must not leak.",
		},
	}
	if got := activeSlashCommand(ev, DefaultConfig()); got != "/proactive" {
		t.Fatalf("activeSlashCommand() = %q, want /proactive", got)
	}
}

func TestActiveSlashCommandIgnoresInlineMention(t *testing.T) {
	ev := Event{
		Issue: Issue{
			Title: "Regular issue",
			Body:  "Please do not treat this inline @gitclaw /proactive mention as the active command.",
		},
	}
	if got := activeSlashCommand(ev, DefaultConfig()); got != "" {
		t.Fatalf("activeSlashCommand() = %q, want empty command", got)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
