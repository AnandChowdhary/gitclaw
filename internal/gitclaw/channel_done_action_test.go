package gitclaw

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleChannelDoneActionClosesArtifactAndQueuesAckWithoutLLM(t *testing.T) {
	cfg := DefaultConfig()
	sourceIssue := Issue{
		Number: 50,
		Title:  "GitClaw telegram thread chat-done-123",
		Body: RenderChannelThreadBody(ChannelIngestOptions{
			Repo:     "owner/repo",
			Channel:  "telegram",
			ThreadID: "chat-done-123",
		}),
		Labels: []string{cfg.ChannelLabel},
	}
	taskIssueBody := RenderChannelTaskIssueBody(ChannelTaskOptions{
		Repo:              "owner/repo",
		Channel:           "telegram",
		ThreadID:          "chat-done-123",
		SourceMessageID:   "source-message-1",
		TaskID:            "task-done-1",
		Title:             "Follow up visible task title",
		Notes:             "Task note with CHANNEL_DONE_TASK_NOTE_SECRET.",
		SourceIssueNumber: sourceIssue.Number,
		SourceCommentID:   700,
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 51,
			"title": "GitClaw channel task: Follow up visible task title",
			"body": `+channelDoneQuoteJSON(t, taskIssueBody)+`,
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 91,
			"body": "@gitclaw /channels done --message-id done-notify-1\nDo not leak done body token CHANNEL_DONE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{
		Issues: []Issue{
			sourceIssue,
			{Number: 51, Title: ev.Issue.Title, Body: ev.Issue.Body, Labels: []string{cfg.TriggerLabel}},
		},
		CommentsByIssue: map[int][]Comment{50: nil, 51: nil},
		IssueLabels: map[int][]string{
			50: {cfg.ChannelLabel},
			51: {cfg.TriggerLabel},
		},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel done action", llm.Calls)
	}
	if !github.ClosedIssues[51] {
		t.Fatalf("channel done action did not close task issue")
	}
	sourceComments := github.CommentsByIssue[50]
	if len(sourceComments) != 1 {
		t.Fatalf("source channel comments = %d, want one done acknowledgement: %#v", len(sourceComments), sourceComments)
	}
	ack := sourceComments[0].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`thread_id="chat-done-123"`,
		`message_id="done-notify-1"`,
		"GitClaw channel task completed",
		"Artifact issue: #51 https://github.com/owner/repo/issues/51",
		"Kind: task",
		"State: closed",
		"Provider delivery performed: false",
	} {
		if !strings.Contains(ack, want) {
			t.Fatalf("channel done acknowledgement missing %q:\n%s", want, ack)
		}
	}
	for _, leaked := range []string{"CHANNEL_DONE_TASK_NOTE_SECRET", "CHANNEL_DONE_BODY_SECRET", "source-message-1", "Follow up visible task title"} {
		if strings.Contains(ack, leaked) {
			t.Fatalf("channel done acknowledgement leaked %q:\n%s", leaked, ack)
		}
	}

	if len(github.CommentsByIssue[51]) != 1 {
		t.Fatalf("artifact comments = %d, want source receipt: %#v", len(github.CommentsByIssue[51]), github.CommentsByIssue[51])
	}
	receipt := github.CommentsByIssue[51][0].Body
	for _, want := range []string{
		"GitClaw Channel Done Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels done`",
		"channel_artifact_kind: `task`",
		"channel_artifact_issue: `#51`",
		"channel_artifact_closed: `true`",
		"source_channel_issue: `#50`",
		"notification_target_issue: `#50`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"channel: `telegram`",
		"raw_artifact_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_artifact_title_included: `false`",
		"raw_artifact_body_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_done_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel done receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"task-done-1", "chat-done-123", "done-notify-1", "Follow up visible task title", "CHANNEL_DONE_TASK_NOTE_SECRET", "CHANNEL_DONE_BODY_SECRET"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel done receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 51,
			"title": "GitClaw channel task: Follow up visible task title",
			"body": `+channelDoneQuoteJSON(t, taskIssueBody)+`,
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 92,
			"body": "@gitclaw /channels done --message-id done-notify-1\nDo not leak duplicate done token CHANNEL_DONE_DUPLICATE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, cfg, github, llm); err != nil {
		t.Fatalf("duplicate Handle returned error: %v", err)
	}
	if len(github.CommentsByIssue[50]) != 1 {
		t.Fatalf("duplicate channel done queued another acknowledgement: %#v", github.CommentsByIssue[50])
	}
	duplicateReceipt := github.CommentsByIssue[51][1].Body
	for _, want := range []string{
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"channel_artifact_closed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel done receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_DONE_DUPLICATE_SECRET", "task-done-1", "chat-done-123", "done-notify-1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel done receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestChannelDoneArtifactRefSupportsDecisionIssues(t *testing.T) {
	body := RenderChannelDecisionIssueBody(ChannelDecisionOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		DecisionID:        "decision-done-1",
		Decision:          "Use GitHub issues as decision logs",
		Rationale:         "Durable and reviewable.",
		SourceIssueNumber: 42,
		SourceCommentID:   4200,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "decision" || ref.ID != "decision-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 42 || ref.SourceCommentID != 4200 {
		t.Fatalf("unexpected decision artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsWatchIssues(t *testing.T) {
	body := RenderChannelWatchIssueBody(ChannelWatchOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		WatchID:           "watch-done-1",
		Cadence:           "daily",
		Title:             "Watch the customer escalation queue",
		Notes:             "Escalate if there is no owner.",
		SourceIssueNumber: 44,
		SourceCommentID:   4400,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "watch" || ref.ID != "watch-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 44 || ref.SourceCommentID != 4400 {
		t.Fatalf("unexpected watch artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsStandingOrderProposalIssues(t *testing.T) {
	body := RenderChannelStandingOrderProposalIssueBody(ChannelStandingOrderProposalOptions{
		Repo:              "owner/repo",
		Channel:           "telegram",
		ThreadID:          "thread-order-done",
		SourceMessageID:   "source-order-done",
		ProposalID:        "order-done-1",
		Cadence:           "weekly",
		Title:             "Weekly triage",
		ProposalBody:      "## Program: Weekly triage\n**Authority:** Draft a private summary.",
		SourceIssueNumber: 42,
		SourceCommentID:   4201,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "standing-order-proposal" || ref.ID != "order-done-1" || ref.Channel != "telegram" || ref.SourceIssueNumber != 42 || ref.SourceCommentID != 4201 {
		t.Fatalf("unexpected standing-order proposal ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsBackupRestoreRequestIssues(t *testing.T) {
	body := RenderBackupRestoreRequestIssueBody(BackupRestoreRequestIssueRequest{
		Repo:              "owner/repo",
		RequestID:         "restore-done-1",
		BackupIssueNumber: 42,
		TargetRepo:        "owner/repo",
		BackupBranch:      "gitclaw-backups",
		BackupRoot:        ".gitclaw/backups",
		RepoBackupDir:     ".gitclaw/backups/owner__repo",
		IssueBackupPath:   ".gitclaw/backups/owner__repo/issues/000042.json",
		IndexPath:         ".gitclaw/backups/owner__repo/index.json",
		VerifyCmd:         "gitclaw backup verify --root .gitclaw/backups --repo owner/repo",
		CoverageCmd:       "gitclaw backup coverage --root .gitclaw/backups --repo owner/repo --issue 42",
		DrillCmd:          "gitclaw backup drill --root .gitclaw/backups --repo owner/repo --issue 42",
		RestorePlanCmd:    "gitclaw backup restore-plan --root .gitclaw/backups --repo owner/repo --target-repo owner/repo --issue 42",
		ManifestCmd:       "gitclaw backup manifest --root .gitclaw/backups --repo owner/repo --issue 42",
		SourceIssueNumber: 43,
		SourceCommentID:   4301,
		SourceKind:        "channel_comment",
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "backup-restore-request" || ref.ID != "restore-done-1" || ref.SourceIssueNumber != 43 || ref.SourceCommentID != 4301 {
		t.Fatalf("unexpected backup restore request ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsCheckpointRehearsalIssues(t *testing.T) {
	body := RenderCheckpointRehearsalIssueBody(CheckpointRehearsalIssueRequest{
		Repo:                 "owner/repo",
		RehearsalID:          "checkpoint-done-1",
		TargetRef:            "HEAD~1",
		TargetRefSHA:         shortDocumentHash("HEAD~1"),
		TargetAllowed:        true,
		CheckpointStatus:     "clean",
		GitAvailable:         true,
		GitRepository:        true,
		Branch:               "main",
		HeadCommit:           "abcdef123456",
		TargetCommit:         "123456abcdef",
		ComparisonRangeSHA:   shortDocumentHash("HEAD~1..HEAD"),
		RestoreMode:          "rehearsal-only",
		SourceIssueNumber:    44,
		SourceCommentID:      4401,
		SourceKind:           "channel_comment",
		CheckpointStatusCmd:  "gitclaw checkpoints status",
		CheckpointPreviewCmd: "gitclaw checkpoints preview HEAD~1",
		CheckpointRiskCmd:    "gitclaw checkpoints risk",
		RollbackDiffCmd:      "gitclaw rollback diff HEAD~1",
		RollbackRiskCmd:      "gitclaw rollback risk",
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "checkpoint-rehearsal" || ref.ID != "checkpoint-done-1" || ref.SourceIssueNumber != 44 || ref.SourceCommentID != 4401 {
		t.Fatalf("unexpected checkpoint rehearsal ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsDigestIssues(t *testing.T) {
	body := RenderChannelDigestIssueBody(ChannelDigestOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		DigestID:          "digest-done-1",
		Summary:           "Team channel reached a useful checkpoint",
		Highlights:        "Move the rest of the follow-up to GitHub.",
		SourceIssueNumber: 43,
		SourceCommentID:   4300,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "digest" || ref.ID != "digest-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 43 || ref.SourceCommentID != 4300 {
		t.Fatalf("unexpected digest artifact ref: %#v", ref)
	}
}

func channelDoneQuoteJSON(t *testing.T, value string) string {
	t.Helper()
	quoted, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return string(quoted)
}
