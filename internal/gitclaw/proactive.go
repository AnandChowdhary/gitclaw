package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

type ProactiveEnqueueOptions struct {
	Repo       string
	Name       string
	Slot       string
	Prompt     string
	PromptFile string
}

type ProactiveEnqueueResult struct {
	IssueNumber int
	IssueURL    string
	Name        string
	Slot        string
	Created     bool
}

func RunProactiveEnqueue(ctx context.Context, cfg Config, github ProactiveGitHubClient, opts ProactiveEnqueueOptions, now time.Time) (ProactiveEnqueueResult, error) {
	opts = normalizeProactiveOptions(opts, now)
	if opts.Prompt == "" && opts.PromptFile != "" {
		data, err := os.ReadFile(opts.PromptFile)
		if err != nil {
			return ProactiveEnqueueResult{}, fmt.Errorf("read proactive prompt file: %w", err)
		}
		opts.Prompt = strings.TrimSpace(string(data))
	}
	if err := validateProactiveOptions(opts); err != nil {
		return ProactiveEnqueueResult{}, err
	}

	issue, created, err := findOrCreateProactiveIssue(ctx, cfg, github, opts)
	if err != nil {
		return ProactiveEnqueueResult{}, err
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ProactiveLabel, cfg.TriggerLabel}); err != nil {
		return ProactiveEnqueueResult{}, fmt.Errorf("label proactive issue: %w", err)
	}
	return ProactiveEnqueueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(opts.Repo, issue.Number),
		Name:        opts.Name,
		Slot:        opts.Slot,
		Created:     created,
	}, nil
}

func normalizeProactiveOptions(opts ProactiveEnqueueOptions, now time.Time) ProactiveEnqueueOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Name = normalizeProactiveName(opts.Name)
	opts.Slot = strings.TrimSpace(opts.Slot)
	opts.Prompt = strings.TrimSpace(opts.Prompt)
	opts.PromptFile = strings.TrimSpace(opts.PromptFile)
	if opts.Slot == "" {
		if now.IsZero() {
			now = time.Now().UTC()
		}
		opts.Slot = now.UTC().Format("2006-01-02")
	}
	return opts
}

func normalizeProactiveName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func validateProactiveOptions(opts ProactiveEnqueueOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Name == "" {
		return fmt.Errorf("missing proactive name")
	}
	if opts.Slot == "" {
		return fmt.Errorf("missing proactive slot")
	}
	if opts.Prompt == "" {
		return fmt.Errorf("missing proactive prompt")
	}
	return nil
}

func findOrCreateProactiveIssue(ctx context.Context, cfg Config, github ProactiveGitHubClient, opts ProactiveEnqueueOptions) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.ProactiveLabel}, 100)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list proactive issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if proactiveRunMatches(issue.Body, opts.Name, opts.Slot) {
			return issue, false, nil
		}
	}

	title := fmt.Sprintf("GitClaw proactive %s %s", opts.Name, opts.Slot)
	issue, err := github.CreateIssue(ctx, opts.Repo, title, RenderProactiveRunBody(opts), nil)
	if err != nil {
		return Issue{}, false, fmt.Errorf("create proactive issue: %w", err)
	}
	return issue, true, nil
}

func RenderProactiveRunBody(opts ProactiveEnqueueOptions) string {
	return fmt.Sprintf(`<!-- gitclaw:proactive-run name="%s" slot="%s" -->
GitClaw proactive run.

- name: %s
- slot: %s

Proactive instruction:

%s`, escapeMarkerValue(opts.Name), escapeMarkerValue(opts.Slot), opts.Name, opts.Slot, strings.TrimSpace(opts.Prompt))
}

func proactiveRunMatches(body, name, slot string) bool {
	return HasProactiveRunMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`name="%s"`, escapeMarkerValue(name))) &&
		strings.Contains(body, fmt.Sprintf(`slot="%s"`, escapeMarkerValue(slot)))
}

func writeProactiveOutputs(result ProactiveEnqueueResult) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil
	}
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	defer file.Close()
	fmt.Fprintf(file, "issue_number=%d\n", result.IssueNumber)
	fmt.Fprintf(file, "issue_url=%s\n", result.IssueURL)
	fmt.Fprintf(file, "name=%s\n", result.Name)
	fmt.Fprintf(file, "slot=%s\n", result.Slot)
	fmt.Fprintf(file, "created=%t\n", result.Created)
	return nil
}
