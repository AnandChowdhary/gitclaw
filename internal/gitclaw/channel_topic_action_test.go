package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelTopicQueuesCurrentThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-topic-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 271,
			"title": "GitClaw telegram thread chat-topic-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-topic-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 27101,
			"body": "@gitclaw /channels topic --topic-id topic-271\nVisible topic CHANNEL_TOPIC_BODY_SECRET.\nDo not leak CHANNEL_TOPIC_SOURCE_SECRET in the receipt.",
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
			Number: 271,
			Title:  "GitClaw telegram thread chat-topic-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{271: {{
			ID: 27100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-topic-123",
				MessageID: "inbound-271",
				Author:    "telegram",
				Body:      "Original mirrored message.",
			}),
		}}},
		IssueLabels: map[int][]string{271: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel topic action", llm.Calls)
	}
	comments := github.CommentsByIssue[271]
	if len(comments) != 3 {
		t.Fatalf("comments = %d, want message + topic + receipt: %#v", len(comments), comments)
	}
	topic := comments[1].Body
	for _, want := range []string{"gitclaw:channel-topic", `channel="telegram"`, `thread_id="chat-topic-123"`, `topic_id="topic-271"`, "Visible topic CHANNEL_TOPIC_BODY_SECRET"} {
		if !strings.Contains(topic, want) {
			t.Fatalf("topic comment missing %q:\n%s", want, topic)
		}
	}
	receipt := comments[2].Body
	for _, want := range []string{
		"GitClaw Channel Topic Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels topic`",
		"channel_topic_status: `queued`",
		"target_issue: `#271`",
		"topic_comment_id: `9000`",
		"target_issue_created: `false`",
		"duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"target_issue_is_source: `true`",
		"topic_sha256_12:",
		"topic_source: `trailing-lines`",
		"raw_thread_id_included: `false`",
		"raw_topic_id_included: `false`",
		"raw_topic_included: `false`",
		"provider_delivery_performed: `false`",
		"github_issue_title_mutation_performed: `false`",
		"provider_topic_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"model_call_performed: `false`",
		"llm_e2e_required_after_channel_topic_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel topic receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOPIC_SOURCE_SECRET", "CHANNEL_TOPIC_BODY_SECRET", "Visible topic", "chat-topic-123", "topic-271"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel topic receipt leaked %q:\n%s", leaked, receipt)
		}
	}
	if !hasLabel(github.IssueLabels[271], "gitclaw:done") || hasLabel(github.IssueLabels[271], "gitclaw:running") || hasLabel(github.IssueLabels[271], "gitclaw:error") {
		t.Fatalf("unexpected source labels: %#v", github.IssueLabels[271])
	}
}

func TestRunChannelTopicDedupesAndOutboxDelivers(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 20,
			Title:  "GitClaw slack thread team-topic",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "slack",
				ThreadID: "team-topic",
			}),
			Labels: []string{cfg.ChannelLabel},
		}},
		CommentsByIssue: map[int][]Comment{20: nil},
		IssueLabels:     map[int][]string{20: []string{cfg.ChannelLabel}},
	}
	result, err := RunChannelTopic(context.Background(), cfg, github, ChannelTopicOptions{
		Repo:     "owner/repo",
		Channel:  "slack",
		ThreadID: "team-topic",
		TopicID:  "topic-1",
		Topic:    "Release triage room",
	})
	if err != nil {
		t.Fatalf("RunChannelTopic returned error: %v", err)
	}
	if result.IssueNumber != 20 || result.CommentID == 0 || result.Created || result.Duplicate || result.TopicIDHash == "" || result.TopicHash == "" {
		t.Fatalf("unexpected topic result: %#v", result)
	}
	comment := github.CommentsByIssue[20][0]
	if !HasChannelTopicMarker(comment.Body) || !strings.Contains(comment.Body, `topic_id="topic-1"`) || !strings.Contains(comment.Body, "Release triage room") {
		t.Fatalf("topic comment missing marker/body:\n%s", comment.Body)
	}

	duplicate, err := RunChannelTopic(context.Background(), cfg, github, ChannelTopicOptions{
		Repo:     "owner/repo",
		Channel:  "slack",
		ThreadID: "team-topic",
		TopicID:  "topic-1",
		Topic:    "A duplicate topic should not post.",
	})
	if err != nil {
		t.Fatalf("duplicate RunChannelTopic returned error: %v", err)
	}
	if !duplicate.Duplicate || duplicate.CommentID != 0 || len(github.CommentsByIssue[20]) != 1 {
		t.Fatalf("duplicate topic not suppressed: result=%#v comments=%#v", duplicate, github.CommentsByIssue[20])
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
	if outbox.SourceTopicComments != 1 || outbox.SourceDeliverableComments != 1 || outbox.PendingMessages != 1 || len(outbox.Messages) != 1 {
		t.Fatalf("unexpected topic outbox result: %#v", outbox)
	}
	if outbox.Messages[0].Kind != "channel-topic" {
		t.Fatalf("outbox kind = %q, want channel-topic", outbox.Messages[0].Kind)
	}

	delivery, err := RunChannelDelivery(context.Background(), cfg, github, ChannelDeliveryOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		AccountID:         "slack-account",
		IssueNumber:       20,
		CommentID:         comment.ID,
		ExternalMessageID: "topic-delivered-1",
	})
	if err != nil {
		t.Fatalf("RunChannelDelivery returned error: %v", err)
	}
	if !delivery.Delivered || delivery.Duplicate || delivery.StateIssueNumber == 0 {
		t.Fatalf("unexpected delivery result: %#v", delivery)
	}
}

func TestBuildChannelTopicActionRequestParsesRouteAndTrailingTopic(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 8,
			Title:  "@gitclaw /channels thread-title --route team-demo --topic-id Topic.One",
			Body:   "Release room",
		},
	}
	req, err := BuildChannelTopicActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelTopicActionRequest returned error: %v", err)
	}
	if req.Subcommand != "thread-title" || req.Options.Route != "team-demo" || req.Options.TopicID != "topic-one" || req.Options.Topic != "Release room" {
		t.Fatalf("unexpected topic parsing: %#v", req)
	}
	if req.TopicSource != "trailing-lines" || req.TopicSHA == "" || req.RequestedRouteHash == "" || req.RequestedTopicIDSHA == "" {
		t.Fatalf("expected topic metadata: %#v", req)
	}
}
