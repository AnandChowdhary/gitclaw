package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelHuddleCreatesIssueAndInvitesRoutesWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: e2e-slack-route
    channel: slack
    thread_id_template: slack-huddle-{route}-{message_id}
    author: gitclaw:test
  - name: e2e-telegram-route
    channel: telegram
    thread_id_template: telegram-huddle-{route}-{message_id}
    author: gitclaw:test
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 270,
			"title": "Plan the channel huddle feature",
			"body": "@gitclaw /channels huddle e2e-slack-route,e2e-telegram-route --huddle-id huddle-1 --message-id huddle-msg-1\n\nTopic: Channel huddle launch\nAgenda:\nVisible huddle agenda with CHANNEL_HUDDLE_AGENDA_SECRET",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{270: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel huddle action", llm.Calls)
	}
	if len(github.Issues) != 3 {
		t.Fatalf("created issues = %d, want huddle plus two channel issues: %#v", len(github.Issues), github.Issues)
	}
	huddle := github.Issues[0]
	if !HasChannelHuddleMarker(huddle.Body) || !strings.Contains(huddle.Body, `huddle_id="huddle-1"`) {
		t.Fatalf("huddle issue missing huddle marker:\n%s", huddle.Body)
	}
	for _, want := range []string{
		"GitClaw channel huddle",
		"Channel huddle launch",
		"Visible huddle agenda with CHANNEL_HUDDLE_AGENDA_SECRET",
		"source_issue: `#270`",
		"raw_route_names_included: `false`",
	} {
		if !strings.Contains(huddle.Body, want) {
			t.Fatalf("huddle issue missing %q:\n%s", want, huddle.Body)
		}
	}
	if !hasLabel(github.IssueLabels[huddle.Number], "gitclaw") {
		t.Fatalf("huddle issue missing gitclaw trigger label: %#v", github.IssueLabels[huddle.Number])
	}

	for _, issue := range github.Issues[1:] {
		if !HasChannelThreadMarker(issue.Body) {
			t.Fatalf("huddle target issue missing channel thread marker: %#v", issue)
		}
		comments := github.CommentsByIssue[issue.Number]
		if len(comments) != 1 {
			t.Fatalf("target issue %d comments = %d, want one outbound: %#v", issue.Number, len(comments), comments)
		}
		for _, want := range []string{
			"gitclaw:channel-outbound",
			`message_id="huddle-msg-1"`,
			"GitClaw channel huddle",
			"Huddle: #100",
			"https://github.com/owner/repo/issues/100",
			"Channel huddle launch",
			"CHANNEL_HUDDLE_AGENDA_SECRET",
		} {
			if !strings.Contains(comments[0].Body, want) {
				t.Fatalf("target issue %d missing outbound huddle invite %q:\n%s", issue.Number, want, comments[0].Body)
			}
		}
	}

	sourceComments := github.CommentsByIssue[270]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want huddle receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Channel Huddle Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels huddle`",
		"channel_huddle_status: `queued`",
		"huddle_issue: `#100`",
		"huddle_issue_created: `true`",
		"huddle_id_auto: `false`",
		"message_id_auto: `false`",
		"huddle_routes: `2`",
		"huddle_invites_queued: `2`",
		"huddle_invite_duplicates: `0`",
		"target_issues_created: `2`",
		"raw_route_names_included: `false`",
		"raw_huddle_id_included: `false`",
		"raw_topic_included: `false`",
		"raw_agenda_included: `false`",
		"raw_outbound_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_huddle_action_change: `true`",
		"channel=`slack`",
		"channel=`telegram`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("huddle receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_HUDDLE_AGENDA_SECRET", "Channel huddle launch", "huddle-1", "huddle-msg-1", "e2e-slack-route", "e2e-telegram-route"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("huddle receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 270,
			"title": "Plan the channel huddle feature",
			"body": "@gitclaw /channels huddle e2e-slack-route,e2e-telegram-route --huddle-id huddle-1 --message-id huddle-msg-1\n\nTopic: Channel huddle launch\nAgenda:\nVisible huddle agenda with CHANNEL_HUDDLE_AGENDA_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 27001,
			"body": "@gitclaw /channels huddle e2e-slack-route,e2e-telegram-route --huddle-id huddle-1 --message-id huddle-msg-1\n\nTopic: Channel huddle launch\nAgenda:\nDo not leak duplicate secret CHANNEL_HUDDLE_DUPLICATE_SECRET",
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
		t.Fatalf("duplicate huddle created more issues: %#v", github.Issues)
	}
	for _, issue := range github.Issues[1:] {
		if got := len(github.CommentsByIssue[issue.Number]); got != 1 {
			t.Fatalf("duplicate huddle posted another outbound comment on issue %d: %d", issue.Number, got)
		}
	}
	duplicateReceipt := github.CommentsByIssue[270][1].Body
	for _, want := range []string{
		"channel_huddle_status: `duplicate`",
		"huddle_issue_created: `false`",
		"huddle_invites_queued: `0`",
		"huddle_invite_duplicates: `2`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate huddle receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_HUDDLE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate huddle receipt leaked body:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelHuddleActionRequestParsesTopicAndAgenda(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 27,
			Title:  "Design review",
		},
		Comment: &Comment{
			ID: 2701,
			Body: `@gitclaw /channels room --route Team-Demo --huddle-id design-jam --message-id huddle-27
Topic: Design review jam
Agenda:
Sketch the route UX.`,
		},
	}
	req, err := BuildChannelHuddleActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelHuddleActionRequest returned error: %v", err)
	}
	if req.Subcommand != "room" || req.Options.Routes[0] != "team-demo" || req.Options.HuddleID != "design-jam" || req.Options.MessageID != "huddle-27" {
		t.Fatalf("unexpected huddle target parsing: %#v", req)
	}
	if req.Options.Topic != "Design review jam" || !strings.Contains(req.Options.Agenda, "Sketch the route UX.") {
		t.Fatalf("unexpected huddle agenda parsing: %#v", req)
	}
	if req.AutoHuddleID || req.AutoMessageID || req.TopicSHA == "" || req.AgendaSHA == "" || req.RoutesSHA == "" {
		t.Fatalf("expected huddle hashes and explicit ids: %#v", req)
	}
}
