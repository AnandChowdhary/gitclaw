package gitclaw

import (
	"context"
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
