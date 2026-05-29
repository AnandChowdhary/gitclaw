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
