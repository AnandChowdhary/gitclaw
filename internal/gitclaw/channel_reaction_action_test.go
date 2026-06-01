package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelReactionQueuesCurrentThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-reaction-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 260,
			"title": "GitClaw telegram thread chat-reaction-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-reaction-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26001,
			"body": "@gitclaw /channels react --message-id inbound-260 --reaction eyes\nDo not leak CHANNEL_REACTION_SOURCE_SECRET.",
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
			Number: 260,
			Title:  "GitClaw telegram thread chat-reaction-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{260: {{
			ID: 26000,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-reaction-123",
				MessageID: "inbound-260",
				Author:    "telegram",
				Body:      "Original mirrored message.",
			}),
		}}},
		IssueLabels: map[int][]string{260: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel reaction action", llm.Calls)
	}
	comments := github.CommentsByIssue[260]
	if len(comments) != 3 {
		t.Fatalf("comments = %d, want message + reaction + receipt: %#v", len(comments), comments)
	}
	reaction := comments[1].Body
	for _, want := range []string{"gitclaw:channel-reaction", `channel="telegram"`, `thread_id="chat-reaction-123"`, `message_id="inbound-260"`, `reaction="eyes"`, "GitClaw channel reaction: eyes"} {
		if !strings.Contains(reaction, want) {
			t.Fatalf("reaction comment missing %q:\n%s", want, reaction)
		}
	}
	receipt := comments[2].Body
	for _, want := range []string{
		"GitClaw Channel Reaction Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels react`",
		"channel_reaction_status: `queued`",
		"target_issue: `#260`",
		"reaction_comment_id: `9000`",
		"target_issue_created: `false`",
		"duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"target_issue_is_source: `true`",
		"raw_target_message_id_included: `false`",
		"raw_reaction_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_reaction_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel reaction receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_REACTION_SOURCE_SECRET", "chat-reaction-123", "inbound-260", "eyes"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel reaction receipt leaked %q:\n%s", leaked, receipt)
		}
	}
	if !hasLabel(github.IssueLabels[260], "gitclaw:done") || hasLabel(github.IssueLabels[260], "gitclaw:running") || hasLabel(github.IssueLabels[260], "gitclaw:error") {
		t.Fatalf("unexpected source labels: %#v", github.IssueLabels[260])
	}
}

func TestRunChannelReactionDedupesAndOutboxDelivers(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 17,
			Title:  "GitClaw slack thread team-react",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "slack",
				ThreadID: "team-react",
			}),
			Labels: []string{cfg.ChannelLabel},
		}},
		CommentsByIssue: map[int][]Comment{17: nil},
		IssueLabels:     map[int][]string{17: []string{cfg.ChannelLabel}},
	}
	result, err := RunChannelReaction(context.Background(), cfg, github, ChannelReactionOptions{
		Repo:      "owner/repo",
		Channel:   "slack",
		ThreadID:  "team-react",
		MessageID: "source-msg-1",
		Reaction:  ":Thumbs-Up:",
	})
	if err != nil {
		t.Fatalf("RunChannelReaction returned error: %v", err)
	}
	if result.IssueNumber != 17 || result.CommentID == 0 || result.Created || result.Duplicate || result.ReactionHash == "" {
		t.Fatalf("unexpected reaction result: %#v", result)
	}
	comment := github.CommentsByIssue[17][0]
	if !HasChannelReactionMarker(comment.Body) || !strings.Contains(comment.Body, `reaction="thumbs-up"`) {
		t.Fatalf("reaction comment missing marker/normalized reaction:\n%s", comment.Body)
	}

	duplicate, err := RunChannelReaction(context.Background(), cfg, github, ChannelReactionOptions{
		Repo:      "owner/repo",
		Channel:   "slack",
		ThreadID:  "team-react",
		MessageID: "source-msg-1",
		Reaction:  "thumbs-up",
	})
	if err != nil {
		t.Fatalf("duplicate RunChannelReaction returned error: %v", err)
	}
	if !duplicate.Duplicate || duplicate.CommentID != 0 || len(github.CommentsByIssue[17]) != 1 {
		t.Fatalf("duplicate reaction not suppressed: result=%#v comments=%#v", duplicate, github.CommentsByIssue[17])
	}

	outbox, err := RunChannelOutbox(context.Background(), cfg, github, ChannelOutboxOptions{
		Repo:        "owner/repo",
		Channel:     "slack",
		AccountID:   "slack-account",
		IssueNumber: 17,
	})
	if err != nil {
		t.Fatalf("RunChannelOutbox returned error: %v", err)
	}
	if outbox.SourceReactionComments != 1 || outbox.SourceDeliverableComments != 1 || outbox.PendingMessages != 1 || len(outbox.Messages) != 1 {
		t.Fatalf("unexpected reaction outbox result: %#v", outbox)
	}
	if outbox.Messages[0].Kind != "channel-reaction" {
		t.Fatalf("outbox kind = %q, want channel-reaction", outbox.Messages[0].Kind)
	}

	delivery, err := RunChannelDelivery(context.Background(), cfg, github, ChannelDeliveryOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		AccountID:         "slack-account",
		IssueNumber:       17,
		CommentID:         comment.ID,
		ExternalMessageID: "reaction-delivered-1",
	})
	if err != nil {
		t.Fatalf("RunChannelDelivery returned error: %v", err)
	}
	if !delivery.Delivered || delivery.Duplicate || delivery.StateIssueNumber == 0 {
		t.Fatalf("unexpected delivery result: %#v", delivery)
	}
}

func TestBuildChannelReactionActionRequestParsesRouteAndReaction(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 8,
			Title:  "@gitclaw /channels ack team-demo --message-id source-1 --emoji :Eyes:",
			Body:   "",
		},
	}
	req, err := BuildChannelReactionActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelReactionActionRequest returned error: %v", err)
	}
	if req.Subcommand != "ack" || req.Options.Route != "team-demo" || req.Options.MessageID != "source-1" || req.Options.Reaction != "eyes" {
		t.Fatalf("unexpected reaction parsing: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.RequestedMsgHash == "" || req.ReactionHash == "" {
		t.Fatalf("expected route/message/reaction hashes: %#v", req)
	}
}

func TestBuildChannelReactionActionRequestDefaultsPinReaction(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 18,
			Title:  "GitClaw slack thread team-pin",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "slack",
				ThreadID: "team-pin",
			}),
		},
		Comment: &Comment{
			ID:   1801,
			Body: "@gitclaw /channels pin --message-id source-18\nDo not leak CHANNEL_PIN_PARSE_TOKEN",
		},
	}
	req, err := BuildChannelReactionActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelReactionActionRequest returned error: %v", err)
	}
	if req.Subcommand != "pin" || req.Options.Channel != "slack" || req.Options.ThreadID != "team-pin" || req.Options.MessageID != "source-18" || req.Options.Reaction != "pushpin" {
		t.Fatalf("unexpected pin reaction parsing: %#v", req)
	}
	if !req.TargetFromIssue || req.RequestedMsgHash == "" || req.ReactionHash == "" {
		t.Fatalf("expected current issue target and hashes: %#v", req)
	}
}
