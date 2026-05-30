package gitclaw

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRepoContextLoadsSoulSkillsAndMentionedFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Be concise and repo-native.")
	writeTestFile(t, root, ".gitclaw/USER.md", "The maintainer prefers GitHub-native state.")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Name: GitClaw")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "No autonomous scheduled writes.")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Durable memory token: GITCLAW_MEMORY_CONTEXT_V1.")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-28.md", "Yesterday: backed up issue #1.")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Today: verify memory context loading.")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

# Repo Reader
Use read-only files.`)
	writeTestFile(t, root, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	writeTestFile(t, root, "README.md", "hello")
	writeTestFile(t, root, "docs/search-fixture.md", "bounded repository search fixture phrase => GITCLAW_SEARCH_CONTEXT_V1\n")

	ctx, err := LoadRepoContext(root, []TranscriptMessage{{
		Role: "user",
		Body: "Please use the repo-reader skill, inspect `go.mod`, search for `bounded repository search fixture phrase`, and tell me the module path.",
	}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}

	if !hasContextDoc(ctx.Documents, ".gitclaw/SOUL.md", "repo-native") {
		t.Fatalf("SOUL.md was not loaded: %#v", ctx.Documents)
	}
	for _, want := range []struct {
		path string
		body string
	}{
		{".gitclaw/USER.md", "GitHub-native state"},
		{".gitclaw/IDENTITY.md", "GitClaw"},
		{".gitclaw/HEARTBEAT.md", "No autonomous"},
		{".gitclaw/MEMORY.md", "GITCLAW_MEMORY_CONTEXT_V1"},
		{".gitclaw/memory/2026-05-28.md", "issue #1"},
		{".gitclaw/memory/2026-05-29.md", "memory context"},
	} {
		if !hasContextDoc(ctx.Documents, want.path, want.body) {
			t.Fatalf("%s was not loaded with %q: %#v", want.path, want.body, ctx.Documents)
		}
	}
	if !hasContextDoc(ctx.Skills, ".gitclaw/SKILLS/repo-reader/SKILL.md", "Repo Reader") {
		t.Fatalf("skill was not loaded: %#v", ctx.Skills)
	}
	if !hasToolOutput(ctx.ToolOutputs, "gitclaw.skill_index", ".gitclaw/SKILLS", "repo-reader") {
		t.Fatalf("skill index missing repo-reader: %#v", ctx.ToolOutputs)
	}
	if !hasToolOutput(ctx.ToolOutputs, "gitclaw.skill_index", ".gitclaw/SKILLS", "sha256_12=") {
		t.Fatalf("skill index missing audit hashes: %#v", ctx.ToolOutputs)
	}
	if !hasToolOutput(ctx.ToolOutputs, "gitclaw.list_files", ".", "go.mod") {
		t.Fatalf("list_files tool output missing go.mod: %#v", ctx.ToolOutputs)
	}
	if !hasToolOutput(ctx.ToolOutputs, "gitclaw.read_file", "go.mod", "module github.com/AnandChowdhary/gitclaw") {
		t.Fatalf("read_file tool output missing go.mod contents: %#v", ctx.ToolOutputs)
	}
	if !hasToolOutputBody(ctx.ToolOutputs, "gitclaw.search_files", "GITCLAW_SEARCH_CONTEXT_V1") {
		t.Fatalf("search_files tool output missing search token: %#v", ctx.ToolOutputs)
	}
}

func TestLoadRepoContextHonorsConfiguredToolGates(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Be concise.")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

# Repo Reader
Use read-only files.`)
	writeTestFile(t, root, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	writeTestFile(t, root, "docs/search-fixture.md", "bounded repository search fixture phrase => GITCLAW_SEARCH_CONTEXT_V1\n")

	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{
		Role: "user",
		Body: "Use repo-reader, inspect `go.mod`, and search for `bounded repository search fixture phrase`.",
	}}, Config{
		AllowedTools:  map[string]bool{"gitclaw.list_files": true, "gitclaw.skill_index": true},
		DisabledTools: map[string]bool{"gitclaw.skill_index": true},
	})
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	if !hasToolOutput(ctx.ToolOutputs, "gitclaw.list_files", ".", "go.mod") {
		t.Fatalf("list_files should remain active: %#v", ctx.ToolOutputs)
	}
	for _, blocked := range []string{"gitclaw.skill_index", "gitclaw.search_files", "gitclaw.read_file"} {
		if hasToolOutputBody(ctx.ToolOutputs, blocked, "") {
			t.Fatalf("%s should not be active: %#v", blocked, ctx.ToolOutputs)
		}
	}
	if enabledToolCount(ctx) != 1 || disabledToolCount(ctx) != 1 || allowlistBlockedToolCount(ctx) != 3 {
		t.Fatalf("unexpected tool gate counts: %#v", ctx)
	}
}

