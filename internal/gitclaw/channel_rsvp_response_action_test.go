package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelRsvpResponseRecordsResponseAndQueuesAckWithoutLLM(t *testing.T) {
	channelBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-rsvp-response-123",
	})
	rsvpBody := RenderChannelRsvpIssueBody(ChannelRsvpOptions{
		Repo:              "owner/repo",
		RsvpID:            "rsvp-1",
		SourceIssueNumber: 284,
		Title:             "Tiny RSVP",
		Routes:            []string{"e2e-telegram-route"},
		MessageID:         "rsvp-msg-1",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 101,
			"title": "GitClaw telegram thread chat-rsvp-response-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-rsvp-response-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 10101,
			"body": "@gitclaw /channels rsvp-response --rsvp-id rsvp-1 --message-id resp-msg-1 --notify-message-id ack-msg-1 --response yes\nResponder: Anand Channel\nNote:\nPlease keep this note: CHANNEL_RSVP_RESPONSE_NOTE_SECRET.",
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
			{Number: 100, Title: "GitClaw channel RSVP: Tiny RSVP", Body: rsvpBody, Labels: []string{"gitclaw"}, AuthorAssociation: "MEMBER"},
			{Number: 101, Title: "GitClaw telegram thread chat-rsvp-response-123", Body: channelBody, Labels: []string{"gitclaw:channel"}, AuthorAssociation: "MEMBER"},
		},
		CommentsByIssue: map[int][]Comment{100: nil, 101: nil},
		IssueLabels: map[int][]string{
			100: []string{"gitclaw"},
			101: []string{"gitclaw:channel"},
		},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel RSVP response action", llm.Calls)
	}
	rsvpComments := github.CommentsByIssue[100]
	if len(rsvpComments) != 1 {
		t.Fatalf("RSVP comments = %d, want one response record: %#v", len(rsvpComments), rsvpComments)
	}
	responseRecord := rsvpComments[0].Body
	for _, want := range []string{
		"gitclaw:channel-rsvp-response",
		`rsvp_id="rsvp-1"`,
		`response_id="resp-msg-1"`,
		"GitClaw channel RSVP response",
		"- response: yes",
		"- source_channel: telegram",
		"- source_issue: #101",
		"Anand Channel",
		"Please keep this note: CHANNEL_RSVP_RESPONSE_NOTE_SECRET.",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
	} {
		if !strings.Contains(responseRecord, want) {
			t.Fatalf("RSVP response record missing %q:\n%s", want, responseRecord)
		}
	}

	channelComments := github.CommentsByIssue[101]
	if len(channelComments) != 2 {
		t.Fatalf("channel comments = %d, want ack plus receipt: %#v", len(channelComments), channelComments)
	}
	ack := channelComments[0].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`message_id="ack-msg-1"`,
		"GitClaw RSVP response recorded",
		"RSVP: #100",
		"Response: yes",
		"Participant: Anand Channel",
	} {
		if !strings.Contains(ack, want) {
			t.Fatalf("RSVP response ack missing %q:\n%s", want, ack)
		}
	}
	if strings.Contains(ack, "CHANNEL_RSVP_RESPONSE_NOTE_SECRET") {
		t.Fatalf("RSVP response ack leaked private note:\n%s", ack)
	}
	receipt := channelComments[1].Body
	for _, want := range []string{
		"GitClaw Channel RSVP Response Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rsvp-response`",
		"channel_rsvp_response_status: `recorded`",
		"rsvp_issue: `#100`",
		"response_comment_id: `9000`",
		"response_recorded: `true`",
		"response_duplicate_suppressed: `false`",
		"notification_target_issue: `#101`",
		"notification_comment_id: `9001`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"channel: `telegram`",
		"response_id_auto: `false`",
		"notify_message_id_auto: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_rsvp_id_included: `false`",
		"raw_response_id_included: `false`",
		"raw_response_included: `false`",
		"raw_responder_included: `false`",
		"raw_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_rsvp_response_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("RSVP response receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"rsvp-1", "resp-msg-1", "ack-msg-1", "yes", "Anand Channel", "CHANNEL_RSVP_RESPONSE_NOTE_SECRET", "chat-rsvp-response-123"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("RSVP response receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 101,
			"title": "GitClaw telegram thread chat-rsvp-response-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-rsvp-response-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 10102,
			"body": "@gitclaw /channels rsvp-response --rsvp-id rsvp-1 --message-id resp-msg-1 --notify-message-id ack-msg-1 --response yes\nNote: duplicate should not leak CHANNEL_RSVP_RESPONSE_DUPLICATE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.CommentsByIssue[100]) != 1 {
		t.Fatalf("duplicate RSVP response posted another response comment: %#v", github.CommentsByIssue[100])
	}
	if got := channelOutboundCommentCount(github.CommentsByIssue[101]); got != 1 {
		t.Fatalf("duplicate RSVP response queued another ack, got %d", got)
	}
	duplicateReceipt := github.CommentsByIssue[101][2].Body
	for _, want := range []string{
		"channel_rsvp_response_status: `duplicate`",
		"response_recorded: `false`",
		"response_duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate RSVP response receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_RSVP_RESPONSE_DUPLICATE_SECRET", "resp-msg-1", "ack-msg-1", "yes"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate RSVP response receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelRsvpResponseActionRequestParsesTrailingResponse(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 45,
			Title:  "GitClaw slack thread",
			Body:   RenderChannelThreadBody(ChannelIngestOptions{Channel: "slack", ThreadID: "thread-45"}),
		},
		Comment: &Comment{
			ID: 4501,
			Body: `@gitclaw /channels respond --rsvp-id Team.Demo --message-id provider-response-1 --notify-message-id ack-1
Response: tentative
Responder: Product Lead
Note:
Can join after standup.`,
		},
	}
	req, err := BuildChannelRsvpResponseActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRsvpResponseActionRequest returned error: %v", err)
	}
	if req.Subcommand != "respond" || req.Options.RsvpID != "team-demo" || req.Options.ResponseID != "provider-response-1" {
		t.Fatalf("unexpected RSVP response identity parsing: %#v", req)
	}
	if req.Options.Response != "maybe" || req.Options.Responder != "Product Lead" || req.Options.Note != "Can join after standup." {
		t.Fatalf("unexpected RSVP response details: %#v", req.Options)
	}
	if req.Options.Channel != "slack" || req.Options.ThreadID != "thread-45" || !req.TargetFromIssue {
		t.Fatalf("expected RSVP response target from channel issue: %#v", req)
	}
	if req.AutoResponseID || req.AutoNotifyMessageID || req.ResponseSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit ids and response hashes: %#v", req)
	}
}

func channelOutboundCommentCount(comments []Comment) int {
	count := 0
	for _, comment := range comments {
		if HasChannelOutboundMarker(comment.Body) {
			count++
		}
	}
	return count
}
