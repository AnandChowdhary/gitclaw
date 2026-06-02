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

func TestChannelDoneArtifactRefSupportsIdeaIssues(t *testing.T) {
	body := RenderChannelIdeaIssueBody(ChannelIdeaOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		IdeaID:            "idea-done-1",
		Title:             "Let channel brainstorms become GitHub idea labs",
		Notes:             "Shape the idea before converting it into a task or skill.",
		SourceIssueNumber: 45,
		SourceCommentID:   4500,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "idea" || ref.ID != "idea-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 45 || ref.SourceCommentID != 4500 {
		t.Fatalf("unexpected idea artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsRetroIssues(t *testing.T) {
	body := RenderChannelRetroIssueBody(ChannelRetroOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		RetroID:           "retro-done-1",
		Title:             "Launch readiness retro",
		WentWell:          "People knew where the decision log lived.",
		RoughEdges:        "Release checklist updates landed late.",
		Next:              "Move follow-up experiments into GitHub.",
		SourceIssueNumber: 46,
		SourceCommentID:   4600,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "retro" || ref.ID != "retro-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 46 || ref.SourceCommentID != 4600 {
		t.Fatalf("unexpected retro artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsPlaybookIssues(t *testing.T) {
	body := RenderChannelPlaybookIssueBody(ChannelPlaybookOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		PlaybookID:        "playbook-done-1",
		Title:             "Deploy the channel gateway",
		Steps:             "Check the route, then dispatch the workflow.",
		Checks:            "Confirm the outbox drains cleanly.",
		Rollback:          "Pause the route and reopen the previous issue.",
		SourceIssueNumber: 47,
		SourceCommentID:   4700,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "playbook" || ref.ID != "playbook-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 47 || ref.SourceCommentID != 4700 {
		t.Fatalf("unexpected playbook artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsInsightIssues(t *testing.T) {
	body := RenderChannelInsightIssueBody(ChannelInsightOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		InsightID:         "insight-done-1",
		Title:             "Release readiness signal",
		Observation:       "The channel had enough context to decide next steps.",
		Evidence:          "The linked checklist was updated after the discussion.",
		Recommendation:    "Keep the follow-up in GitHub.",
		SourceIssueNumber: 48,
		SourceCommentID:   4800,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "insight" || ref.ID != "insight-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 48 || ref.SourceCommentID != 4800 {
		t.Fatalf("unexpected insight artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsIncidentIssues(t *testing.T) {
	body := RenderChannelIncidentIssueBody(ChannelIncidentOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		IncidentID:        "incident-done-1",
		Severity:          "sev1",
		Title:             "Production bot channel is not receiving messages",
		Notes:             "Keep triage and resolution notes in GitHub.",
		SourceIssueNumber: 46,
		SourceCommentID:   4600,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "incident" || ref.ID != "incident-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 46 || ref.SourceCommentID != 4600 {
		t.Fatalf("unexpected incident artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsVoiceIssues(t *testing.T) {
	body := RenderChannelVoiceIssueBody(ChannelVoiceOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		VoiceID:           "voice-done-1",
		Title:             "Visible voice note",
		Transcript:        "Durable transcript.",
		DurationSeconds:   42,
		MediaType:         "audio/ogg",
		AudioURL:          "https://media.example.invalid/voice.ogg",
		SourceIssueNumber: 47,
		SourceCommentID:   4700,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "voice" || ref.ID != "voice-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 47 || ref.SourceCommentID != 4700 {
		t.Fatalf("unexpected voice artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsImageIssues(t *testing.T) {
	body := RenderChannelImageIssueBody(ChannelImageOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		ImageID:           "image-done-1",
		Title:             "Visible image note",
		Description:       "Durable visual context.",
		Width:             1280,
		Height:            720,
		MediaType:         "image/png",
		SourceURL:         "https://media.example.invalid/image.png",
		SourceIssueNumber: 48,
		SourceCommentID:   4800,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "image" || ref.ID != "image-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 48 || ref.SourceCommentID != 4800 {
		t.Fatalf("unexpected image artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsLinkIssues(t *testing.T) {
	body := RenderChannelLinkIssueBody(ChannelLinkOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		LinkID:            "link-done-1",
		LinkURL:           "https://example.invalid/link-done-secret",
		Title:             "Visible link card",
		Notes:             "Durable link context.",
		SourceIssueNumber: 49,
		SourceCommentID:   4900,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "link" || ref.ID != "link-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 49 || ref.SourceCommentID != 4900 {
		t.Fatalf("unexpected link artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsAccessRequestIssues(t *testing.T) {
	body := RenderChannelAccessRequestIssueBody(ChannelAccessRequestOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		AccessID:          "access-done-1",
		Requester:         "Visible requester",
		ProviderUserID:    "provider-secret-user",
		ProviderHandle:    "@provider-secret",
		Scope:             "team-demo",
		RequestedRole:     "user",
		Reason:            "Review complete.",
		SourceIssueNumber: 50,
		SourceCommentID:   5000,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "access-request" || ref.ID != "access-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 50 || ref.SourceCommentID != 5000 {
		t.Fatalf("unexpected access request artifact ref: %#v", ref)
	}
}

func TestChannelDoneArtifactRefSupportsContactIssues(t *testing.T) {
	body := RenderChannelContactIssueBody(ChannelContactOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		ThreadID:          "team-thread-1",
		SourceMessageID:   "source-message-1",
		ContactID:         "contact-done-1",
		DisplayName:       "Visible contact",
		ProviderUserID:    "provider-secret-user",
		ProviderHandle:    "@provider-secret",
		ContactRole:       "reviewer",
		Notes:             "Contact reviewed.",
		SourceIssueNumber: 51,
		SourceCommentID:   5100,
	})
	ref, err := channelDoneArtifactRefFromBody(body)
	if err != nil {
		t.Fatalf("channelDoneArtifactRefFromBody returned error: %v", err)
	}
	if ref.Kind != "contact" || ref.ID != "contact-done-1" || ref.Channel != "slack" || ref.SourceIssueNumber != 51 || ref.SourceCommentID != 5100 {
		t.Fatalf("unexpected contact artifact ref: %#v", ref)
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
