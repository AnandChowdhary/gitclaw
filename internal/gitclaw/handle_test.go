package gitclaw

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleDryRunPostsExactlyOneIdempotentComment(t *testing.T) {
	eventJSON := []byte(`{
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
		"sender": {"login": "alice", "type": "User"},
		"after": "abc123"
	}`)
	ev, err := ParseEvent("issues", eventJSON)
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{
		CommentsByIssue: map[int][]Comment{42: nil},
	}
	llm := &FakeLLM{Response: "Auth is handled by the repo code."}
	err = Handle(context.Background(), ev, DefaultConfig(), github, llm)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	if !strings.Contains(github.Posted[0].Body, "Auth is handled") {
		t.Fatalf("posted body missing LLM response: %s", github.Posted[0].Body)
	}

	err = Handle(context.Background(), ev, DefaultConfig(), github, llm)
	if err != nil {
		t.Fatalf("second Handle returned error: %v", err)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("idempotent retry posted %d comments, want still 1", len(github.Posted))
	}
	if llm.Calls != 1 {
		t.Fatalf("LLM called %d times, want 1 due existing idempotency marker", llm.Calls)
	}
	if !hasLabel(github.IssueLabels[42], "gitclaw:done") || hasLabel(github.IssueLabels[42], "gitclaw:running") || hasLabel(github.IssueLabels[42], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[42])
	}
}

func TestHandleSkipsUntrustedBeforeLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 2,
			"title": "@gitclaw explain",
			"body": "",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 55,
			"body": "run this",
			"author_association": "CONTRIBUTOR",
			"user": {"login": "mallory", "type": "User"}
		},
		"sender": {"login": "mallory", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{2: nil}}
	llm := &FakeLLM{Response: "should not be used"}
	err = Handle(context.Background(), ev, DefaultConfig(), github, llm)
	if err == nil {
		t.Fatalf("Handle should return preflight rejection")
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called for untrusted actor")
	}
	if len(github.Posted) != 0 {
		t.Fatalf("posted comments for untrusted actor: %#v", github.Posted)
	}
}

func TestHandlePassesRepoContextToLLM(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/AnandChowdhary/gitclaw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".gitclaw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitclaw", "SOUL.md"), []byte("Be repo-native."), 0o600); err != nil {
		t.Fatal(err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 88,
			"title": "@gitclaw inspect go.mod",
			"body": "What module path is in go.mod?",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{88: nil}}
	llm := &FakeLLM{Response: "module path found"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !hasContextDoc(llm.LastRequest.Context.Documents, ".gitclaw/SOUL.md", "repo-native") {
		t.Fatalf("LLM request missing soul context: %#v", llm.LastRequest.Context.Documents)
	}
	if !hasToolOutput(llm.LastRequest.Context.ToolOutputs, "gitclaw.read_file", "go.mod", "module github.com/AnandChowdhary/gitclaw") {
		t.Fatalf("LLM request missing read_file tool output: %#v", llm.LastRequest.Context.ToolOutputs)
	}
}

func TestHandleContextCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/AnandChowdhary/gitclaw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".gitclaw", "SKILLS", "repo-reader"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitclaw", "SOUL.md"), []byte("Be repo-native."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitclaw", "SKILLS", "repo-reader", "SKILL.md"), []byte(`---
name: repo-reader
description: Read repository files.
---

Skill token.
`), 0o600); err != nil {
		t.Fatal(err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 90,
			"title": "@gitclaw /context",
			"body": "Please inspect go.mod with the repo-reader skill.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{90: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic context command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Context Report", "Generated without a model call", ".gitclaw/SOUL.md", ".gitclaw/SKILLS/repo-reader/SKILL.md", "gitclaw.list_files", "gitclaw.read_file", "model=\"gitclaw/context\""} {
		if !strings.Contains(body, want) {
			t.Fatalf("context report missing %q:\n%s", want, body)
		}
	}
	if !hasLabel(github.IssueLabels[90], "gitclaw:done") || hasLabel(github.IssueLabels[90], "gitclaw:running") || hasLabel(github.IssueLabels[90], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[90])
	}
}

func TestHandleBackupCommandPostsReportWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 96,
			"title": "@gitclaw /backup",
			"body": "Hidden backup token: BACKUP_SECRET_TOKEN.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{96: {
		{
			ID:                21,
			Body:              "<!-- gitclaw:assistant-turn idempotency_key=old -->\nAssistant body token.",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
	}}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic backup command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Backup Report", "Generated without a model call", "model=\"gitclaw/backup\"", "backup_branch: `gitclaw-backups`", ".gitclaw/backups/owner__repo/issues/000096.json", ".gitclaw/backups/owner__repo/index.json", ".gitclaw/backups/owner__repo/README.md", "raw_comments_now: `1`", "transcript_messages_now: `2`", "assistant_turn_comments_now: `1`", "backup_schema_version: `1`", "gitclaw backup verify --root .gitclaw/backups --repo <owner/repo>", "traversal-safe payload paths"} {
		if !strings.Contains(body, want) {
			t.Fatalf("backup report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "BACKUP_SECRET_TOKEN") || strings.Contains(body, "Assistant body token") {
		t.Fatalf("backup report leaked body token:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[96], "gitclaw:done") || hasLabel(github.IssueLabels[96], "gitclaw:running") || hasLabel(github.IssueLabels[96], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[96])
	}
}

func TestHandleProactiveCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".github/workflows/gitclaw-proactive.yml", `name: GitClaw Proactive
on:
  workflow_dispatch:
  schedule:
    - cron: "23 8 * * 1"
`)
	writeTestFile(t, root, ".gitclaw/proactive/repo-hygiene.md", "Proactive prompt token: PROACTIVE_PROMPT_SECRET.")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 97,
			"title": "@gitclaw /proactive",
			"body": "Hidden proactive token: PROACTIVE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{97: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic proactive command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Proactive Report", "Generated without a model call", "model=\"gitclaw/proactive\"", "workflow_present: `true`", "workflow_dispatch_trigger: `true`", "schedule_trigger: `true`", "prompt_files: `1`", ".github/workflows/gitclaw-proactive.yml", ".gitclaw/proactive/repo-hygiene.md", "gitclaw proactive enqueue"} {
		if !strings.Contains(body, want) {
			t.Fatalf("proactive report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "PROACTIVE_BODY_SECRET") || strings.Contains(body, "PROACTIVE_PROMPT_SECRET") {
		t.Fatalf("proactive report leaked body token:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[97], "gitclaw:done") || hasLabel(github.IssueLabels[97], "gitclaw:running") || hasLabel(github.IssueLabels[97], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[97])
	}
}

func TestHandleModelsCommandPostsReportWithoutLLM(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITCLAW_LLM_API_KEY", "")
	t.Setenv("GITCLAW_LLM_BASE_URL", "")
	t.Setenv("GITCLAW_LLM_MAX_ATTEMPTS", "6")
	t.Setenv("GITCLAW_LLM_TIMEOUT_SECONDS", "75")
	t.Setenv("GITCLAW_LLM_RETRY_BASE_DELAY_SECONDS", "10")
	t.Setenv("GITCLAW_LLM_RETRY_MAX_DELAY_SECONDS", "90")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 98,
			"title": "@gitclaw /models",
			"body": "Hidden model token: MODEL_REPORT_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{98: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic models command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Model Report", "Generated without a model call", "model=\"gitclaw/models\"", "provider: `github-models`", "model: `openai/gpt-5-mini`", "endpoint_host: `models.github.ai`", "token_source: `GITHUB_TOKEN`", "request_timeout_seconds: `75`", "retry_max_attempts: `6`", "retry_base_delay_seconds: `10`", "retry_max_delay_seconds: `90`", "retryable_statuses: `429, 408, 5xx`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("model report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "MODEL_REPORT_SECRET") || strings.Contains(body, "github-token") {
		t.Fatalf("model report leaked sensitive value:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[98], "gitclaw:done") || hasLabel(github.IssueLabels[98], "gitclaw:running") || hasLabel(github.IssueLabels[98], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[98])
	}
}

func TestHandleConfigCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", "name: GitClaw\n")
	writeTestFile(t, root, ".github/workflows/gitclaw-heartbeat.yml", "name: GitClaw Heartbeat\n")
	writeTestFile(t, root, ".gitclaw/config.yml", `trigger:
  label: gitclaw
  prefix: "@gitclaw"

model:
  max_prompt_bytes: 60000
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 100,
			"title": "@gitclaw /config",
			"body": "Hidden config token: CONFIG_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	cfg, err = LoadConfigFromWorkdir(cfg)
	if err != nil {
		t.Fatalf("LoadConfigFromWorkdir returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{100: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic config command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Config Report", "Generated without a model call", "model=\"gitclaw/config\"", "config_source: `defaults+repo`", "config_file_path: `.gitclaw/config.yml`", "config_file_present: `true`", "trigger_label: `gitclaw`", "trigger_prefix: `@gitclaw`", "run_mode: `read-only`", "max_prompt_bytes: `60000`", "max_output_tokens: `4000`", "workflows_present: `2`", "slash_commands: `15`", "OWNER", "COLLABORATOR", "gitclaw:disabled", "/channels", "/doctor", "/help", "/memory", "/models", "/prompt", "/config", ".github/workflows/gitclaw.yml", ".github/workflows/gitclaw-heartbeat.yml", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("config report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "CONFIG_BODY_SECRET") {
		t.Fatalf("config report leaked sensitive value:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[100], "gitclaw:done") || hasLabel(github.IssueLabels[100], "gitclaw:running") || hasLabel(github.IssueLabels[100], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[100])
	}
}

func TestHandleChannelsCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".github/workflows/gitclaw-channel-ingest.yml", `name: GitClaw Channel Ingest

on:
  workflow_dispatch:
    inputs:
      channel:
        required: true
      thread_id:
        required: true
      message_id:
        required: true
      author:
        required: false
      body:
        required: true

jobs:
  ingest:
    permissions:
      actions: write
      issues: write
    steps:
      - run: echo WORKFLOW_CHANNEL_SECRET
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 101,
			"title": "@gitclaw /channels",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-123\" -->\nHidden channel body token: CHANNEL_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{
		101: {{
			ID: 55,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-123",
				MessageID: "message-456",
				Author:    "telegram:alice",
				Body:      "Hidden mirrored token: CHANNEL_MESSAGE_SECRET.",
			}),
		}},
	}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic channels command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Channel Report", "Generated without a model call", "model=\"gitclaw/channels\"", "channel_label: `gitclaw:channel`", "trigger_label: `gitclaw`", "workflow_present: `true`", "workflow_dispatch_trigger: `true`", "permissions_actions_write: `true`", "permissions_issues_write: `true`", "workflow_inputs: `5`", "channel_thread_issue: `true`", "channel_message_comments_now: `1`", "supported_providers: `telegram, slack, generic`", "wake_strategy: `workflow_dispatch`", ".github/workflows/gitclaw-channel-ingest.yml", "telegram", "slack", "generic", "gitclaw channel-ingest", "dispatch id: `<channel>-<message_id>`", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("channel report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "CHANNEL_BODY_SECRET") || strings.Contains(body, "CHANNEL_MESSAGE_SECRET") || strings.Contains(body, "WORKFLOW_CHANNEL_SECRET") {
		t.Fatalf("channel report leaked sensitive value:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[101], "gitclaw:done") || hasLabel(github.IssueLabels[101], "gitclaw:running") || hasLabel(github.IssueLabels[101], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[101])
	}
}

func TestHandleSkillsCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gitclaw", "SKILLS", "repo-reader"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".gitclaw", "SKILLS", "always-on"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitclaw", "SKILLS", "repo-reader", "SKILL.md"), []byte(`---
name: repo-reader
description: Read repository files.
---

Skill token.
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitclaw", "SKILLS", "always-on", "SKILL.md"), []byte(`---
name: always-on
description: Always included.
always: true
---

Always token.
`), 0o600); err != nil {
		t.Fatal(err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 91,
			"title": "@gitclaw /skills",
			"body": "Show the skill inventory and repo-reader selection.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{91: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic skills command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Skills Report", "Generated without a model call", "available_skills: `2`", "selected_skills: `2`", "skills_with_frontmatter: `2`", "skills_with_description: `2`", "skills_missing_requirements: `0`", "skill_validation_status: `ok`", "skill_validation_errors: `0`", "skill_validation_warnings: `0`", "skill_duplicate_names: `0`", "skill_invalid_names: `0`", "skill_name_folder_mismatches: `0`", "### Validation", "- none", "repo-reader", "always-on", ".gitclaw/SKILLS/repo-reader/SKILL.md", "frontmatter=`true`", "description=`true`", "sha256_12=", "requires_env=`0`", "missing_bins=`0`", "model=\"gitclaw/skills\""} {
		if !strings.Contains(body, want) {
			t.Fatalf("skills report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Skill token") || strings.Contains(body, "Always token") {
		t.Fatalf("skills report should not dump full skill bodies:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[91], "gitclaw:done") || hasLabel(github.IssueLabels[91], "gitclaw:running") || hasLabel(github.IssueLabels[91], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[91])
	}
}

func TestHandleSkillsInfoCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

SKILL_INFO_HANDLER_SECRET
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 113,
			"title": "@gitclaw /skills info repo-reader",
			"body": "Hidden skill info body token: SKILL_INFO_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{113: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic skills info command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Skill Info Report", "Generated without a model call", "model=\"gitclaw/skills\"", "requested_skill: `repo-reader`", "skill_info_status: `ok`", "matched_skills: `1`", "skill_name=`repo-reader`", "selected_for_this_turn=`true`", ".gitclaw/SKILLS/repo-reader/SKILL.md", "### Validation For Matches", "- none"} {
		if !strings.Contains(body, want) {
			t.Fatalf("skills info report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SKILL_INFO_HANDLER_SECRET") || strings.Contains(body, "SKILL_INFO_HANDLER_BODY_SECRET") {
		t.Fatalf("skills info report leaked body token:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[113], "gitclaw:done") || hasLabel(github.IssueLabels[113], "gitclaw:running") || hasLabel(github.IssueLabels[113], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[113])
	}
}

func TestHandleSoulCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gitclaw", "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".gitclaw/SOUL.md":              "SOUL_SECRET_TOKEN: stay repo native.",
		".gitclaw/IDENTITY.md":          "Identity: GitClaw.",
		".gitclaw/USER.md":              "USER_PRIVATE_TOKEN: maintainer facts.",
		".gitclaw/TOOLS.md":             "TOOLS_PRIVATE_TOKEN: read-only tools.",
		".gitclaw/MEMORY.md":            "Memory: durable facts.",
		".gitclaw/HEARTBEAT.md":         "Heartbeat: scheduled workflow notes.",
		".gitclaw/memory/2026-05-29.md": "Daily note token.",
	}
	for path, body := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 92,
			"title": "@gitclaw /soul",
			"body": "Show loaded soul and memory files.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{92: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic soul command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Soul Report", "Generated without a model call", "model=\"gitclaw/soul\"", "soul_validation_status: `ok`", "soul_validation_errors: `0`", "soul_validation_warnings: `0`", "soul_required_files_present: `6`", "soul_required_files_missing: `0`", "soul_memory_notes: `1`", "soul_noncanonical_memory_notes: `0`", "### Validation", "- none", ".gitclaw/SOUL.md", ".gitclaw/IDENTITY.md", ".gitclaw/USER.md", ".gitclaw/TOOLS.md", ".gitclaw/MEMORY.md", ".gitclaw/HEARTBEAT.md", ".gitclaw/memory/2026-05-29.md", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("soul report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SOUL_SECRET_TOKEN") || strings.Contains(body, "USER_PRIVATE_TOKEN") || strings.Contains(body, "TOOLS_PRIVATE_TOKEN") {
		t.Fatalf("soul report should not dump full context bodies:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[92], "gitclaw:done") || hasLabel(github.IssueLabels[92], "gitclaw:running") || hasLabel(github.IssueLabels[92], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[92])
	}
}

func TestHandleSoulSearchCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Repo-native operating guidance SOUL_SEARCH_HANDLER_SECRET.")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "GitClaw identity details.")
	writeTestFile(t, root, ".gitclaw/USER.md", "User operating preference USER_SEARCH_HANDLER_SECRET.")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 116,
			"title": "GitClaw soul search handler test",
			"body": "@gitclaw /soul search operating SOUL_SEARCH_HANDLER_QUERY_SECRET\n\nHidden soul search body token: SOUL_SEARCH_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{116: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic soul search command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Soul Search Report", "Generated without a model call", "model=\"gitclaw/soul\"", "soul_search_status: `ok`", "query_sha256_12:", "max_results: `10`", "files_scanned: `3`", "matched_files: `2`", "matched_lines: `2`", "results_returned: `2`", "raw_bodies_included: `false`", "path=`.gitclaw/SOUL.md`", "category=`soul`", "path=`.gitclaw/USER.md`", "category=`user`", "line_sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("soul search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SOUL_SEARCH_HANDLER_SECRET", "USER_SEARCH_HANDLER_SECRET", "SOUL_SEARCH_HANDLER_QUERY_SECRET", "SOUL_SEARCH_HANDLER_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("soul search report leaked body/query token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[116], "gitclaw:done") || hasLabel(github.IssueLabels[116], "gitclaw:running") || hasLabel(github.IssueLabels[116], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[116])
	}
}

func TestHandleMemoryCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "LONG_TERM_MEMORY_SECRET: durable facts.")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-26.md", "OLD_MEMORY_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-27.md", "THIRD_MEMORY_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-28.md", "SECOND_MEMORY_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "LATEST_MEMORY_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/scratch.md", "NONCANONICAL_MEMORY_SECRET")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 103,
			"title": "@gitclaw /memory",
			"body": "Show memory inventory. Hidden memory body token: MEMORY_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{103: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic memory command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Memory Report", "Generated without a model call", "model=\"gitclaw/memory\"", "memory_mode: `read-only`", "long_term_memory_present: `true`", "long_term_memory_loaded: `true`", "dated_memory_notes: `5`", "canonical_dated_memory_notes: `4`", "noncanonical_dated_memory_notes: `1`", "loaded_memory_notes: `3`", "max_loaded_memory_notes: `3`", "omitted_memory_notes: `2`", "latest_memory_note: `.gitclaw/memory/2026-05-29.md`", "memory_validation_status: `warn`", "memory_validation_errors: `0`", "memory_validation_warnings: `1`", "code=`noncanonical_memory_note`", ".gitclaw/MEMORY.md", ".gitclaw/memory/scratch.md", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"LONG_TERM_MEMORY_SECRET", "LATEST_MEMORY_SECRET", "OLD_MEMORY_SECRET", "MEMORY_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[103], "gitclaw:done") || hasLabel(github.IssueLabels[103], "gitclaw:running") || hasLabel(github.IssueLabels[103], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[103])
	}
}

func TestHandleMemoryValidateCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_VALIDATE_HANDLER_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "DATED_MEMORY_VALIDATE_HANDLER_SECRET")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 114,
			"title": "@gitclaw /memory validate",
			"body": "Hidden memory validate body token: MEMORY_VALIDATE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{114: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic memory validate command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Memory Validate Report", "Generated without a model call", "model=\"gitclaw/memory\"", "memory_validation_status: `ok`", "memory_validation_errors: `0`", "memory_validation_warnings: `0`", "long_term_memory_present: `true`", "dated_memory_notes: `1`", "canonical_dated_memory_notes: `1`", "potential_secret_findings: `0`", "### Findings", "- none"} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory validate report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MEMORY_VALIDATE_HANDLER_SECRET", "DATED_MEMORY_VALIDATE_HANDLER_SECRET", "MEMORY_VALIDATE_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory validate report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[114], "gitclaw:done") || hasLabel(github.IssueLabels[114], "gitclaw:running") || hasLabel(github.IssueLabels[114], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[114])
	}
}

func TestHandleMemorySearchCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Deployment preference MEMORY_SEARCH_HANDLER_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Deployment note DATED_MEMORY_SEARCH_HANDLER_SECRET")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 115,
			"title": "@gitclaw /memory search deployment MEMORY_SEARCH_HANDLER_QUERY_SECRET",
			"body": "Hidden memory search body token: MEMORY_SEARCH_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{115: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic memory search command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Memory Search Report", "Generated without a model call", "model=\"gitclaw/memory\"", "memory_search_status: `ok`", "query_sha256_12:", "max_results: `10`", "files_scanned: `2`", "matched_files: `2`", "matched_lines: `2`", "results_returned: `2`", "raw_bodies_included: `false`", "path=`.gitclaw/MEMORY.md`", "path=`.gitclaw/memory/2026-05-29.md`", "line_sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MEMORY_SEARCH_HANDLER_SECRET", "DATED_MEMORY_SEARCH_HANDLER_SECRET", "MEMORY_SEARCH_HANDLER_QUERY_SECRET", "MEMORY_SEARCH_HANDLER_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory search report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[115], "gitclaw:done") || hasLabel(github.IssueLabels[115], "gitclaw:running") || hasLabel(github.IssueLabels[115], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[115])
	}
}

func TestHandlePromptCommandPostsReportWithoutLLM(t *testing.T) {
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", filepath.Join(t.TempDir(), "prompt.md"))
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "SOUL_PROMPT_SECRET: stay repo native.")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOL_PROMPT_SECRET: read-only tools.")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository files.
---

PROMPT_SKILL_SECRET
`)
	writeTestFile(t, root, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	writeTestFile(t, root, "docs/search-fixture.md", "bounded repository search fixture phrase => GITCLAW_PROMPT_SEARCH_SECRET\n")

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 104,
			"title": "@gitclaw /prompt",
			"body": "Inspect go.mod with the repo-reader skill, search for \"bounded repository search fixture phrase\", and hide PROMPT_BODY_SECRET. `+strings.Repeat("noise ", 80)+`",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{104: {
		{ID: 1, Body: "older prompt comment " + strings.Repeat("x", 120), AuthorAssociation: "MEMBER", User: User{Login: "alice", Type: "User"}},
		{ID: 2, Body: "middle prompt comment " + strings.Repeat("y", 120), AuthorAssociation: "MEMBER", User: User{Login: "alice", Type: "User"}},
		{ID: 3, Body: "latest prompt comment " + strings.Repeat("z", 120), AuthorAssociation: "MEMBER", User: User{Login: "alice", Type: "User"}},
	}}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	cfg.MaxTranscriptMessages = 2
	cfg.MaxTranscriptMessageBytes = 80
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic prompt command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Prompt Report", "Generated without a model call", "model=\"gitclaw/prompt\"", "provider: `github-models`", "model: `openai/gpt-5-mini`", "system_prompt_sha256_12:", "prompt_bytes:", "prompt_sha256_12:", "max_prompt_bytes: `60000`", "max_transcript_messages: `2`", "max_transcript_message_bytes: `80`", "transcript_messages: `4`", "bounded_transcript_messages: `2`", "omitted_older_messages: `2`", "truncated_transcript_bodies: `2`", "prompt_contains_truncation_marker: `true`", "prompt_artifact_enabled: `true`", "prompt_artifact_redaction_patterns:", "prompt_body_included: `false`", "context_files:", "selected_skills: `1`", "available_skills: `1`", "tool_outputs:", ".gitclaw/SOUL.md", ".gitclaw/TOOLS.md", ".gitclaw/SKILLS/repo-reader/SKILL.md", "gitclaw.list_files", "gitclaw.skill_index", "gitclaw.search_files", "gitclaw.read_file", "input=`go.mod`", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("prompt report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROMPT_BODY_SECRET", "SOUL_PROMPT_SECRET", "TOOL_PROMPT_SECRET", "PROMPT_SKILL_SECRET", "GITCLAW_PROMPT_SEARCH_SECRET", "module github.com/AnandChowdhary/gitclaw", "latest prompt comment"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("prompt report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[104], "gitclaw:done") || hasLabel(github.IssueLabels[104], "gitclaw:running") || hasLabel(github.IssueLabels[104], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[104])
	}
}

func TestHandleToolsCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOL_SECRET_TOKEN: read-only tools only.")
	writeTestFile(t, root, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	writeTestFile(t, root, "docs/search-fixture.md", "bounded repository search fixture phrase => GITCLAW_SEARCH_CONTEXT_V1\n")

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 93,
			"title": "@gitclaw /tools",
			"body": "Inspect go.mod and search for \"bounded repository search fixture phrase\".",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{93: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tools command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Tools Report", "Generated without a model call", "model=\"gitclaw/tools\"", "available_tools: `5`", "active_tool_outputs: `3`", "tool_validation_status: `ok`", "tool_validation_errors: `0`", "tool_validation_warnings: `0`", "tool_contracts: `5`", "tool_active_outputs: `3`", "tool_guidance_files: `1`", "tool_unknown_outputs: `0`", "tool_unsafe_contracts: `0`", "tool_over_limit_outputs: `0`", "tool_missing_guidance: `0`", "tool_duplicate_contracts: `0`", "### Validation", "- none", ".gitclaw/TOOLS.md", "gitclaw.list_files", "gitclaw.search_files", "gitclaw.read_file", "input=`go.mod`", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_SECRET_TOKEN", "module github.com/AnandChowdhary/gitclaw", "GITCLAW_SEARCH_CONTEXT_V1"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tools report leaked output/body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[93], "gitclaw:done") || hasLabel(github.IssueLabels[93], "gitclaw:running") || hasLabel(github.IssueLabels[93], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[93])
	}
}

func TestHandleToolsSearchCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOLS_SEARCH_HANDLER_SECRET: read-only tools only.")
	writeTestFile(t, root, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 117,
			"title": "GitClaw tools search handler test",
			"body": "@gitclaw /tools search read_file TOOLS_SEARCH_HANDLER_QUERY_SECRET\n\nMention go.mod so read_file output exists. Hidden tools search body token: TOOLS_SEARCH_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{117: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tools search command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Tools Search Report", "Generated without a model call", "model=\"gitclaw/tools\"", "tool_search_status: `ok`", "query_sha256_12:", "max_results: `10`", "available_tools: `5`", "active_tool_outputs: `3`", "matched_contracts: `1`", "matched_outputs: `2`", "results_returned: `3`", "raw_bodies_included: `false`", "raw_inputs_included: `false`", "kind=`contract` name=`gitclaw.read_file`", "kind=`active-output` name=`gitclaw.read_file`", "kind=`active-output` name=`gitclaw.search_files`", "input_sha256_12=", "output_sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOLS_SEARCH_HANDLER_SECRET", "TOOLS_SEARCH_HANDLER_QUERY_SECRET", "TOOLS_SEARCH_HANDLER_BODY_SECRET", "module github.com/AnandChowdhary/gitclaw", "go.mod"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tools search report leaked body/input/query token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[117], "gitclaw:done") || hasLabel(github.IssueLabels[117], "gitclaw:running") || hasLabel(github.IssueLabels[117], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[117])
	}
}

func TestHandlePolicyCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Use read-only tools.")

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 94,
			"title": "@gitclaw /policy",
			"body": "Please implement this change and open a PR. Hidden token: GITCLAW_POLICY_SECRET_E2E.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{94: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic policy command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Policy Report", "Generated without a model call", "model=\"gitclaw/policy\"", "preflight_allowed: `true`", "actor_association: `MEMBER`", "actor_trusted: `true`", "write_request_detected: `true`", "run_mode: `read-only`", "OWNER", "MEMBER", "COLLABORATOR", "gitclaw:write-requested", "gitclaw.policy", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("policy report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "GITCLAW_POLICY_SECRET_E2E") || strings.Contains(body, "Please implement this change") {
		t.Fatalf("policy report should not dump issue body:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[94], "gitclaw:write-requested") || !hasLabel(github.IssueLabels[94], "gitclaw:done") || hasLabel(github.IssueLabels[94], "gitclaw:running") || hasLabel(github.IssueLabels[94], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[94])
	}
}

func TestHandleCommandsCommandPostsReportWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 108,
			"title": "@gitclaw /help",
			"body": "Hidden command body token: COMMAND_HANDLER_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = t.TempDir()
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{108: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic commands command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Commands Report", "Generated without a model call", "model=\"gitclaw/commands\"", "commands: `15`", "aliases: `7`", "local_cli_helpers: `19`", "/commands", "/backup", "/tools", "`gitclaw commands` command=`/help`", "`gitclaw backup retention-plan` command=`/backup`", "`gitclaw memory validate` command=`/memory`", "`gitclaw memory search <query>` command=`/memory`", "`gitclaw soul search <query>` command=`/soul`", "`gitclaw skills info <name>` command=`/skills`", "`gitclaw skills search <query>` command=`/skills`", "`gitclaw tools validate` command=`/tools`", "`gitclaw tools search <query>` command=`/tools`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("commands report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "COMMAND_HANDLER_SECRET") {
		t.Fatalf("commands report leaked issue body token:\n%s", body)
	}
	if !hasLabel(github.IssueLabels[108], "gitclaw:done") || hasLabel(github.IssueLabels[108], "gitclaw:running") || hasLabel(github.IssueLabels[108], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[108])
	}
}

func TestHandleDoctorCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `model:
  model: openai/gpt-5-mini
`)
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", "name: GitClaw\n")
	writeTestFile(t, root, ".github/workflows/gitclaw-heartbeat.yml", "name: GitClaw Heartbeat\n")
	writeTestFile(t, root, ".github/workflows/gitclaw-proactive.yml", "name: GitClaw Proactive\non:\n  workflow_dispatch:\n  schedule:\n")
	writeTestFile(t, root, ".github/workflows/gitclaw-channel-ingest.yml", "name: GitClaw Channel Ingest\n")
	writeTestFile(t, root, ".gitclaw/SOUL.md", "SOUL_DOCTOR_SECRET")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "IDENTITY_DOCTOR_SECRET")
	writeTestFile(t, root, ".gitclaw/USER.md", "USER_DOCTOR_SECRET")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOLS_DOCTOR_SECRET")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_DOCTOR_SECRET")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "HEARTBEAT_DOCTOR_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "MEMORY_NOTE_DOCTOR_SECRET")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SKILL_DOCTOR_SECRET
`)
	writeTestFile(t, root, ".gitclaw/proactive/repo-hygiene.md", "PROACTIVE_DOCTOR_SECRET")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 102,
			"title": "@gitclaw /doctor",
			"body": "Hidden doctor body token: DOCTOR_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	cfg, err = LoadConfigFromWorkdir(cfg)
	if err != nil {
		t.Fatalf("LoadConfigFromWorkdir returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{102: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic doctor command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Doctor Report", "Generated without a model call", "model=\"gitclaw/doctor\"", "health_status: `ok`", "config_source: `defaults+repo`", "config_valid: `true`", "config_file_present: `true`", "workflows_present: `4`", "context_files_present: `6`", "memory_notes: `1`", "skill_files: `1`", "proactive_prompt_files: `1`", "validation_errors: `0`", "validation_warnings: `0`", "skill_validation_status: `ok`", "skill_validation_errors: `0`", "skill_validation_warnings: `0`", "soul_validation_status: `ok`", "soul_validation_errors: `0`", "soul_validation_warnings: `0`", "memory_validation_status: `ok`", "memory_validation_errors: `0`", "memory_validation_warnings: `0`", "tool_validation_status: `ok`", "tool_validation_errors: `0`", "tool_validation_warnings: `0`", "`config_validation`: `ok`", "`main_workflow`: `ok`", "`local_skills`: `ok`", "`skill_validation`: `ok`", "`soul_validation`: `ok`", "`memory_validation`: `ok`", "`tool_validation`: `ok`", ".gitclaw/SOUL.md", ".gitclaw/SKILLS/repo-reader/SKILL.md", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("doctor report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"DOCTOR_BODY_SECRET", "SOUL_DOCTOR_SECRET", "IDENTITY_DOCTOR_SECRET", "SKILL_DOCTOR_SECRET", "PROACTIVE_DOCTOR_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("doctor report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[102], "gitclaw:done") || hasLabel(github.IssueLabels[102], "gitclaw:running") || hasLabel(github.IssueLabels[102], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[102])
	}
}

func TestHandleSessionCommandPostsReportWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 95,
			"title": "@gitclaw explain",
			"body": "Initial body token: ISSUE_SECRET_SESSION_TOKEN.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 22,
			"body": "@gitclaw /session\nHidden comment token: COMMENT_SECRET_SESSION_TOKEN.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"created_at": "2026-05-29T12:00:00Z",
			"updated_at": "2026-05-29T12:00:00Z"
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{95: {
		{
			ID:                21,
			Body:              "<!-- gitclaw:assistant-turn idempotency_key=old -->\nAssistant body token: ASSISTANT_SECRET_SESSION_TOKEN.",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                22,
			Body:              "@gitclaw /session\nHidden comment token: COMMENT_SECRET_SESSION_TOKEN.",
			User:              User{Login: "alice", Type: "User"},
			AuthorAssociation: "MEMBER",
			CreatedAt:         "2026-05-29T12:00:00Z",
			UpdatedAt:         "2026-05-29T12:00:00Z",
		},
	}}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic session command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Session Report", "Generated without a model call", "model=\"gitclaw/session\"", "raw_comments: `2`", "transcript_messages: `3`", "user_messages: `2`", "assistant_messages: `1`", "assistant_turn_comments: `1`", "source=`comment:21`", "source=`comment:22`", "sha256_12="} {
		if !strings.Contains(body, want) {
			t.Fatalf("session report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"ISSUE_SECRET_SESSION_TOKEN", "COMMENT_SECRET_SESSION_TOKEN", "ASSISTANT_SECRET_SESSION_TOKEN"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[95], "gitclaw:done") || hasLabel(github.IssueLabels[95], "gitclaw:running") || hasLabel(github.IssueLabels[95], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[95])
	}
}

func TestHandleLabelsWriteRequestsAndAddsPolicyContext(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 99,
			"title": "@gitclaw implement this",
			"body": "Please implement a new CLI command and open a PR.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{99: nil}}
	llm := &FakeLLM{Response: "I cannot modify files, but here is a proposed plan."}
	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !hasLabel(github.IssueLabels[99], "gitclaw:write-requested") {
		t.Fatalf("write request label missing: %#v", github.IssueLabels[99])
	}
	if !hasToolOutput(llm.LastRequest.Context.ToolOutputs, "gitclaw.policy", "write-request", "Current GitClaw mode is read-only") {
		t.Fatalf("LLM request missing write policy output: %#v", llm.LastRequest.Context.ToolOutputs)
	}
}

func TestHandleSetsErrorStatusWhenLLMFails(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 77,
			"title": "@gitclaw fail",
			"body": "Please fail.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{77: nil}}
	llm := &FakeLLM{Err: errors.New("provider unavailable")}
	err = Handle(context.Background(), ev, DefaultConfig(), github, llm)
	if err == nil {
		t.Fatalf("Handle should return LLM error")
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want one safe error comment: %#v", len(github.Posted), github.Posted)
	}
	if !HasGitClawErrorMarker(github.Posted[0].Body) {
		t.Fatalf("error comment missing marker: %s", github.Posted[0].Body)
	}
	if !strings.Contains(github.Posted[0].Body, "model provider request failed") {
		t.Fatalf("error comment missing safe diagnostic: %s", github.Posted[0].Body)
	}
	if containsUnsafeDiagnosticContent(github.Posted[0].Body, "provider unavailable", "Please fail") {
		t.Fatalf("error comment leaked unsafe content: %s", github.Posted[0].Body)
	}
	labels := github.IssueLabels[77]
	if !hasLabel(labels, "gitclaw:error") || hasLabel(labels, "gitclaw:running") || hasLabel(labels, "gitclaw:done") {
		t.Fatalf("unexpected error labels: %#v", labels)
	}
}

type FakeGitHub struct {
	Issues          []Issue
	CommentsByIssue map[int][]Comment
	Posted          []PostedComment
	IssueLabels     map[int][]string
}

func (f *FakeGitHub) CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error) {
	number := 100 + len(f.Issues)
	issue := Issue{
		Number:            number,
		Title:             title,
		Body:              body,
		AuthorAssociation: "MEMBER",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		Labels:            append([]string(nil), labels...),
	}
	f.Issues = append(f.Issues, issue)
	f.ensureIssueLabels()
	f.IssueLabels[number] = append([]string(nil), labels...)
	return issue, nil
}

func (f *FakeGitHub) GetIssue(ctx context.Context, repo string, issueNumber int) (Issue, error) {
	for _, issue := range f.Issues {
		if issue.Number == issueNumber {
			return issue, nil
		}
	}
	return Issue{}, errors.New("issue not found")
}

func (f *FakeGitHub) ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error) {
	var issues []Issue
	for _, issue := range f.Issues {
		if !issueHasAllLabels(issue, labels) {
			continue
		}
		issues = append(issues, issue)
		if limit > 0 && len(issues) >= limit {
			break
		}
	}
	return issues, nil
}

func (f *FakeGitHub) ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error) {
	return append([]Comment(nil), f.CommentsByIssue[issueNumber]...), nil
}

func (f *FakeGitHub) PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error) {
	if f.CommentsByIssue == nil {
		f.CommentsByIssue = map[int][]Comment{}
	}
	posted := PostedComment{ID: int64(9000 + len(f.Posted)), Body: body}
	f.Posted = append(f.Posted, posted)
	f.CommentsByIssue[issueNumber] = append(f.CommentsByIssue[issueNumber], Comment{
		ID:   posted.ID,
		Body: body,
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	})
	return posted, nil
}

func (f *FakeGitHub) AddIssueLabels(ctx context.Context, repo string, issueNumber int, labels []string) error {
	f.ensureIssueLabels()
	for _, label := range labels {
		if label == "" || hasLabel(f.IssueLabels[issueNumber], label) {
			continue
		}
		f.IssueLabels[issueNumber] = append(f.IssueLabels[issueNumber], label)
	}
	return nil
}

func (f *FakeGitHub) RemoveIssueLabel(ctx context.Context, repo string, issueNumber int, label string) error {
	f.ensureIssueLabels()
	var kept []string
	for _, existing := range f.IssueLabels[issueNumber] {
		if !strings.EqualFold(existing, label) {
			kept = append(kept, existing)
		}
	}
	f.IssueLabels[issueNumber] = kept
	return nil
}

func (f *FakeGitHub) ensureIssueLabels() {
	if f.IssueLabels == nil {
		f.IssueLabels = map[int][]string{}
	}
}

type FakeLLM struct {
	Response    string
	Err         error
	Calls       int
	LastRequest LLMRequest
}

func (f *FakeLLM) Complete(ctx context.Context, req LLMRequest) (string, error) {
	f.Calls++
	f.LastRequest = req
	if f.Err != nil {
		return "", f.Err
	}
	return f.Response, nil
}

func issueHasAllLabels(issue Issue, labels []string) bool {
	for _, label := range labels {
		if !hasLabel(issue.Labels, label) {
			return false
		}
	}
	return true
}

func containsUnsafeDiagnosticContent(body string, values ...string) bool {
	body = strings.ToLower(body)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && strings.Contains(body, value) {
			return true
		}
	}
	return false
}
