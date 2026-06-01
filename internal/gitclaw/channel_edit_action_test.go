package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelEditQueuesCurrentThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-edit-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 271,
			"title": "GitClaw telegram thread chat-edit-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-edit-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 27101,
			"body": "@gitclaw /channels edit --message-id outbound-271 --edit-id edit-271\nUpdated channel message with CHANNEL_EDIT_BODY_SECRET.\nDo not leak CHANNEL_EDIT_SOURCE_SECRET in the receipt.",
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
			Title:  "GitClaw telegram thread chat-edit-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{271: {{
			ID: 27100,
			Body: RenderChannelOutboundComment(ChannelSendOptions{
				Channel:   "telegram",
				ThreadID:  "chat-edit-123",
				MessageID: "outbound-271",
				Author:    "gitclaw",
				Body:      "Original outbound message.",
			}),
		}}},
		IssueLabels: map[int][]string{271: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel edit action", llm.Calls)
	}
	comments := github.CommentsByIssue[271]
	if len(comments) != 3 {
		t.Fatalf("comments = %d, want outbound + edit + receipt: %#v", len(comments), comments)
	}
	edit := comments[1].Body
	for _, want := range []string{"gitclaw:channel-edit", `channel="telegram"`, `thread_id="chat-edit-123"`, `target_message_id="outbound-271"`, `edit_id="edit-271"`, "CHANNEL_EDIT_BODY_SECRET"} {
		if !strings.Contains(edit, want) {
			t.Fatalf("edit comment missing %q:\n%s", want, edit)
		}
	}
	receipt := comments[2].Body
	for _, want := range []string{
		"GitClaw Channel Edit Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels edit`",
		"channel_edit_status: `queued`",
		"target_issue: `#271`",
		"edit_comment_id: `9000`",
		"target_issue_created: `false`",
		"duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"target_issue_is_source: `true`",
		"edit_body_sha256_12:",
		"raw_target_message_id_included: `false`",
		"raw_edit_id_included: `false`",
		"raw_edit_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_edit_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel edit receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_EDIT_SOURCE_SECRET", "CHANNEL_EDIT_BODY_SECRET", "chat-edit-123", "outbound-271", "edit-271"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel edit receipt leaked %q:\n%s", leaked, receipt)
		}
	}
	if !hasLabel(github.IssueLabels[271], "gitclaw:done") || hasLabel(github.IssueLabels[271], "gitclaw:running") || hasLabel(github.IssueLabels[271], "gitclaw:error") {
		t.Fatalf("unexpected source labels: %#v", github.IssueLabels[271])
	}
}

func TestRunChannelEditDedupesAndOutboxDelivers(t *testing.T) {
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 20,
			Title:  "GitClaw slack thread team-edit",
			Body: RenderChannelThreadBody(ChannelIngestOptions{
				Channel:  "slack",
				ThreadID: "team-edit",
			}),
			Labels: []string{cfg.ChannelLabel},
		}},
		CommentsByIssue: map[int][]Comment{20: nil},
		IssueLabels:     map[int][]string{20: []string{cfg.ChannelLabel}},
	}
	result, err := RunChannelEdit(context.Background(), cfg, github, ChannelEditOptions{
		Repo:            "owner/repo",
		Channel:         "slack",
		ThreadID:        "team-edit",
		TargetMessageID: "provider-msg-1",
		EditID:          "Edit.One",
		Body:            "Updated provider message.",
	})
	if err != nil {
		t.Fatalf("RunChannelEdit returned error: %v", err)
	}
	if result.IssueNumber != 20 || result.CommentID == 0 || result.Created || result.Duplicate || result.EditIDHash == "" {
		t.Fatalf("unexpected edit result: %#v", result)
	}
	comment := github.CommentsByIssue[20][0]
	if !HasChannelEditMarker(comment.Body) || !strings.Contains(comment.Body, `edit_id="edit-one"`) {
		t.Fatalf("edit comment missing marker/normalized id:\n%s", comment.Body)
	}

	duplicate, err := RunChannelEdit(context.Background(), cfg, github, ChannelEditOptions{
		Repo:            "owner/repo",
		Channel:         "slack",
		ThreadID:        "team-edit",
		TargetMessageID: "provider-msg-1",
		EditID:          "edit-one",
		Body:            "A duplicate body should not post.",
	})
	if err != nil {
		t.Fatalf("duplicate RunChannelEdit returned error: %v", err)
	}
	if !duplicate.Duplicate || duplicate.CommentID != 0 || len(github.CommentsByIssue[20]) != 1 {
		t.Fatalf("duplicate edit not suppressed: result=%#v comments=%#v", duplicate, github.CommentsByIssue[20])
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
	if outbox.SourceEditComments != 1 || outbox.SourceDeliverableComments != 1 || outbox.PendingMessages != 1 || len(outbox.Messages) != 1 {
		t.Fatalf("unexpected edit outbox result: %#v", outbox)
	}
	if outbox.Messages[0].Kind != "channel-edit" {
		t.Fatalf("outbox kind = %q, want channel-edit", outbox.Messages[0].Kind)
	}

	delivery, err := RunChannelDelivery(context.Background(), cfg, github, ChannelDeliveryOptions{
		Repo:              "owner/repo",
		Channel:           "slack",
		AccountID:         "slack-account",
		IssueNumber:       20,
		CommentID:         comment.ID,
		ExternalMessageID: "edit-applied-1",
	})
	if err != nil {
		t.Fatalf("RunChannelDelivery returned error: %v", err)
	}
	if !delivery.Delivered || delivery.Duplicate || delivery.StateIssueNumber == 0 {
		t.Fatalf("unexpected delivery result: %#v", delivery)
	}
}

func TestBuildChannelEditActionRequestParsesRouteAndBody(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 8,
			Title:  "@gitclaw /channels update team-demo --message-id source-1 --edit-id Edit.One",
			Body:   "Replacement body line.",
		},
	}
	req, err := BuildChannelEditActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelEditActionRequest returned error: %v", err)
	}
	if req.Subcommand != "update" || req.Options.Route != "team-demo" || req.Options.TargetMessageID != "source-1" || req.Options.EditID != "edit-one" {
		t.Fatalf("unexpected edit parsing: %#v", req)
	}
	if req.BodySource != "trailing-lines" || req.Options.Body != "Replacement body line." {
		t.Fatalf("unexpected edit body parsing: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.RequestedTargetMsgHash == "" || req.RequestedEditIDHash == "" {
		t.Fatalf("expected route/message/edit hashes: %#v", req)
	}
}
