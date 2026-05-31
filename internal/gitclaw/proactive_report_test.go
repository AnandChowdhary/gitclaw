package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderProactiveListRequestMarksLiveLLME2EWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".github/workflows/gitclaw-proactive.yml", `name: GitClaw Proactive
on:
  workflow_dispatch:
  schedule:
    - cron: "23 8 * * 1"
PROACTIVE_LIST_MARKER_WORKFLOW_SECRET
`)
	writeTestFile(t, root, ".gitclaw/proactive/repo-hygiene.md", "PROACTIVE_LIST_MARKER_PROMPT_SECRET")

	cfg := DefaultConfig()
	cfg.Workdir = root
	body := RenderProactiveReport(Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 9,
			Title:  "@gitclaw /proactive list",
			Body:   "Hidden proactive list body token: PROACTIVE_LIST_MARKER_BODY_SECRET",
		},
	}, cfg)

	for _, want := range []string{
		"GitClaw Proactive Report",
		"repository: `owner/repo`",
		"issue: `#9`",
		"llm_e2e_required_after_proactive_report_change: `true`",
		"llm_e2e_required_after_proactive_list_change: `true`",
		"workflow_dispatch_trigger: `true`",
		"schedule_trigger: `true`",
		".gitclaw/proactive/repo-hygiene.md",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("proactive list report missing %q:\n%s", want, body)
		}
	}

	for _, notWant := range []string{
		"PROACTIVE_LIST_MARKER_BODY_SECRET",
		"PROACTIVE_LIST_MARKER_WORKFLOW_SECRET",
		"PROACTIVE_LIST_MARKER_PROMPT_SECRET",
	} {
		if strings.Contains(body, notWant) {
			t.Fatalf("proactive list report leaked %q:\n%s", notWant, body)
		}
	}
}
