package gitclaw

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestBackupIssueWritesCanonicalIssueTranscript(t *testing.T) {
	ev := Event{
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number:            12,
			Title:             "@gitclaw backup this",
			Body:              "Initial request",
			AuthorAssociation: "MEMBER",
			User:              User{Login: "alice", Type: "User"},
			Labels:            []string{"gitclaw"},
		},
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{
		12: {
			{
				ID:                1,
				Body:              "<!-- gitclaw:assistant-turn idempotency_key=old -->\nFirst reply",
				User:              User{Login: "github-actions[bot]", Type: "Bot"},
				AuthorAssociation: "MEMBER",
			},
			{
				ID:                2,
				Body:              "Follow up",
				User:              User{Login: "alice", Type: "User"},
				AuthorAssociation: "MEMBER",
			},
		},
	}}

	path, err := BackupIssue(context.Background(), ev, github, t.TempDir(), time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BackupIssue returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		t.Fatalf("backup is not JSON: %v\n%s", err, data)
	}
	if backup.Version != 1 || backup.Repo != "owner/repo" || backup.Issue.Number != 12 {
		t.Fatalf("unexpected backup metadata: %#v", backup)
	}
	if len(backup.Comments) != 2 {
		t.Fatalf("comments = %d, want 2", len(backup.Comments))
	}
	if len(backup.Transcript) != 3 {
		t.Fatalf("transcript = %d, want 3: %#v", len(backup.Transcript), backup.Transcript)
	}
	if backup.Transcript[1].Role != "assistant" || backup.Transcript[1].Body != "First reply" {
		t.Fatalf("assistant transcript not reconstructed: %#v", backup.Transcript)
	}
}

func TestIssueBackupPathIsRepoScoped(t *testing.T) {
	got := issueBackupPath(".gitclaw/backups", "owner/repo", 7)
	want := ".gitclaw/backups/owner__repo/issues/000007.json"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
