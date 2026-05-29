package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
