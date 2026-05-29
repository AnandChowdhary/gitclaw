package gitclaw

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreflightCommandWritesOutputsWithoutLLMSecret(t *testing.T) {
	dir := t.TempDir()
	eventPath := filepath.Join(dir, "event.json")
	outputPath := filepath.Join(dir, "output")
	eventJSON := `{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 42,
			"title": "@gitclaw explain auth",
			"body": "How does auth work?",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": []
		},
		"sender": {"login": "alice", "type": "User"}
	}`
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_EVENT_NAME", "issues")
	t.Setenv("GITHUB_OUTPUT", outputPath)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	if err := RunCLI(context.Background(), []string{"preflight", "--event", eventPath}); err != nil {
		t.Fatalf("preflight returned error: %v", err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "allowed=true") {
		t.Fatalf("GITHUB_OUTPUT missing allowed=true: %s", output)
	}
}

func TestSkillsValidateCommandReportsCurrentRepoShape(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SECRET_SKILL_BODY_TOKEN
`)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "validate"}); err != nil {
			t.Fatalf("skills validate returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Skills Validate Report", "skill_validation_status: `ok`", "skill_validation_errors: `0`", "skill_validation_warnings: `0`", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills validate output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "SECRET_SKILL_BODY_TOKEN") {
		t.Fatalf("skills validate leaked skill body:\n%s", output)
	}
}

func TestSoulValidateCommandReportsCurrentRepoShape(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SOUL.md", "SOUL_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/IDENTITY.md", "IDENTITY_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/USER.md", "USER_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "TOOLS_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "MEMORY_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/HEARTBEAT.md", "HEARTBEAT_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/memory/2026-05-29.md", "DATED_MEMORY_BODY_TOKEN")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"soul", "validate"}); err != nil {
			t.Fatalf("soul validate returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Soul Validate Report", "soul_validation_status: `ok`", "soul_validation_errors: `0`", "soul_validation_warnings: `0`", "soul_required_files_present: `6`", "soul_memory_notes: `1`", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("soul validate output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SOUL_BODY_TOKEN", "USER_BODY_TOKEN", "MEMORY_BODY_TOKEN", "DATED_MEMORY_BODY_TOKEN"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("soul validate leaked body token %q:\n%s", leaked, output)
		}
	}
}

func TestToolsValidateCommandReportsCurrentRepoShape(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "TOOLS_BODY_TOKEN")
	writeTestFile(t, dir, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "validate"}); err != nil {
			t.Fatalf("tools validate returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Tools Validate Report", "tool_validation_status: `ok`", "tool_validation_errors: `0`", "tool_validation_warnings: `0`", "tool_contracts: `5`", "tool_active_outputs: `1`", "tool_guidance_files: `1`", "tool_missing_guidance: `0`", "tool_duplicate_contracts: `0`", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools validate output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "TOOLS_BODY_TOKEN") || strings.Contains(output, "module github.com/AnandChowdhary/gitclaw") {
		t.Fatalf("tools validate leaked body/output token:\n%s", output)
	}
}

func TestBackupStatsCommandReportsFetchedBackupTree(t *testing.T) {
	dir := t.TempDir()
	writeBackupFixture(t, dir, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 7, Title: "@gitclaw cli stats", Body: "CLI_STATS_BODY_TOKEN"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "CLI_STATS_TRANSCRIPT_TOKEN"}},
	})
	if _, err := WriteBackupIndex(dir, "owner/repo", time.Date(2026, 5, 29, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"backup", "stats", "--root", dir, "--repo", "owner/repo"}); err != nil {
			t.Fatalf("backup stats returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Backup Stats Report", "backup_stats_status: `ok`", "backup_verify_status: `ok`", "issue_count: `1`", "transcript_messages: `1`", "latest_issue: `#7`", "raw_bodies_included: `false`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("backup stats output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "CLI_STATS_BODY_TOKEN") || strings.Contains(output, "CLI_STATS_TRANSCRIPT_TOKEN") || strings.Contains(output, "@gitclaw cli stats") {
		t.Fatalf("backup stats leaked body/title token:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}
