package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelPollCreatesIssueAndInvitesRoutesWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: e2e-slack-route
    channel: slack
    thread_id_template: slack-poll-{route}-{message_id}
    author: gitclaw:test
  - name: e2e-telegram-route
    channel: telegram
    thread_id_template: telegram-poll-{route}-{message_id}
    author: gitclaw:test
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 280,
			"title": "Plan the channel poll feature",
			"body": "@gitclaw /channels poll e2e-slack-route,e2e-telegram-route --poll-id poll-1 --message-id poll-msg-1\n\nQuestion: Which channel poll option should ship with CHANNEL_POLL_QUESTION_SECRET?\nOptions:\n- Ship it\n- Hold it",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{280: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel poll action", llm.Calls)
	}
	if len(github.Issues) != 3 {
		t.Fatalf("created issues = %d, want poll plus two channel issues: %#v", len(github.Issues), github.Issues)
	}
	poll := github.Issues[0]
	if !HasChannelPollMarker(poll.Body) || !strings.Contains(poll.Body, `poll_id="poll-1"`) {
		t.Fatalf("poll issue missing poll marker:\n%s", poll.Body)
	}
	for _, want := range []string{
		"GitClaw channel poll",
		"Which channel poll option should ship with CHANNEL_POLL_QUESTION_SECRET?",
		"1. Ship it",
		"2. Hold it",
		"source_issue: `#280`",
		"raw_route_names_included: `false`",
	} {
		if !strings.Contains(poll.Body, want) {
			t.Fatalf("poll issue missing %q:\n%s", want, poll.Body)
		}
	}
	if !hasLabel(github.IssueLabels[poll.Number], "gitclaw") {
		t.Fatalf("poll issue missing gitclaw trigger label: %#v", github.IssueLabels[poll.Number])
	}

	for _, issue := range github.Issues[1:] {
		if !HasChannelThreadMarker(issue.Body) {
			t.Fatalf("poll target issue missing channel thread marker: %#v", issue)
		}
		comments := github.CommentsByIssue[issue.Number]
		if len(comments) != 1 {
			t.Fatalf("target issue %d comments = %d, want one outbound: %#v", issue.Number, len(comments), comments)
		}
		for _, want := range []string{
			"gitclaw:channel-outbound",
			`message_id="poll-msg-1"`,
			"GitClaw channel poll",
			"Poll: #100",
			"https://github.com/owner/repo/issues/100",
			"Which channel poll option should ship with CHANNEL_POLL_QUESTION_SECRET?",
			"1. Ship it",
			"2. Hold it",
		} {
			if !strings.Contains(comments[0].Body, want) {
				t.Fatalf("target issue %d missing outbound poll invite %q:\n%s", issue.Number, want, comments[0].Body)
			}
		}
	}

	sourceComments := github.CommentsByIssue[280]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want poll receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Channel Poll Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels poll`",
		"channel_poll_status: `queued`",
		"poll_issue: `#100`",
		"poll_issue_created: `true`",
		"poll_id_auto: `false`",
		"message_id_auto: `false`",
		"poll_options: `2`",
		"poll_routes: `2`",
		"poll_invites_queued: `2`",
		"poll_invite_duplicates: `0`",
		"target_issues_created: `2`",
		"raw_route_names_included: `false`",
		"raw_poll_id_included: `false`",
		"raw_question_included: `false`",
		"raw_options_included: `false`",
		"raw_outbound_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_poll_action_change: `true`",
		"channel=`slack`",
		"channel=`telegram`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("poll receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_POLL_QUESTION_SECRET", "Which channel poll option", "Ship it", "Hold it", "poll-1", "poll-msg-1", "e2e-slack-route", "e2e-telegram-route"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("poll receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 280,
			"title": "Plan the channel poll feature",
			"body": "@gitclaw /channels poll e2e-slack-route,e2e-telegram-route --poll-id poll-1 --message-id poll-msg-1\n\nQuestion: Which channel poll option should ship?\nOptions:\n- Ship it\n- Hold it",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 28001,
			"body": "@gitclaw /channels poll e2e-slack-route,e2e-telegram-route --poll-id poll-1 --message-id poll-msg-1\n\nQuestion: Do not leak duplicate secret CHANNEL_POLL_DUPLICATE_SECRET?\nOptions:\n- Again\n- Later",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, cfg, github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 3 {
		t.Fatalf("duplicate poll created more issues: %#v", github.Issues)
	}
	for _, issue := range github.Issues[1:] {
		if got := len(github.CommentsByIssue[issue.Number]); got != 1 {
			t.Fatalf("duplicate poll posted another outbound comment on issue %d: %d", issue.Number, got)
		}
	}
	duplicateReceipt := github.CommentsByIssue[280][1].Body
	for _, want := range []string{
		"channel_poll_status: `duplicate`",
		"poll_issue_created: `false`",
		"poll_invites_queued: `0`",
		"poll_invite_duplicates: `2`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate poll receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_POLL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate poll receipt leaked body:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelPollActionRequestParsesQuestionAndOptions(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 28,
			Title:  "Design poll",
		},
		Comment: &Comment{
			ID: 2801,
			Body: `@gitclaw /channels vote --route Team-Demo --poll-id Design.Poll --message-id poll-28
Question: Which design should we use?
Options:
- Compact
- Spacious`,
		},
	}
	req, err := BuildChannelPollActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPollActionRequest returned error: %v", err)
	}
	if req.Subcommand != "vote" || req.Options.Routes[0] != "team-demo" || req.Options.PollID != "design-poll" || req.Options.MessageID != "poll-28" {
		t.Fatalf("unexpected poll target parsing: %#v", req)
	}
	if req.Options.Question != "Which design should we use?" {
		t.Fatalf("unexpected poll question: %#v", req)
	}
	if len(req.Options.Options) != 2 || req.Options.Options[0] != "Compact" || req.Options.Options[1] != "Spacious" {
		t.Fatalf("unexpected poll options: %#v", req.Options.Options)
	}
	if req.AutoPollID || req.AutoMessageID || req.QuestionSHA == "" || req.OptionsSHA == "" || req.RoutesSHA == "" {
		t.Fatalf("expected poll hashes and explicit ids: %#v", req)
	}
}
