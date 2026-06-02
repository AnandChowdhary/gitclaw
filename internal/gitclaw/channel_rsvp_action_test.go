package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelRsvpCreatesIssueAndInvitesRoutesWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: e2e-slack-route
    channel: slack
    thread_id_template: slack-rsvp-{route}-{message_id}
    author: gitclaw:test
  - name: e2e-telegram-route
    channel: telegram
    thread_id_template: telegram-rsvp-{route}-{message_id}
    author: gitclaw:test
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 284,
			"title": "Plan the channel RSVP feature",
			"body": "@gitclaw /channels rsvp e2e-slack-route,e2e-telegram-route --rsvp-id rsvp-1 --message-id rsvp-msg-1\n\nTitle: Tiny channel demo with CHANNEL_RSVP_TITLE_SECRET\nWhen: 2026-06-04T15:00:00Z\nWhere: Demo room\nHost: Anand\nDetails:\nBring a sharp question and CHANNEL_RSVP_DETAILS_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{284: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel RSVP action", llm.Calls)
	}
	if len(github.Issues) != 3 {
		t.Fatalf("created issues = %d, want RSVP plus two channel issues: %#v", len(github.Issues), github.Issues)
	}
	rsvp := github.Issues[0]
	if !HasChannelRsvpMarker(rsvp.Body) || !strings.Contains(rsvp.Body, `rsvp_id="rsvp-1"`) {
		t.Fatalf("RSVP issue missing marker:\n%s", rsvp.Body)
	}
	for _, want := range []string{
		"GitClaw channel RSVP",
		"Title: Tiny channel demo with CHANNEL_RSVP_TITLE_SECRET",
		"When: 2026-06-04T15:00:00Z",
		"Where: Demo room",
		"Host: Anand",
		"Bring a sharp question and CHANNEL_RSVP_DETAILS_SECRET.",
		"source_issue: `#284`",
		"raw_route_names_included: `false`",
	} {
		if !strings.Contains(rsvp.Body, want) {
			t.Fatalf("RSVP issue missing %q:\n%s", want, rsvp.Body)
		}
	}
	if !hasLabel(github.IssueLabels[rsvp.Number], "gitclaw") {
		t.Fatalf("RSVP issue missing gitclaw trigger label: %#v", github.IssueLabels[rsvp.Number])
	}

	for _, issue := range github.Issues[1:] {
		if !HasChannelThreadMarker(issue.Body) {
			t.Fatalf("RSVP target issue missing channel thread marker: %#v", issue)
		}
		comments := github.CommentsByIssue[issue.Number]
		if len(comments) != 1 {
			t.Fatalf("target issue %d comments = %d, want one outbound: %#v", issue.Number, len(comments), comments)
		}
		for _, want := range []string{
			"gitclaw:channel-outbound",
			`message_id="rsvp-msg-1"`,
			"GitClaw channel RSVP",
			"RSVP: #100",
			"https://github.com/owner/repo/issues/100",
			"Title: Tiny channel demo with CHANNEL_RSVP_TITLE_SECRET",
			"When: 2026-06-04T15:00:00Z",
			"Where: Demo room",
			"Host: Anand",
			"Bring a sharp question and CHANNEL_RSVP_DETAILS_SECRET.",
		} {
			if !strings.Contains(comments[0].Body, want) {
				t.Fatalf("target issue %d missing outbound RSVP invite %q:\n%s", issue.Number, want, comments[0].Body)
			}
		}
	}

	sourceComments := github.CommentsByIssue[284]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want RSVP receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Channel RSVP Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rsvp`",
		"channel_rsvp_status: `queued`",
		"rsvp_issue: `#100`",
		"rsvp_issue_created: `true`",
		"rsvp_id_auto: `false`",
		"message_id_auto: `false`",
		"rsvp_routes: `2`",
		"rsvp_invites_queued: `2`",
		"rsvp_invite_duplicates: `0`",
		"target_issues_created: `2`",
		"raw_route_names_included: `false`",
		"raw_rsvp_id_included: `false`",
		"raw_title_included: `false`",
		"raw_when_included: `false`",
		"raw_where_included: `false`",
		"raw_host_included: `false`",
		"raw_details_included: `false`",
		"raw_outbound_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_rsvp_action_change: `true`",
		"channel=`slack`",
		"channel=`telegram`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("RSVP receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_RSVP_TITLE_SECRET", "CHANNEL_RSVP_DETAILS_SECRET", "Tiny channel demo", "2026-06-04T15:00:00Z", "Demo room", "Anand", "rsvp-1", "rsvp-msg-1", "e2e-slack-route", "e2e-telegram-route"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("RSVP receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 284,
			"title": "Plan the channel RSVP feature",
			"body": "@gitclaw /channels rsvp e2e-slack-route,e2e-telegram-route --rsvp-id rsvp-1 --message-id rsvp-msg-1\n\nTitle: Tiny channel demo",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 28401,
			"body": "@gitclaw /channels rsvp e2e-slack-route,e2e-telegram-route --rsvp-id rsvp-1 --message-id rsvp-msg-1\n\nTitle: Do not leak duplicate secret CHANNEL_RSVP_DUPLICATE_SECRET",
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
		t.Fatalf("duplicate RSVP created more issues: %#v", github.Issues)
	}
	for _, issue := range github.Issues[1:] {
		if got := len(github.CommentsByIssue[issue.Number]); got != 1 {
			t.Fatalf("duplicate RSVP posted another outbound comment on issue %d: %d", issue.Number, got)
		}
	}
	duplicateReceipt := github.CommentsByIssue[284][1].Body
	for _, want := range []string{
		"channel_rsvp_status: `duplicate`",
		"rsvp_issue_created: `false`",
		"rsvp_invites_queued: `0`",
		"rsvp_invite_duplicates: `2`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate RSVP receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_RSVP_DUPLICATE_SECRET") {
		t.Fatalf("duplicate RSVP receipt leaked body:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelRsvpActionRequestParsesEventDetails(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 29,
			Title:  "Design RSVP",
		},
		Comment: &Comment{
			ID: 2901,
			Body: `@gitclaw /channels event --route Team-Demo --rsvp-id Design.RSVP --message-id rsvp-29
Title: Design review
When: Friday 10:00 UTC
Where: Huddle room
Host: Product
Details:
Bring sketches.`,
		},
	}
	req, err := BuildChannelRsvpActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRsvpActionRequest returned error: %v", err)
	}
	if req.Subcommand != "event" || req.Options.Routes[0] != "team-demo" || req.Options.RsvpID != "design-rsvp" || req.Options.MessageID != "rsvp-29" {
		t.Fatalf("unexpected RSVP target parsing: %#v", req)
	}
	if req.Options.Title != "Design review" || req.Options.When != "Friday 10:00 UTC" || req.Options.Where != "Huddle room" || req.Options.Host != "Product" {
		t.Fatalf("unexpected RSVP event fields: %#v", req.Options)
	}
	if req.Options.Details != "Bring sketches." {
		t.Fatalf("unexpected RSVP details: %#v", req.Options.Details)
	}
	if req.AutoRsvpID || req.AutoMessageID || req.TitleSHA == "" || req.DetailsSHA == "" || req.RoutesSHA == "" {
		t.Fatalf("expected RSVP hashes and explicit ids: %#v", req)
	}
}