func TestLoadSkillContextSelectsRequestedAndAlwaysSkills(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository files and explain Go modules.
---

# Repo Reader
Skill token: GITCLAW_SKILL_CONTEXT_V1.`)
	writeTestFile(t, root, ".gitclaw/SKILLS/always-on/SKILL.md", `---
name: always-on
description: Always loaded baseline behavior.
metadata:
  openclaw:
    always: true
    requires:
      env:
        - GITCLAW_SKILL_TEST_ENV_MISSING
      bins: [gitclaw-no-such-binary-for-test]
---

# Always On
Always included.`)
	writeTestFile(t, root, ".gitclaw/SKILLS/unrelated/SKILL.md", `---
name: unrelated
description: Handle unrelated calendar work.
---

# Unrelated
Should not be selected.`)

	summaries, skills := loadSkillContext(root, []TranscriptMessage{{Role: "user", Body: "Use the repo-reader skill for go.mod."}}, Config{})
	if len(summaries) != 3 {
		t.Fatalf("len(summaries) = %d, want 3: %#v", len(summaries), summaries)
	}
	if !hasContextDoc(skills, ".gitclaw/SKILLS/repo-reader/SKILL.md", "GITCLAW_SKILL_CONTEXT_V1") {
		t.Fatalf("requested repo-reader skill was not selected: %#v", skills)
	}
	if !hasContextDoc(skills, ".gitclaw/SKILLS/always-on/SKILL.md", "Always included") {
		t.Fatalf("always-on skill was not selected: %#v", skills)
	}
	if hasContextDoc(skills, ".gitclaw/SKILLS/unrelated/SKILL.md", "Should not be selected") {
		t.Fatalf("unrelated skill should not be selected: %#v", skills)
	}
	index := renderSkillIndex(summaries)
	for _, want := range []string{"repo-reader", "always-on", "unrelated", "enabled=true", "disabled_by_config=false", "blocked_by_allowlist=false", "frontmatter=true", "description=true", "sha256_12=", "requires_env=1", "requires_bins=1", "missing_env=1", "missing_bins=1"} {
		if !strings.Contains(index, want) {
			t.Fatalf("skill index missing %q: %s", want, index)
		}
	}
}

func TestLoadSkillContextHonorsConfiguredSkillGates(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository files.
---

# Repo Reader
Skill token: GITCLAW_REPO_READER_ENABLED.`)
	writeTestFile(t, root, ".gitclaw/SKILLS/always-on/SKILL.md", `---
name: always-on
description: Always loaded baseline behavior.
always: true
---

# Always On
Skill token: GITCLAW_ALWAYS_ON_DISABLED.`)
	writeTestFile(t, root, ".gitclaw/SKILLS/blocked/SKILL.md", `---
name: blocked
description: Blocked by allowlist.
always: true
---

# Blocked
Skill token: GITCLAW_BLOCKED_BY_ALLOWLIST.`)

	cfg := Config{
		AllowedSkills:  map[string]bool{"repo-reader": true, "always-on": true},
		DisabledSkills: map[string]bool{"always-on": true},
	}
	summaries, skills := loadSkillContext(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader and blocked."}}, cfg)
	if len(summaries) != 3 {
		t.Fatalf("len(summaries) = %d, want 3: %#v", len(summaries), summaries)
	}
	if !hasContextDoc(skills, ".gitclaw/SKILLS/repo-reader/SKILL.md", "GITCLAW_REPO_READER_ENABLED") {
		t.Fatalf("repo-reader should be selected: %#v", skills)
	}
	if hasContextDoc(skills, ".gitclaw/SKILLS/always-on/SKILL.md", "GITCLAW_ALWAYS_ON_DISABLED") {
		t.Fatalf("disabled always-on skill should not be selected: %#v", skills)
	}
	if hasContextDoc(skills, ".gitclaw/SKILLS/blocked/SKILL.md", "GITCLAW_BLOCKED_BY_ALLOWLIST") {
		t.Fatalf("allowlist-blocked skill should not be selected: %#v", skills)
	}
	if enabledSkillCount(summaries) != 1 || disabledByConfigCount(summaries) != 1 || blockedByAllowlistCount(summaries) != 1 {
		t.Fatalf("unexpected skill gate counts: %#v", summaries)
	}
	index := renderSkillIndex(summaries)
	for _, want := range []string{"name=repo-reader", "enabled=true", "name=always-on", "disabled_by_config=true", "name=blocked", "blocked_by_allowlist=true"} {
		if !strings.Contains(index, want) {
			t.Fatalf("skill index missing %q: %s", want, index)
		}
	}
}

