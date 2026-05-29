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
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", "# Repo Reader\nUse read-only files.")
	writeTestFile(t, root, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	writeTestFile(t, root, "README.md", "hello")

	ctx, err := LoadRepoContext(root, []TranscriptMessage{{
		Role: "user",
		Body: "Please inspect `go.mod` and tell me the module path.",
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
	if !hasToolOutput(ctx.ToolOutputs, "gitclaw.list_files", ".", "go.mod") {
		t.Fatalf("list_files tool output missing go.mod: %#v", ctx.ToolOutputs)
	}
	if !hasToolOutput(ctx.ToolOutputs, "gitclaw.read_file", "go.mod", "module github.com/AnandChowdhary/gitclaw") {
		t.Fatalf("read_file tool output missing go.mod contents: %#v", ctx.ToolOutputs)
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

func contextDocPaths(docs []ContextDocument) string {
	paths := make([]string, 0, len(docs))
	for _, doc := range docs {
		paths = append(paths, doc.Path)
	}
	return strings.Join(paths, "\n")
}
