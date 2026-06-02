package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelPollVoteRecordsVoteAndQueuesAckWithoutLLM(t *testing.T) {
	channelBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "slack",
		ThreadID: "chat-poll-vote-123",
	})
	pollBody := RenderChannelPollIssueBody(ChannelPollOptions{
		Repo:              "owner/repo",
		PollID:            "poll-1",
		SourceIssueNumber: 284,
		Question:          "Which tiny thing should ship?",
		Options:           []string{"Ship the tiny poll", "Keep the huddle only"},
		Routes:            []string{"e2e-slack-route"},
		MessageID:         "poll-msg-1",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 101,
			"title": "GitClaw slack thread chat-poll-vote-123",
			"body": "<!-- gitclaw:channel-thread channel=\"slack\" thread_id=\"chat-poll-vote-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 10101,
			"body": "@gitclaw /channels poll-vote --poll-id poll-1 --message-id vote-msg-1 --notify-message-id vote-ack-1 --choice 1\nVoter: Anand Channel\nNote:\nPlease keep this vote note: CHANNEL_POLL_VOTE_NOTE_SECRET.",
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
			{Number: 100, Title: "GitClaw channel poll: Which tiny thing should ship?", Body: pollBody, Labels: []string{"gitclaw"}, AuthorAssociation: "MEMBER"},
			{Number: 101, Title: "GitClaw slack thread chat-poll-vote-123", Body: channelBody, Labels: []string{"gitclaw:channel"}, AuthorAssociation: "MEMBER"},
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
		t.Fatalf("LLM called %d times for channel poll vote action", llm.Calls)
	}
	pollComments := github.CommentsByIssue[100]
	if len(pollComments) != 1 {
		t.Fatalf("poll comments = %d, want one vote record: %#v", len(pollComments), pollComments)
	}
	voteRecord := pollComments[0].Body
	for _, want := range []string{
		"gitclaw:channel-poll-vote",
		`poll_id="poll-1"`,
		`vote_id="vote-msg-1"`,
		"GitClaw channel poll vote",
		"- choice: Ship the tiny poll",
		"- choice_index: 1",
		"- source_channel: slack",
		"- source_issue: #101",
		"Anand Channel",
		"Please keep this vote note: CHANNEL_POLL_VOTE_NOTE_SECRET.",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
	} {
		if !strings.Contains(voteRecord, want) {
			t.Fatalf("poll vote record missing %q:\n%s", want, voteRecord)
		}
	}

	channelComments := github.CommentsByIssue[101]
	if len(channelComments) != 2 {
		t.Fatalf("channel comments = %d, want ack plus receipt: %#v", len(channelComments), channelComments)
	}
	ack := channelComments[0].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`message_id="vote-ack-1"`,
		"GitClaw poll vote recorded",
		"Poll: #100",
		"Choice: Ship the tiny poll",
		"Choice index: 1",
		"Participant: Anand Channel",
	} {
		if !strings.Contains(ack, want) {
			t.Fatalf("poll vote ack missing %q:\n%s", want, ack)
		}
	}
	if strings.Contains(ack, "CHANNEL_POLL_VOTE_NOTE_SECRET") {
		t.Fatalf("poll vote ack leaked private note:\n%s", ack)
	}
	receipt := channelComments[1].Body
	for _, want := range []string{
		"GitClaw Channel Poll Vote Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels poll-vote`",
		"channel_poll_vote_status: `recorded`",
		"poll_issue: `#100`",
		"vote_comment_id: `9000`",
		"vote_recorded: `true`",
		"vote_duplicate_suppressed: `false`",
		"notification_target_issue: `#101`",
		"notification_comment_id: `9001`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"channel: `slack`",
		"vote_id_auto: `false`",
		"notify_message_id_auto: `false`",
		"choice_resolved_from_poll_options: `true`",
		"choice_index: `1`",
		"target_from_current_channel_issue: `true`",
		"raw_poll_id_included: `false`",
		"raw_vote_id_included: `false`",
		"raw_choice_included: `false`",
		"raw_voter_included: `false`",
		"raw_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_poll_vote_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("poll vote receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"poll-1", "vote-msg-1", "vote-ack-1", "Ship the tiny poll", "Anand Channel", "CHANNEL_POLL_VOTE_NOTE_SECRET", "chat-poll-vote-123"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("poll vote receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 101,
			"title": "GitClaw slack thread chat-poll-vote-123",
			"body": "<!-- gitclaw:channel-thread channel=\"slack\" thread_id=\"chat-poll-vote-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 10102,
			"body": "@gitclaw /channels poll-vote --poll-id poll-1 --message-id vote-msg-1 --notify-message-id vote-ack-1 --choice 1\nNote: duplicate should not leak CHANNEL_POLL_VOTE_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate poll vote posted another vote comment: %#v", github.CommentsByIssue[100])
	}
	if got := channelOutboundCommentCount(github.CommentsByIssue[101]); got != 1 {
		t.Fatalf("duplicate poll vote queued another ack, got %d", got)
	}
	duplicateReceipt := github.CommentsByIssue[101][2].Body
	for _, want := range []string{
		"channel_poll_vote_status: `duplicate`",
		"vote_recorded: `false`",
		"vote_duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate poll vote receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_POLL_VOTE_DUPLICATE_SECRET", "vote-msg-1", "vote-ack-1", "Ship the tiny poll"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate poll vote receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelPollVoteActionRequestParsesTrailingVote(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 45,
			Title:  "GitClaw telegram thread",
			Body:   RenderChannelThreadBody(ChannelIngestOptions{Channel: "telegram", ThreadID: "thread-45"}),
		},
		Comment: &Comment{
			ID: 4501,
			Body: `@gitclaw /channels cast-vote --poll-id Team.Poll --message-id provider-vote-1 --notify-message-id ack-1
Choice: 2
Voter: Product Lead
Note:
Prefer the second option.`,
		},
	}
	req, err := BuildChannelPollVoteActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPollVoteActionRequest returned error: %v", err)
	}
	if req.Subcommand != "cast-vote" || req.Options.PollID != "team-poll" || req.Options.VoteID != "provider-vote-1" {
		t.Fatalf("unexpected poll vote identity parsing: %#v", req)
	}
	if req.Options.Choice != "2" || req.Options.Voter != "Product Lead" || req.Options.Note != "Prefer the second option." {
		t.Fatalf("unexpected poll vote details: %#v", req.Options)
	}
	if req.Options.Channel != "telegram" || req.Options.ThreadID != "thread-45" || !req.TargetFromIssue {
		t.Fatalf("expected poll vote target from channel issue: %#v", req)
	}
	if req.AutoVoteID || req.AutoNotifyMessageID || req.ChoiceSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit ids and vote hashes: %#v", req)
	}
}