func TestParseSkillDocumentFrontmatter(t *testing.T) {
	skill := parseSkillDocument(".gitclaw/SKILLS/example/SKILL.md", `---
name: frontmatter-name
description: Frontmatter description.
metadata:
  openclaw:
    always: true
    requires:
      env:
        - GITCLAW_SKILL_TEST_ENV
      bins: [gitclaw-no-such-binary-for-test]
---

# Example`)
	if skill.Name != "frontmatter-name" {
		t.Fatalf("name = %q", skill.Name)
	}
	if skill.Description != "Frontmatter description." {
		t.Fatalf("description = %q", skill.Description)
	}
	if !skill.Always {
		t.Fatalf("always metadata was not parsed")
	}
	if !skill.FrontmatterPresent {
		t.Fatalf("frontmatter presence was not parsed")
	}
	if len(skill.RequiredEnv) != 1 || skill.RequiredEnv[0] != "GITCLAW_SKILL_TEST_ENV" {
		t.Fatalf("required env not parsed: %#v", skill.RequiredEnv)
	}
	if len(skill.RequiredBins) != 1 || skill.RequiredBins[0] != "gitclaw-no-such-binary-for-test" {
		t.Fatalf("required bins not parsed: %#v", skill.RequiredBins)
	}
}

func TestSearchQueriesPreferExplicitPhrasesAndIdentifiers(t *testing.T) {
	queries := searchQueriesFromTranscript([]TranscriptMessage{{
		Role: "user",
		Body: "Please search for `bounded repository search fixture phrase` and find BackupIssue without treating Please as code.",
	}})
	joined := strings.Join(queries, "\n")
	if !strings.Contains(joined, "bounded repository search fixture phrase") {
		t.Fatalf("queries missing explicit phrase: %#v", queries)
	}
	if !strings.Contains(joined, "BackupIssue") {
		t.Fatalf("queries missing code identifier: %#v", queries)
	}
	if strings.Contains(joined, "Please") {
		t.Fatalf("queries should not include title-case prose: %#v", queries)
	}
}

func TestLoadMemoryDocumentsKeepsLatestBoundedNotes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/memory/2026-05-26.md", "old")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-27.md", "third")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-28.md", "second")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "first")

	docs := loadMemoryDocuments(root)
	if len(docs) != 3 {
		t.Fatalf("len(docs) = %d, want 3: %#v", len(docs), docs)
	}
	if hasContextDoc(docs, ".gitclaw/memory/2026-05-26.md", "old") {
		t.Fatalf("oldest memory note should not be loaded: %#v", docs)
	}
	for _, want := range []string{"2026-05-27", "2026-05-28", "2026-05-29"} {
		if !strings.Contains(contextDocPaths(docs), want) {
			t.Fatalf("missing latest memory note %s: %#v", want, docs)
		}
	}
}

func TestSafeRepoPathRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	if _, err := safeRepoPath(root, "../secret"); err == nil {
		t.Fatalf("safeRepoPath allowed path traversal")
	}
	if _, err := safeRepoPath(root, "/tmp/secret"); err == nil {
		t.Fatalf("safeRepoPath allowed absolute path")
	}
}

func TestBuildPromptIncludesRepoContextAndToolOutputs(t *testing.T) {
	prompt := BuildPrompt(LLMRequest{
		Event: Event{
			Repo: "owner/repo",
			Issue: Issue{
				Number: 1,
				Title:  "@gitclaw inspect go.mod",
			},
		},
		Context: RepoContext{
			Documents: []ContextDocument{{Path: ".gitclaw/SOUL.md", Body: "Be concise."}},
			Skills:    []ContextDocument{{Path: ".gitclaw/SKILLS/repo-reader/SKILL.md", Body: "Use repo files."}},
			ToolOutputs: []ToolOutput{{
				Name:   "gitclaw.read_file",
				Input:  "go.mod",
				Output: "module github.com/AnandChowdhary/gitclaw",
			}},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "Read go.mod"}},
	})
	for _, want := range []string{".gitclaw/SOUL.md", "repo-reader", "gitclaw.read_file", "module github.com/AnandChowdhary/gitclaw"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func writeTestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hasContextDoc(docs []ContextDocument, path, bodyPart string) bool {
	for _, doc := range docs {
		if doc.Path == path && strings.Contains(doc.Body, bodyPart) {
			return true
		}
	}
	return false
}

func hasToolOutput(outputs []ToolOutput, name, input, bodyPart string) bool {
	for _, output := range outputs {
		if output.Name == name && output.Input == input && strings.Contains(output.Output, bodyPart) {
			return true
		}
	}
	return false
}

func hasToolOutputBody(outputs []ToolOutput, name, bodyPart string) bool {
	for _, output := range outputs {
		if output.Name == name && strings.Contains(output.Output, bodyPart) {
			return true
		}
	}
	return false
}

func contextDocPaths(docs []ContextDocument) string {
	paths := make([]string, 0, len(docs))
	for _, doc := range docs {
		paths = append(paths, doc.Path)
	}
	return strings.Join(paths, "\n")
}
