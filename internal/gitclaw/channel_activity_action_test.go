package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelActivityQueuesCurrentThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-activity-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 281,
			"title": "GitClaw telegram thread chat-activity-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-activity-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 28101,
			"body": "@gitclaw /channels activity --activity-id activity-281 --message-id inbound-281 --activity uploading --ttl-seconds 30\nDo not leak CHANNEL_ACTIVITY_SOURCE_SECRET in the receipt.",
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
			Number: 281,
			Title:  "GitClaw telegram thread chat-activity-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{281: {{
			ID: 28100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-activity-123",
				MessageID: "inbound-281",
				Author:    "telegram",
				Body:      "Original mirrored message.",
			}),
		}}},
		IssueLabels: map[int][]string{281: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel activity action", llm.Calls)
	}
	comments := github.CommentsByIssue[281]
	if len(comments) != 3 {
		t.Fatalf("comments = %d, want message + activity + receipt: %#v", len(comments), comments)
	}
	activity := comments[1].Body
	for _, want := range []string{"gitclaw:channel-activity", `channel="telegram"`, `thread_id="chat-activity-123"`, `target_message_id="inbound-281"`, `activity_id="activity-281"`, `activity="uploading"`, `ttl_seconds="30"`} {
		if !strings.Contains(activity, want) {
			t.Fatalf("activity comment missing %q:\n%s", want, activity)
		}
	}
	receipt := comments[2].Body
	for _, want := range []string{
		"GitClaw Channel Activity Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels activity`",
		"channel_activity_status: `queued`",
		"target_issue: `#281`",
		"activity_comment_id: `9000`",
		"target_issue_created: `false`",
		"duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"target_issue_is_source: `true`",
		"target_message_id_sha256_12:",
		"activity_id_sha256_12:",
		"activity_sha256_12:",
		"ttl_seconds: `30`",
		"raw_thread_id_included: `false`",
		"raw_target_message_id_included: `false`",
		"raw_activity_id_included: `false`",
		"raw_activity_included: `false`",
		"provider_delivery_performed: `false`",
		"provider_activity_performed: `false`",
		"repository_mutation_performed: `false`",
		"model_call_performed: `false`",
		"llm_e2e_required_after_channel_activity_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel activity receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ACTIVITY_SOURCE_SECRET", "chat-activity-123", "activity-281", "inbound-281", "uploading"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel activity receipt leaked %q:\n%s", leaked, receipt)
		}
	}
	if !hasLabel(github.IssueLabels[281], "gitclaw:done") || hasLabel(github.IssueLabels[281], "gitclaw:running") || hasLabel(github.IssueLabels[281], "gitclaw:error") {
		t.Fatalf("unexpected source labels: %#v", github.IssueLabels[281])
	}
}

func TestRunChannelActivityDedupesAndOutboxDelivers(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 20,
			Title:  "GitClaw slack thread team-activity",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "slack",
				ThreadID: "team-activity",
			}),
			Labels: []string{cfg.ChannelLabel},
		}},
		CommentsByIssue: map[int][]Comment{20: nil},
		IssueLabels:     map[int][]string{20: []string{cfg.ChannelLabel}},
	}
	result, err := RunChannelActivity(context.Background(), cfg, github, ChannelActivityOptions{
		Repo:            "owner/repo",
		Channel:         "slack",
		ThreadID:        "team-activity",
		TargetMessageID: "provider-msg-1",
		ActivityID:      "activity-1",
		Activity:        "recording",
		TTLSeconds:      12,
	})
	if err != nil {
		t.Fatalf("RunChannelActivity returned error: %v", err)
	}
	if result.IssueNumber != 20 || result.CommentID == 0 || result.Created || result.Duplicate || result.ActivityIDHash == "" || result.ActivityHash == "" || result.TargetMessageHash == "" {
		t.Fatalf("unexpected activity result: %#v", result)
	}
	comment := github.CommentsByIssue[20][0]
	if !HasChannelActivityMarker(comment.Body) || !strings.Contains(comment.Body, `activity_id="activity-1"`) || !strings.Contains(comment.Body, `activity="recording"`) {
		t.Fatalf("activity comment missing marker/body:\n%s", comment.Body)
	}

	duplicate, err := RunChannelActivity(context.Background(), cfg, github, ChannelActivityOptions{
		Repo:            "owner/repo",
		Channel:         "slack",
		ThreadID:        "team-activity",
		TargetMessageID: "provider-msg-1",
		ActivityID:      "activity-1",
		Activity:        "uploading",
		TTLSeconds:      30,
	})
	if err != nil {
		t.Fatalf("duplicate RunChannelActivity returned error: %v", err)
	}
	if !duplicate.Duplicate || duplicate.CommentID != 0 || len(github.CommentsByIssue[20]) != 1 {
		t.Fatalf("duplicate activity not suppressed: result=%#v comments=%#v", duplicate, github.CommentsByIssue[20])
	}

	outbox, err := RunChannelOutbox(context.Background(), cfg, github, ChannelOutboxOptions{
		Repo:        "owner/repo",
		Channel:     "slack",
		AccountID:   "slack-account",
		IssueNumber: 20,
	})
	if err != nil {
		t.Fatalf("RunChannelOutbox returned error: %v", err)
	}
	if outbox.SourceActivityComments != 1 || outbox.SourceDeliverableComments != 1 || outbox.PendingMessages != 1 || len(outbox.Messages) != 1 {
		t.Fatalf("unexpected activity outbox result: %#v", outbox)
	}
	if outbox.Messages[0].Kind != "channel-activity" {
		t.Fatalf("outbox kind = %q, want channel-activity", outbox.Messages[0].Kind)
	}

	delivery, err := RunChannelDelivery(context.Background(), cfg, github, ChannelDeliveryOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		AccountID:         "slack-account",
		IssueNumber:       20,
		CommentID:         comment.ID,
		ExternalMessageID: "activity-delivered-1",
	})
	if err != nil {
		t.Fatalf("RunChannelDelivery returned error: %v", err)
	}
	if !delivery.Delivered || delivery.Duplicate || delivery.StateIssueNumber == 0 {
		t.Fatalf("unexpected delivery result: %#v", delivery)
	}
}

func TestBuildChannelActivityActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 8,
			Title:  "@gitclaw /channels recording --route team-demo --activity-id Activity.One --message-id provider-1 --ttl 15",
			Body:   "Release room",
		},
	}
	req, err := BuildChannelActivityActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelActivityActionRequest returned error: %v", err)
	}
	if req.Subcommand != "recording" || req.Options.Route != "team-demo" || req.Options.ActivityID != "activity-one" || req.Options.TargetMessageID != "provider-1" || req.Options.Activity != "recording" || req.Options.TTLSeconds != 15 {
		t.Fatalf("unexpected activity parsing: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.RequestedTargetMsgHash == "" || req.RequestedActivityIDHash == "" || req.ActivityHash == "" {
		t.Fatalf("expected activity metadata: %#v", req)
	}
}
