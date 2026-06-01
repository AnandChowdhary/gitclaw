package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelStatusQueuesCurrentThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-status-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 270,
			"title": "GitClaw telegram thread chat-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 27001,
			"body": "@gitclaw /channels status --message-id inbound-270 --status-id progress-270 --state working\nWorking through the request with CHANNEL_STATUS_BODY_SECRET.\nDo not leak CHANNEL_STATUS_SOURCE_SECRET in the receipt.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 270,
			Title:  "GitClaw telegram thread chat-status-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{270: {{
			ID: 27000,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-status-123",
				MessageID: "inbound-270",
				Author:    "telegram",
				Body:      "Original mirrored message.",
			}),
		}}},
		IssueLabels: map[int][]string{270: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel status action", llm.Calls)
	}
	comments := github.CommentsByIssue[270]
	if len(comments) != 3 {
		t.Fatalf("comments = %d, want message + status + receipt: %#v", len(comments), comments)
	}
	status := comments[1].Body
	for _, want := range []string{"gitclaw:channel-status", `channel="telegram"`, `thread_id="chat-status-123"`, `target_message_id="inbound-270"`, `status_id="progress-270"`, `state="working"`, "CHANNEL_STATUS_BODY_SECRET"} {
		if !strings.Contains(status, want) {
			t.Fatalf("status comment missing %q:\n%s", want, status)
		}
	}
	receipt := comments[2].Body
	for _, want := range []string{
		"GitClaw Channel Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels status`",
		"channel_status_status: `queued`",
		"target_issue: `#270`",
		"status_comment_id: `9000`",
		"target_issue_created: `false`",
		"duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"target_issue_is_source: `true`",
		"status_body_sha256_12:",
		"raw_target_message_id_included: `false`",
		"raw_status_id_included: `false`",
		"raw_status_state_included: `false`",
		"raw_status_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_STATUS_SOURCE_SECRET", "CHANNEL_STATUS_BODY_SECRET", "chat-status-123", "inbound-270", "progress-270", "working"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel status receipt leaked %q:\n%s", leaked, receipt)
		}
	}
	if !hasLabel(github.IssueLabels[270], "gitclaw:done") || hasLabel(github.IssueLabels[270], "gitclaw:running") || hasLabel(github.IssueLabels[270], "gitclaw:error") {
		t.Fatalf("unexpected source labels: %#v", github.IssueLabels[270])
	}
}

func TestRunChannelStatusDedupesAndOutboxDelivers(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 19,
			Title:  "GitClaw slack thread team-status",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "slack",
				ThreadID: "team-status",
			}),
			Labels: []string{cfg.ChannelLabel},
		}},
		CommentsByIssue: map[int][]Comment{19: nil},
		IssueLabels:     map[int][]string{19: []string{cfg.ChannelLabel}},
	}
	result, err := RunChannelStatus(context.Background(), cfg, github, ChannelStatusOptions{
		Repo:            "owner/repo",
		Channel:         "slack",
		ThreadID:        "team-status",
		TargetMessageID: "source-msg-1",
		StatusID:        "status-1",
		State:           "Working",
		Body:            "Working through the request.",
	})
	if err != nil {
		t.Fatalf("RunChannelStatus returned error: %v", err)
	}
	if result.IssueNumber != 19 || result.CommentID == 0 || result.Created || result.Duplicate || result.StatusIDHash == "" || result.StateHash == "" {
		t.Fatalf("unexpected status result: %#v", result)
	}
	comment := github.CommentsByIssue[19][0]
	if !HasChannelStatusMarker(comment.Body) || !strings.Contains(comment.Body, `state="working"`) {
		t.Fatalf("status comment missing marker/normalized state:\n%s", comment.Body)
	}

	duplicate, err := RunChannelStatus(context.Background(), cfg, github, ChannelStatusOptions{
		Repo:            "owner/repo",
		Channel:         "slack",
		ThreadID:        "team-status",
		TargetMessageID: "source-msg-1",
		StatusID:        "status-1",
		State:           "working",
		Body:            "A duplicate body should not post.",
	})
	if err != nil {
		t.Fatalf("duplicate RunChannelStatus returned error: %v", err)
	}
	if !duplicate.Duplicate || duplicate.CommentID != 0 || len(github.CommentsByIssue[19]) != 1 {
		t.Fatalf("duplicate status not suppressed: result=%#v comments=%#v", duplicate, github.CommentsByIssue[19])
	}

	outbox, err := RunChannelOutbox(context.Background(), cfg, github, ChannelOutboxOptions{
		Repo:        "owner/repo",
		Channel:     "slack",
		AccountID:   "slack-account",
		IssueNumber: 19,
	})
	if err != nil {
		t.Fatalf("RunChannelOutbox returned error: %v", err)
	}
	if outbox.SourceStatusComments != 1 || outbox.SourceDeliverableComments != 1 || outbox.PendingMessages != 1 || len(outbox.Messages) != 1 {
		t.Fatalf("unexpected status outbox result: %#v", outbox)
	}
	if outbox.Messages[0].Kind != "channel-status" {
		t.Fatalf("outbox kind = %q, want channel-status", outbox.Messages[0].Kind)
	}

	delivery, err := RunChannelDelivery(context.Background(), cfg, github, ChannelDeliveryOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		AccountID:         "slack-account",
		IssueNumber:       19,
		CommentID:         comment.ID,
		ExternalMessageID: "status-delivered-1",
	})
	if err != nil {
		t.Fatalf("RunChannelDelivery returned error: %v", err)
	}
	if !delivery.Delivered || delivery.Duplicate || delivery.StateIssueNumber == 0 {
		t.Fatalf("unexpected delivery result: %#v", delivery)
	}
}

func TestBuildChannelStatusActionRequestParsesRouteAndBody(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 8,
			Title:  "@gitclaw /channels progress team-demo --message-id source-1 --status-id Progress.One --state In-Progress",
			Body:   "Status body line.",
		},
	}
	req, err := BuildChannelStatusActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelStatusActionRequest returned error: %v", err)
	}
	if req.Subcommand != "progress" || req.Options.Route != "team-demo" || req.Options.TargetMessageID != "source-1" || req.Options.StatusID != "progress-one" || req.Options.State != "in-progress" {
		t.Fatalf("unexpected status parsing: %#v", req)
	}
	if req.BodySource != "trailing-lines" || req.Options.Body != "Status body line." {
		t.Fatalf("unexpected status body parsing: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.RequestedTargetMsgHash == "" || req.RequestedStatusIDHash == "" || req.RequestedStatusStateSHA == "" {
		t.Fatalf("expected route/message/status hashes: %#v", req)
	}
}
