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

func TestPlanBackupRestoreRendersDryRunWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw restore",
			Body:   "RESTORE_ISSUE_BODY_SECRET",
			Labels: []string{"gitclaw", "gitclaw:e2e"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "RESTORE_TRANSCRIPT_SECRET", Actor: "alice", AuthorAssociation: "OWNER", Trusted: true},
			{Role: "assistant", Body: "RESTORE_ASSISTANT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nRESTORE_COMMENT_SECRET"},
			{ID: 13, Body: "<!-- gitclaw:error -->\nRESTORE_ERROR_SECRET"},
		},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	plan, err := PlanBackupRestore(root, "owner/repo", 7, "owner/restored")
	if err != nil {
		t.Fatalf("PlanBackupRestore returned error: %v", err)
	}
	if plan.TargetRepo != "owner/restored" || plan.Comments != 2 || plan.TranscriptMessages != 2 || plan.AssistantTurns != 1 || plan.ErrorComments != 1 {
		t.Fatalf("unexpected restore plan: %#v", plan)
	}
	report := RenderBackupRestorePlan(plan)
	for _, want := range []string{"GitClaw Backup Restore Plan", "restore_mode: `dry-run`", "source_repository: `owner/repo`", "target_repository: `owner/restored`", "issue: `#7`", "issue_backup_path: `issues/000007.json`", "backup_schema_version: `1`", "comments: `2`", "transcript_messages: `2`", "assistant_turn_comments: `1`", "error_comments: `1`", "raw_bodies_included: `false`", "comment_1_sha256_12:", "message_1_sha256_12:", "gitclaw:e2e"} {
		if !strings.Contains(report, want) {
			t.Fatalf("restore plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"RESTORE_ISSUE_BODY_SECRET", "RESTORE_TRANSCRIPT_SECRET", "RESTORE_ASSISTANT_SECRET", "RESTORE_COMMENT_SECRET", "RESTORE_ERROR_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("restore plan leaked body token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupManifestRendersHashesWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw manifest",
			Body:   "MANIFEST_ISSUE_BODY_SECRET",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "MANIFEST_TRANSCRIPT_SECRET", Actor: "alice", AuthorAssociation: "OWNER", Trusted: true},
			{Role: "assistant", Body: "MANIFEST_ASSISTANT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{{ID: 12, Body: "MANIFEST_COMMENT_SECRET"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 8, Title: "@gitclaw other", Body: "OTHER_MANIFEST_SECRET"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "other"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	manifest, err := BuildBackupManifest(root, "owner/repo", 7)
	if err != nil {
		t.Fatalf("BuildBackupManifest returned error: %v", err)
	}
	if manifest.Repo != "owner/repo" || manifest.SchemaVersion != 1 || len(manifest.ControlFiles) != 2 || len(manifest.IssuePayloads) != 1 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	payload := manifest.IssuePayloads[0]
	if payload.IssueNumber != 7 || payload.Path != "issues/000007.json" || payload.Comments != 1 || payload.TranscriptMessages != 2 || payload.SHA == "" || payload.Bytes == 0 {
		t.Fatalf("unexpected manifest payload: %#v", payload)
	}
	report := RenderBackupManifest(manifest)
	for _, want := range []string{"GitClaw Backup Manifest", "repository: `owner/repo`", "backup_schema_version: `1`", "issue_filter: `#7`", "control_files: `2`", "issue_payload_files: `1`", "total_comments: `1`", "total_transcript_messages: `2`", "raw_bodies_included: `false`", "`index.json` bytes=", "`README.md` bytes=", "issue=`#7` path=`issues/000007.json`", "sha256_12="} {
		if !strings.Contains(report, want) {
			t.Fatalf("manifest report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"MANIFEST_ISSUE_BODY_SECRET", "MANIFEST_TRANSCRIPT_SECRET", "MANIFEST_ASSISTANT_SECRET", "MANIFEST_COMMENT_SECRET", "OTHER_MANIFEST_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("manifest leaked body token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupStatsSummarizesBackupsWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw stats one",
			Body:   "STATS_ISSUE_BODY_SECRET",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "STATS_TRANSCRIPT_SECRET", Actor: "alice", Trusted: true},
			{Role: "assistant", Body: "STATS_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nSTATS_COMMENT_SECRET"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue:       IssueBackupIssue{Number: 8, Title: "@gitclaw stats two", Body: "OTHER_STATS_SECRET"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "second user"}},
		Comments:    []IssueBackupComment{{ID: 13, Body: "<!-- gitclaw:error -->\nerror body"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	stats, err := BuildBackupStats(root, "owner/repo")
	if err != nil {
		t.Fatalf("BuildBackupStats returned error: %v", err)
	}
	if stats.BackupStatsStatus != "ok" || stats.IssueCount != 2 || stats.CommentCount != 2 || stats.TranscriptMessages != 3 || stats.UserMessages != 2 || stats.AssistantMessages != 1 || stats.AssistantTurns != 1 || stats.ErrorComments != 1 || stats.LatestIssueNumber != 8 {
		t.Fatalf("unexpected backup stats: %#v", stats)
	}
	report := RenderBackupStats(stats)
	for _, want := range []string{"GitClaw Backup Stats Report", "repository: `owner/repo`", "backup_stats_status: `ok`", "backup_verify_status: `ok`", "verification_failures: `0`", "backup_schema_version: `1`", "issue_count: `2`", "comment_count: `2`", "transcript_messages: `3`", "user_messages: `2`", "assistant_messages: `1`", "assistant_turn_comments: `1`", "error_comments: `1`", "event_types: `2`", "latest_issue: `#8`", "latest_issue_path: `issues/000008.json`", "latest_issue_title_sha256_12:", "raw_bodies_included: `false`", "`issues`: `1`", "`issue_comment`: `1`"} {
		if !strings.Contains(report, want) {
			t.Fatalf("stats report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"STATS_ISSUE_BODY_SECRET", "STATS_TRANSCRIPT_SECRET", "STATS_ASSISTANT_SECRET", "STATS_COMMENT_SECRET", "OTHER_STATS_SECRET", "@gitclaw stats one", "@gitclaw stats two"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("stats report leaked body/title token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupListListsNewestBackupsWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw list oldest LIST_OLDEST_TITLE_TOKEN",
			Body:   "LIST_OLDEST_BODY_TOKEN",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "LIST_OLDEST_TRANSCRIPT_TOKEN"}},
		Comments:   []IssueBackupComment{{ID: 11, Body: "LIST_OLDEST_COMMENT_TOKEN"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw list newest LIST_NEWEST_TITLE_TOKEN",
			Body:   "LIST_NEWEST_BODY_TOKEN",
			Labels: []string{"gitclaw", "gitclaw:e2e"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "LIST_NEWEST_TRANSCRIPT_TOKEN"}, {Role: "assistant", Body: "LIST_NEWEST_ASSISTANT_TOKEN"}},
		Comments:   []IssueBackupComment{{ID: 12, Body: "LIST_NEWEST_COMMENT_TOKEN"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	list, err := BuildBackupList(root, "owner/repo", 1)
	if err != nil {
		t.Fatalf("BuildBackupList returned error: %v", err)
	}
	if list.BackupListStatus != "ok" || list.BackupVerifyStatus != "ok" || list.IssueCount != 2 || list.Limit != 1 || list.BackupsReturned != 1 {
		t.Fatalf("unexpected backup list metadata: %#v", list)
	}
	if list.Issues[0].IssueNumber != 8 || list.Issues[0].Labels != 2 || list.Issues[0].TranscriptMessages != 2 {
		t.Fatalf("unexpected backup list issue: %#v", list.Issues[0])
	}
	report := RenderBackupList(list)
	for _, want := range []string{"GitClaw Backup List Report", "backup_list_status: `ok`", "backup_verify_status: `ok`", "verification_failures: `0`", "backup_schema_version: `1`", "issue_count: `2`", "limit: `1`", "backups_returned: `1`", "raw_bodies_included: `false`", "### Indexed Backups", "issue=#8 path=`issues/000008.json`", "event=`issue_comment`", "labels=`2`", "comments=`1`", "transcript_messages=`2`", "title_sha256_12="} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup list report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"LIST_OLDEST_TITLE_TOKEN", "LIST_OLDEST_BODY_TOKEN", "LIST_OLDEST_TRANSCRIPT_TOKEN", "LIST_OLDEST_COMMENT_TOKEN", "LIST_NEWEST_TITLE_TOKEN", "LIST_NEWEST_BODY_TOKEN", "LIST_NEWEST_TRANSCRIPT_TOKEN", "LIST_NEWEST_ASSISTANT_TOKEN", "LIST_NEWEST_COMMENT_TOKEN", "@gitclaw list oldest", "@gitclaw list newest"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup list leaked body/title token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupTimelineRendersRecentChronologyWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw timeline old TIMELINE_OLD_TITLE_TOKEN",
			Body:   "TIMELINE_OLD_BODY_TOKEN",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "TIMELINE_OLD_TRANSCRIPT_TOKEN"}},
		Comments:   []IssueBackupComment{{ID: 11, Body: "TIMELINE_OLD_COMMENT_TOKEN"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T14:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw timeline middle TIMELINE_MIDDLE_TITLE_TOKEN",
			Body:   "TIMELINE_MIDDLE_BODY_TOKEN",
			Labels: []string{"gitclaw", "gitclaw:e2e"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "TIMELINE_MIDDLE_TRANSCRIPT_TOKEN"},
			{Role: "assistant", Body: "TIMELINE_MIDDLE_ASSISTANT_TOKEN"},
		},
		Comments: []IssueBackupComment{
			{ID: 12, Body: "<!-- gitclaw:error -->\nTIMELINE_MIDDLE_ERROR_TOKEN"},
		},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T15:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 9,
			Title:  "@gitclaw timeline latest TIMELINE_LATEST_TITLE_TOKEN",
			Body:   "TIMELINE_LATEST_BODY_TOKEN",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "TIMELINE_LATEST_TRANSCRIPT_TOKEN"},
			{Role: "assistant", Body: "TIMELINE_LATEST_ASSISTANT_TOKEN"},
		},
		Comments: []IssueBackupComment{
			{ID: 13, Body: "<!-- gitclaw:assistant-turn -->\nTIMELINE_LATEST_COMMENT_TOKEN"},
		},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 16, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	timeline, err := BuildBackupTimeline(root, "owner/repo", 2)
	if err != nil {
		t.Fatalf("BuildBackupTimeline returned error: %v", err)
	}
	if timeline.BackupTimelineStatus != "ok" || timeline.BackupVerifyStatus != "ok" || timeline.IssueCount != 3 || timeline.Limit != 2 || timeline.TimelinePoints != 2 {
		t.Fatalf("unexpected backup timeline metadata: %#v", timeline)
	}
	if timeline.FirstIssueNumber != 8 || timeline.LatestIssueNumber != 9 || timeline.TotalSpanSeconds != 3600 {
		t.Fatalf("unexpected timeline endpoints: %#v", timeline)
	}
	if timeline.Points[0].IssueNumber != 8 || timeline.Points[0].GapSecondsSincePrevious != 0 || timeline.Points[0].ErrorComments != 1 {
		t.Fatalf("unexpected first timeline point: %#v", timeline.Points[0])
	}
	if timeline.Points[1].IssueNumber != 9 || timeline.Points[1].GapSecondsSincePrevious != 3600 || timeline.Points[1].AssistantTurns != 1 {
		t.Fatalf("unexpected latest timeline point: %#v", timeline.Points[1])
	}
	report := RenderBackupTimeline(timeline)
	for _, want := range []string{"GitClaw Backup Timeline Report", "backup_timeline_status: `ok`", "backup_verify_status: `ok`", "verification_failures: `0`", "backup_schema_version: `1`", "issue_count: `3`", "limit: `2`", "timeline_points: `2`", "timeline_order: `chronological`", "timeline_window: `latest_by_backup_generated_at`", "first_issue: `#8`", "latest_issue: `#9`", "total_span_seconds: `3600`", "raw_bodies_included: `false`", "llm_e2e_required_after_backup_timeline_change: `true`", "### Timeline Points", "issue=#8 path=`issues/000008.json`", "gap_seconds_since_previous=`0`", "error_comments=`1`", "issue=#9 path=`issues/000009.json`", "gap_seconds_since_previous=`3600`", "assistant_turn_comments=`1`", "payload_sha256_12=", "title_sha256_12="} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup timeline report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"TIMELINE_OLD_TITLE_TOKEN", "TIMELINE_OLD_BODY_TOKEN", "TIMELINE_OLD_TRANSCRIPT_TOKEN", "TIMELINE_OLD_COMMENT_TOKEN", "TIMELINE_MIDDLE_TITLE_TOKEN", "TIMELINE_MIDDLE_BODY_TOKEN", "TIMELINE_MIDDLE_TRANSCRIPT_TOKEN", "TIMELINE_MIDDLE_ASSISTANT_TOKEN", "TIMELINE_MIDDLE_ERROR_TOKEN", "TIMELINE_LATEST_TITLE_TOKEN", "TIMELINE_LATEST_BODY_TOKEN", "TIMELINE_LATEST_TRANSCRIPT_TOKEN", "TIMELINE_LATEST_ASSISTANT_TOKEN", "TIMELINE_LATEST_COMMENT_TOKEN", "@gitclaw timeline old", "@gitclaw timeline middle", "@gitclaw timeline latest"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup timeline leaked body/title token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupInfoReportsOneBackupWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw info title BACKUP_INFO_TITLE_TOKEN",
			Body:   "BACKUP_INFO_BODY_TOKEN",
			Labels: []string{"gitclaw:e2e", "gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "BACKUP_INFO_TRANSCRIPT_TOKEN", Actor: "alice", Trusted: true},
			{Role: "assistant", Body: "BACKUP_INFO_ASSISTANT_TOKEN", Actor: "github-actions[bot]", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nBACKUP_INFO_COMMENT_TOKEN"},
			{ID: 13, Body: "<!-- gitclaw:error -->\nBACKUP_INFO_ERROR_TOKEN"},
		},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	info, err := BuildBackupInfo(root, "owner/repo", 8)
	if err != nil {
		t.Fatalf("BuildBackupInfo returned error: %v", err)
	}
	if info.BackupInfoStatus != "ok" || info.BackupVerifyStatus != "ok" || info.IssueNumber != 8 || info.Comments != 2 || info.TranscriptMessages != 2 || info.UserMessages != 1 || info.AssistantMessages != 1 || info.AssistantTurns != 1 || info.ErrorComments != 1 {
		t.Fatalf("unexpected backup info metadata: %#v", info)
	}
	report := RenderBackupInfo(info)
	for _, want := range []string{"GitClaw Backup Info Report", "repository: `owner/repo`", "backup_info_status: `ok`", "backup_verify_status: `ok`", "verification_failures: `0`", "backup_schema_version: `1`", "issue: `#8`", "issue_backup_path: `issues/000008.json`", "backup_event_name: `issue_comment`", "payload_bytes:", "payload_sha256_12:", "labels: `2`", "comments: `2`", "transcript_messages: `2`", "user_messages: `1`", "assistant_messages: `1`", "assistant_turn_comments: `1`", "error_comments: `1`", "issue_title_sha256_12:", "issue_body_sha256_12:", "raw_bodies_included: `false`", "gitclaw:e2e", "comment_1_sha256_12:", "message_1_sha256_12:"} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup info report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_INFO_TITLE_TOKEN", "BACKUP_INFO_BODY_TOKEN", "BACKUP_INFO_TRANSCRIPT_TOKEN", "BACKUP_INFO_ASSISTANT_TOKEN", "BACKUP_INFO_COMMENT_TOKEN", "BACKUP_INFO_ERROR_TOKEN", "@gitclaw info title"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup info leaked body/title token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupCoverageReportsOneIssueWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw coverage title BACKUP_COVERAGE_TITLE_SECRET",
			Body:   "BACKUP_COVERAGE_BODY_SECRET",
			Labels: []string{"gitclaw:e2e", "gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "BACKUP_COVERAGE_TRANSCRIPT_TOKEN", Actor: "alice", Trusted: true},
			{Role: "assistant", Body: "BACKUP_COVERAGE_ASSISTANT_TOKEN", Actor: "github-actions[bot]", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nBACKUP_COVERAGE_COMMENT_TOKEN"},
			{ID: 13, Body: "<!-- gitclaw:error -->\nBACKUP_COVERAGE_ERROR_TOKEN"},
		},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	coverage, err := BuildBackupCoverage(root, "owner/repo", 8)
	if err != nil {
		t.Fatalf("BuildBackupCoverage returned error: %v", err)
	}
	if !coverage.OK() || !coverage.IssueIndexed || !coverage.IssueBackupPayloadReadable || !coverage.IssueBackupPathCanonical || coverage.Comments != 2 || coverage.TranscriptMessages != 2 || coverage.AssistantTurns != 1 || coverage.ErrorComments != 1 {
		t.Fatalf("unexpected backup coverage metadata: %#v", coverage)
	}
	report := RenderBackupCoverage(coverage)
	for _, want := range []string{"GitClaw Backup Coverage Report", "repository: `owner/repo`", "backup_coverage_status: `ok`", "backup_verify_status: `ok`", "verification_failures: `0`", "backup_schema_version: `1`", "issue: `#8`", "issue_indexed: `true`", "expected_issue_backup_path: `issues/000008.json`", "issue_backup_path: `issues/000008.json`", "issue_backup_path_canonical: `true`", "issue_backup_payload_readable: `true`", "payload_bytes:", "payload_sha256_12:", "backup_event_name: `issue_comment`", "labels: `2`", "comments: `2`", "transcript_messages: `2`", "assistant_turn_comments: `1`", "error_comments: `1`", "issue_title_sha256_12:", "issue_body_sha256_12:", "raw_bodies_included: `false`", "llm_e2e_required_after_backup_coverage_change: `true`", "index_entry=`present`", "mutation_performed=`false`"} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup coverage report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_COVERAGE_TITLE_SECRET", "BACKUP_COVERAGE_BODY_SECRET", "BACKUP_COVERAGE_TRANSCRIPT_TOKEN", "BACKUP_COVERAGE_ASSISTANT_TOKEN", "BACKUP_COVERAGE_COMMENT_TOKEN", "BACKUP_COVERAGE_ERROR_TOKEN", "@gitclaw coverage title"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup coverage leaked body/title token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupDrillComposesRestoreReadinessWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:30:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 10,
			Title:  "@gitclaw drill title BACKUP_DRILL_TITLE_SECRET",
			Body:   "BACKUP_DRILL_BODY_SECRET",
			Labels: []string{"gitclaw:e2e", "gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "BACKUP_DRILL_TRANSCRIPT_TOKEN", Actor: "alice", Trusted: true},
			{Role: "assistant", Body: "BACKUP_DRILL_ASSISTANT_TOKEN", Actor: "github-actions[bot]", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nBACKUP_DRILL_COMMENT_TOKEN"},
			{ID: 13, Body: "<!-- gitclaw:error -->\nBACKUP_DRILL_ERROR_TOKEN"},
		},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	drill, err := BuildBackupDrill(root, "owner/repo", 10, "owner/restored")
	if err != nil {
		t.Fatalf("BuildBackupDrill returned error: %v", err)
	}
	if !drill.OK() || drill.TargetRepo != "owner/restored" || drill.BackupVerifyStatus != "ok" || drill.BackupCoverageStatus != "ok" || drill.RestorePlanStatus != "ok" || !drill.RestorePlanAvailable || drill.Comments != 2 || drill.TranscriptMessages != 2 || drill.AssistantTurns != 1 || drill.ErrorComments != 1 {
		t.Fatalf("unexpected backup drill metadata: %#v", drill)
	}
	report := RenderBackupDrill(drill)
	for _, want := range []string{"GitClaw Backup Drill Report", "repository: `owner/repo`", "target_repository: `owner/restored`", "backup_drill_status: `ok`", "backup_verify_status: `ok`", "backup_coverage_status: `ok`", "restore_plan_status: `ok`", "restore_mode: `dry-run`", "verification_failures: `0`", "backup_schema_version: `1`", "issue: `#10`", "issue_indexed: `true`", "expected_issue_backup_path: `issues/000010.json`", "issue_backup_path: `issues/000010.json`", "issue_backup_payload_readable: `true`", "comments: `2`", "transcript_messages: `2`", "assistant_turn_comments: `1`", "error_comments: `1`", "restore_plan_available: `true`", "raw_bodies_included: `false`", "mutation_performed: `false`", "github_api_calls_performed: `false`", "llm_e2e_required_after_backup_drill_change: `true`", "verify_gate=`pass`", "coverage_gate=`pass`", "restore_plan_gate=`pass`", "comment_1_sha256_12:", "message_1_sha256_12:"} {
		if !strings.Contains(report, want) {
			t.Fatalf("backup drill report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_DRILL_TITLE_SECRET", "BACKUP_DRILL_BODY_SECRET", "BACKUP_DRILL_TRANSCRIPT_TOKEN", "BACKUP_DRILL_ASSISTANT_TOKEN", "BACKUP_DRILL_COMMENT_TOKEN", "BACKUP_DRILL_ERROR_TOKEN", "@gitclaw drill title"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("backup drill leaked body/title token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupCoverageReportsMissingIssue(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 8, Title: "@gitclaw covered"},
		Transcript:  []TranscriptMessage{{Role: "user"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	coverage, err := BuildBackupCoverage(root, "owner/repo", 9)
	if err != nil {
		t.Fatalf("BuildBackupCoverage returned error: %v", err)
	}
	if coverage.OK() || coverage.BackupCoverageStatus != "missing" || coverage.IssueIndexed || coverage.IssueBackupPayloadReadable {
		t.Fatalf("unexpected missing coverage metadata: %#v", coverage)
	}
	report := RenderBackupCoverage(coverage)
	for _, want := range []string{"backup_coverage_status: `missing`", "issue: `#9`", "issue_indexed: `false`", "expected_issue_backup_path: `issues/000009.json`", "issue_backup_path: `none`", "index_entry=`missing`", "payload_read=`skipped`"} {
		if !strings.Contains(report, want) {
			t.Fatalf("missing coverage report missing %q:\n%s", want, report)
		}
	}
}

func TestBuildBackupCoverageReportsUnreadableIndexedPayload(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw coverage unreadable BACKUP_COVERAGE_UNREADABLE_TITLE_SECRET",
			Body:   "BACKUP_COVERAGE_UNREADABLE_BODY_SECRET",
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "BACKUP_COVERAGE_UNREADABLE_TRANSCRIPT_SECRET"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	if err := os.Remove(issueBackupPath(root, "owner/repo", 8)); err != nil {
		t.Fatal(err)
	}

	coverage, err := BuildBackupCoverage(root, "owner/repo", 8)
	if err != nil {
		t.Fatalf("BuildBackupCoverage returned error: %v", err)
	}
	if coverage.OK() || coverage.BackupCoverageStatus != "warn" || coverage.BackupVerifyStatus != "warn" || !coverage.IssueIndexed || coverage.IssueBackupPayloadReadable {
		t.Fatalf("unexpected unreadable coverage metadata: %#v", coverage)
	}
	report := RenderBackupCoverage(coverage)
	for _, want := range []string{"backup_coverage_status: `warn`", "backup_verify_status: `warn`", "issue_indexed: `true`", "issue_backup_path: `issues/000008.json`", "issue_backup_payload_readable: `false`", "payload_read=`false`"} {
		if !strings.Contains(report, want) {
			t.Fatalf("unreadable coverage report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BACKUP_COVERAGE_UNREADABLE_TITLE_SECRET", "BACKUP_COVERAGE_UNREADABLE_BODY_SECRET", "BACKUP_COVERAGE_UNREADABLE_TRANSCRIPT_SECRET", "@gitclaw coverage unreadable"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("unreadable coverage leaked body/title token %q:\n%s", leaked, report)
		}
	}
}

func TestBuildBackupSearchFindsBackedUpConversationWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number:            7,
			Title:             "@gitclaw backup search title BACKUP_SEARCH_TITLE_SECRET",
			Body:              "Backup search issue body has BACKUP_SEARCH_BODY_SECRET and retrieval notes.",
			Author:            "alice",
			AuthorAssociation: "OWNER",
			Labels:            []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "retrieval transcript token BACKUP_SEARCH_TRANSCRIPT_SECRET", Actor: "alice", AuthorAssociation: "OWNER", Trusted: true},
			{Role: "assistant", Body: "assistant backup search reply BACKUP_SEARCH_ASSISTANT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 12, Trusted: true},
		},
		Comments: []IssueBackupComment{{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nBACKUP_SEARCH_COMMENT_SECRET", Author: "github-actions[bot]", AuthorAssociation: "NONE"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 8, Title: "@gitclaw unrelated", Body: "OTHER_BACKUP_SEARCH_SECRET"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "other body"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	report, err := BuildBackupSearch(root, "owner/repo", "retrieval BACKUP_SEARCH_QUERY_SECRET", 2)
	if err != nil {
		t.Fatalf("BuildBackupSearch returned error: %v", err)
	}
	if report.SearchStatus != "ok" || report.BackupVerifyStatus != "ok" || report.IssueCount != 2 || report.MatchedIssues != 1 || report.MatchedLines != 2 || report.ResultsReturned != 2 {
		t.Fatalf("unexpected backup search report: %#v", report)
	}
	rendered := RenderBackupSearchReport(report)
	for _, want := range []string{"GitClaw Backup Search Report", "backup_search_status: `ok`", "backup_verify_status: `ok`", "query_sha256_12:", "query_terms:", "max_results: `2`", "issue_count: `2`", "issue_fields_searched: `4`", "comment_bodies_searched: `1`", "transcript_messages_searched: `3`", "matched_issues: `1`", "matched_lines: `2`", "results_returned: `2`", "raw_bodies_included: `false`", "issue=`#7` path=`issues/000007.json`", "source=`issue.body`", "source=`transcript:01`", "line_sha256_12="} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("backup search report missing %q:\n%s", want, rendered)
		}
	}
	for _, leaked := range []string{"BACKUP_SEARCH_TITLE_SECRET", "BACKUP_SEARCH_BODY_SECRET", "BACKUP_SEARCH_TRANSCRIPT_SECRET", "BACKUP_SEARCH_ASSISTANT_SECRET", "BACKUP_SEARCH_COMMENT_SECRET", "OTHER_BACKUP_SEARCH_SECRET", "BACKUP_SEARCH_QUERY_SECRET", "retrieval BACKUP_SEARCH_QUERY_SECRET", "@gitclaw backup search title"} {
		if strings.Contains(rendered, leaked) {
			t.Fatalf("backup search report leaked %q:\n%s", leaked, rendered)
		}
	}
}

func TestBuildBackupRetentionPlanKeepsNewestWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw retention oldest",
			Body:   "RETENTION_OLDEST_BODY_SECRET",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "RETENTION_OLDEST_TRANSCRIPT_SECRET"}},
		Comments:   []IssueBackupComment{{ID: 11, Body: "RETENTION_OLDEST_COMMENT_SECRET"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw retention middle",
			Body:   "RETENTION_MIDDLE_BODY_SECRET",
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "RETENTION_MIDDLE_TRANSCRIPT_SECRET"}, {Role: "assistant", Body: "RETENTION_MIDDLE_ASSISTANT_SECRET"}},
		Comments:   []IssueBackupComment{{ID: 12, Body: "RETENTION_MIDDLE_COMMENT_SECRET"}},
	})
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T14:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 9,
			Title:  "@gitclaw retention newest",
			Body:   "RETENTION_NEWEST_BODY_SECRET",
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "RETENTION_NEWEST_TRANSCRIPT_SECRET"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 29, 15, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	plan, err := BuildBackupRetentionPlan(root, "owner/repo", 2)
	if err != nil {
		t.Fatalf("BuildBackupRetentionPlan returned error: %v", err)
	}
	if plan.RetentionPlanStatus != "ok" || plan.BackupVerifyStatus != "ok" || plan.KeepLatest != 2 || plan.IssueCount != 3 || plan.KeepCount != 2 || plan.PruneCandidateCount != 1 {
		t.Fatalf("unexpected retention plan metadata: %#v", plan)
	}
	if plan.Kept[0].IssueNumber != 9 || plan.Kept[1].IssueNumber != 8 || plan.PruneCandidates[0].IssueNumber != 7 {
		t.Fatalf("unexpected retention ordering: kept=%#v prune=%#v", plan.Kept, plan.PruneCandidates)
	}
	if plan.NewestKeptIssueNumber != 9 || plan.OldestKeptIssueNumber != 8 {
		t.Fatalf("unexpected kept boundary metadata: %#v", plan)
	}
	report := RenderBackupRetentionPlan(plan)
	for _, want := range []string{"GitClaw Backup Retention Plan", "retention_mode: `dry-run`", "backup_retention_status: `ok`", "backup_verify_status: `ok`", "verification_failures: `0`", "keep_latest: `2`", "issue_count: `3`", "keep_count: `2`", "prune_candidate_count: `1`", "newest_kept_issue: `#9`", "oldest_kept_issue: `#8`", "raw_bodies_included: `false`", "### Kept Backups", "issue=#9 path=`issues/000009.json`", "issue=#8 path=`issues/000008.json`", "### Prune Candidates", "issue=#7 path=`issues/000007.json`", "title_sha256_12="} {
		if !strings.Contains(report, want) {
			t.Fatalf("retention plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"RETENTION_OLDEST_BODY_SECRET", "RETENTION_OLDEST_TRANSCRIPT_SECRET", "RETENTION_OLDEST_COMMENT_SECRET", "RETENTION_MIDDLE_BODY_SECRET", "RETENTION_MIDDLE_TRANSCRIPT_SECRET", "RETENTION_MIDDLE_ASSISTANT_SECRET", "RETENTION_MIDDLE_COMMENT_SECRET", "RETENTION_NEWEST_BODY_SECRET", "RETENTION_NEWEST_TRANSCRIPT_SECRET", "@gitclaw retention oldest", "@gitclaw retention middle", "@gitclaw retention newest"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("retention plan leaked body/title token %q:\n%s", leaked, report)
		}
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
