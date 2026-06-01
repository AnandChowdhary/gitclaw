package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelInviteQueuesIssueInvitesWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: e2e-slack-route
    channel: slack
    thread_id_template: slack-invite-{route}-{message_id}
    author: gitclaw:test
  - name: e2e-telegram-route
    channel: telegram
    thread_id_template: telegram-invite-{route}-{message_id}
    author: gitclaw:test
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 260,
			"title": "Discuss the channel invite feature",
			"body": "@gitclaw /channels invite e2e-slack-route,e2e-telegram-route --message-id invite-1\n\nInvite note with CHANNEL_INVITE_NOTE_SECRET",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{260: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel invite action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want two channel issues: %#v", len(github.Issues), github.Issues)
	}
	for _, issue := range github.Issues {
		if !HasChannelThreadMarker(issue.Body) {
			t.Fatalf("invite target issue missing channel thread marker: %#v", issue)
		}
		comments := github.CommentsByIssue[issue.Number]
		if len(comments) != 1 {
			t.Fatalf("target issue %d comments = %d, want one outbound: %#v", issue.Number, len(comments), comments)
		}
		for _, want := range []string{
			"gitclaw:channel-outbound",
			`message_id="invite-1"`,
			"GitClaw channel invite",
			"Issue: #260 Discuss the channel invite feature",
			"https://github.com/owner/repo/issues/260",
			"CHANNEL_INVITE_NOTE_SECRET",
		} {
			if !strings.Contains(comments[0].Body, want) {
				t.Fatalf("target issue %d missing outbound invite %q:\n%s", issue.Number, want, comments[0].Body)
			}
		}
	}

	sourceComments := github.CommentsByIssue[260]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want action receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Channel Invite Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels invite`",
		"channel_invite_status: `queued`",
		"invite_routes: `2`",
		"invite_queued: `2`",
		"invite_duplicates: `0`",
		"target_issues_created: `2`",
		"raw_route_names_included: `false`",
		"raw_issue_title_included: `false`",
		"raw_invite_note_included: `false`",
		"raw_outbound_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_invite_action_change: `true`",
		"channel=`slack`",
		"channel=`telegram`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("invite receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_INVITE_NOTE_SECRET", "Discuss the channel invite feature", "e2e-slack-route", "e2e-telegram-route", "invite-1"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("invite receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 260,
			"title": "Discuss the channel invite feature",
			"body": "@gitclaw /channels invite e2e-slack-route,e2e-telegram-route --message-id invite-1\n\nInvite note with CHANNEL_INVITE_NOTE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 26001,
			"body": "@gitclaw /channels invite e2e-slack-route,e2e-telegram-route --message-id invite-1\n\nRepeat note with CHANNEL_INVITE_DUPLICATE_SECRET",
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
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate invite created more target issues: %#v", github.Issues)
	}
	for _, issue := range github.Issues {
		if got := len(github.CommentsByIssue[issue.Number]); got != 1 {
			t.Fatalf("duplicate invite posted another outbound comment on issue %d: %d", issue.Number, got)
		}
	}
	duplicateReceipt := github.CommentsByIssue[260][1].Body
	for _, want := range []string{
		"channel_invite_status: `duplicate`",
		"invite_queued: `0`",
		"invite_duplicates: `2`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate invite receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_INVITE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate invite receipt leaked body:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelInviteActionRequestSupportsInlineNote(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 27,
			Title:  "Design review",
		},
		Comment: &Comment{
			ID:   2701,
			Body: "@gitclaw /channels share --route Team-Demo --message-id invite-27 --note please look here",
		},
	}
	req, err := BuildChannelInviteActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelInviteActionRequest returned error: %v", err)
	}
	if req.Subcommand != "share" || req.Options.Routes[0] != "team-demo" || req.Options.MessageID != "invite-27" {
		t.Fatalf("unexpected invite target parsing: %#v", req)
	}
	if req.InviteNoteSource != "inline" || !strings.Contains(req.Options.Body, "please look here") {
		t.Fatalf("unexpected invite note parsing: %#v", req)
	}
	if req.AutoMessageID || req.InviteNoteSHA == "" || req.OutboundBodySHA == "" || req.RoutesSHA == "" {
		t.Fatalf("expected invite hashes and explicit message id: %#v", req)
	}
}
