package gitclaw

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestWriteBackupIndexSummarizesIssueBackups(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw first | title",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "hi"}},
		Comments:   []IssueBackupComment{{ID: 1, Body: "comment"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 12,
			Title:  "@gitclaw second",
		},
		Transcript: []TranscriptMessage{{Role: "user"}, {Role: "assistant"}},
		Comments:   []IssueBackupComment{{ID: 1}, {ID: 2}},
	})

	indexPath, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	var index BackupIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("index is not JSON: %v\n%s", err, data)
	}
	if index.Version != 1 || index.Repo != "owner/repo" || index.Count != 2 {
		t.Fatalf("unexpected index metadata: %#v", index)
	}
	if index.Issues[0].Number != 7 || index.Issues[0].Path != "issues/000007.json" || index.Issues[0].CommentCount != 1 {
		t.Fatalf("unexpected first index issue: %#v", index.Issues[0])
	}
	if index.Issues[1].Number != 12 || index.Issues[1].TranscriptMessages != 2 {
		t.Fatalf("unexpected second index issue: %#v", index.Issues[1])
	}
	readme, err := os.ReadFile(filepath.Join(root, "owner__repo", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readmeBody := string(readme)
	for _, want := range []string{"# GitClaw Backups", "#7", "@gitclaw first \\| title", "issues/000012.json"} {
		if !strings.Contains(readmeBody, want) {
			t.Fatalf("README missing %q:\n%s", want, readmeBody)
		}
	}
}

func TestExportBackupJSONLEmitsTranscriptRecords(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw export",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "initial export token", Actor: "alice", AuthorAssociation: "OWNER", Trusted: true},
			{Role: "assistant", Body: "assistant export reply", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{{ID: 12, Body: "assistant raw comment"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 8, Title: "@gitclaw other"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "other issue token"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	output, err := ExportBackupJSONL(root, "owner/repo", 7)
	if err != nil {
		t.Fatalf("ExportBackupJSONL returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("jsonl lines = %d, want 2:\n%s", len(lines), output)
	}
	var first BackupJSONLRecord
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line is not JSON: %v\n%s", err, lines[0])
	}
	if first.Schema != "gitclaw.backup.transcript.v1" || first.Repo != "owner/repo" || first.IssueNumber != 7 || first.Sequence != 1 || first.Source != "issue" || first.Body != "initial export token" {
		t.Fatalf("unexpected first record: %#v", first)
	}
	if first.BodySHA != shortDocumentHash("initial export token") {
		t.Fatalf("first body hash = %q", first.BodySHA)
	}
	var second BackupJSONLRecord
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("second line is not JSON: %v\n%s", err, lines[1])
	}
	if second.Source != "comment:12" || second.CommentID != 12 || second.Body != "assistant export reply" {
		t.Fatalf("unexpected second record: %#v", second)
	}
	if strings.Contains(output, "other issue token") {
		t.Fatalf("filtered export included another issue:\n%s", output)
	}
}

func TestVerifyBackupTreeAcceptsCanonicalBackupIndex(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw verify",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user"}, {Role: "assistant"}},
		Comments:   []IssueBackupComment{{ID: 1}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	result, err := VerifyBackupTree(root, "owner/repo")
	if err != nil {
		t.Fatalf("VerifyBackupTree returned error: %v", err)
	}
	if !result.OK() {
		t.Fatalf("verification failed: %#v", result.VerificationFailures)
	}
	if result.IssuesChecked != 1 || result.CommentsChecked != 1 || result.TranscriptMessages != 2 {
		t.Fatalf("unexpected verification counts: %#v", result)
	}
	report := RenderBackupVerifyReport(result)
	for _, want := range []string{"GitClaw Backup Verify Report", "backup_verify_status: `ok`", "issues_checked: `1`", "comments_checked: `1`", "verification_failures: `0`", "canonical `issues/000000.json`"} {
		if !strings.Contains(report, want) {
			t.Fatalf("verify report missing %q:\n%s", want, report)
		}
	}
}

func TestVerifyBackupTreeRejectsTraversalPath(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw verify",
		},
		Transcript: []TranscriptMessage{{Role: "user"}},
	})
	indexPath, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	var index BackupIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	index.Issues[0].Path = "../escape.json"
	data, err = json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := VerifyBackupTree(root, "owner/repo")
	if err != nil {
		t.Fatalf("VerifyBackupTree returned error: %v", err)
	}
	if result.OK() {
		t.Fatalf("verification unexpectedly passed: %#v", result)
	}
	report := RenderBackupVerifyReport(result)
	for _, want := range []string{"backup_verify_status: `warn`", "issue_path_safe", "issue_path_canonical", "issue_indexed"} {
		if !strings.Contains(report, want) {
			t.Fatalf("verify report missing %q:\n%s", want, report)
		}
	}
}

func writeBackupFixture(t *testing.T, root string, backup IssueBackup) {
	t.Helper()
	path := issueBackupPath(root, backup.Repo, backup.Issue.Number)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
