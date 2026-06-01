package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelRollcallCreatesIssueAndInvitesRoutesWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: e2e-slack-route
    channel: slack
    thread_id_template: slack-rollcall-{route}-{message_id}
    author: gitclaw:test
  - name: e2e-telegram-route
    channel: telegram
    thread_id_template: telegram-rollcall-{route}-{message_id}
    author: gitclaw:test
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 280,
			"title": "Plan the channel rollcall feature",
			"body": "@gitclaw /channels rollcall e2e-slack-route,e2e-telegram-route --rollcall-id rollcall-1 --message-id rollcall-msg-1\n\nPrompt: What should everyone check in with for CHANNEL_ROLLCALL_PROMPT_SECRET?\nInstructions:\n- Done\n- Doing\n- Blocked",
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
		t.Fatalf("LLM called %d times for channel rollcall action", llm.Calls)
	}
	if len(github.Issues) != 3 {
		t.Fatalf("created issues = %d, want rollcall plus two channel issues: %#v", len(github.Issues), github.Issues)
	}
	rollcall := github.Issues[0]
	if !HasChannelRollcallMarker(rollcall.Body) || !strings.Contains(rollcall.Body, `rollcall_id="rollcall-1"`) {
		t.Fatalf("rollcall issue missing rollcall marker:\n%s", rollcall.Body)
	}
	for _, want := range []string{
		"GitClaw channel rollcall",
		"What should everyone check in with for CHANNEL_ROLLCALL_PROMPT_SECRET?",
		"- Done",
		"- Doing",
		"- Blocked",
		"source_issue: `#280`",
		"raw_route_names_included: `false`",
	} {
		if !strings.Contains(rollcall.Body, want) {
			t.Fatalf("rollcall issue missing %q:\n%s", want, rollcall.Body)
		}
	}
	if !hasLabel(github.IssueLabels[rollcall.Number], "gitclaw") {
		t.Fatalf("rollcall issue missing gitclaw trigger label: %#v", github.IssueLabels[rollcall.Number])
	}

	for _, issue := range github.Issues[1:] {
		if !HasChannelThreadMarker(issue.Body) {
			t.Fatalf("rollcall target issue missing channel thread marker: %#v", issue)
		}
		comments := github.CommentsByIssue[issue.Number]
		if len(comments) != 1 {
			t.Fatalf("target issue %d comments = %d, want one outbound: %#v", issue.Number, len(comments), comments)
		}
		for _, want := range []string{
			"gitclaw:channel-outbound",
			`message_id="rollcall-msg-1"`,
			"GitClaw channel rollcall",
			"Rollcall: #100",
			"https://github.com/owner/repo/issues/100",
			"CHANNEL_ROLLCALL_PROMPT_SECRET",
		} {
			if !strings.Contains(comments[0].Body, want) {
				t.Fatalf("target issue %d missing outbound rollcall invite %q:\n%s", issue.Number, want, comments[0].Body)
			}
		}
	}

	sourceComments := github.CommentsByIssue[280]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want rollcall receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Channel Rollcall Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rollcall`",
		"channel_rollcall_status: `queued`",
		"rollcall_issue: `#100`",
		"rollcall_issue_created: `true`",
		"rollcall_id_auto: `false`",
		"message_id_auto: `false`",
		"rollcall_routes: `2`",
		"rollcall_invites_queued: `2`",
		"rollcall_invite_duplicates: `0`",
		"target_issues_created: `2`",
		"raw_route_names_included: `false`",
		"raw_rollcall_id_included: `false`",
		"raw_prompt_included: `false`",
		"raw_instructions_included: `false`",
		"raw_outbound_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_rollcall_action_change: `true`",
		"channel=`slack`",
		"channel=`telegram`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("rollcall receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ROLLCALL_PROMPT_SECRET", "What should everyone check in", "rollcall-1", "rollcall-msg-1", "e2e-slack-route", "e2e-telegram-route"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("rollcall receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 280,
			"title": "Plan the channel rollcall feature",
			"body": "@gitclaw /channels rollcall e2e-slack-route,e2e-telegram-route --rollcall-id rollcall-1 --message-id rollcall-msg-1\n\nPrompt: What should everyone check in with for CHANNEL_ROLLCALL_PROMPT_SECRET?\nInstructions:\n- Done\n- Doing\n- Blocked",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 28001,
			"body": "@gitclaw /channels rollcall e2e-slack-route,e2e-telegram-route --rollcall-id rollcall-1 --message-id rollcall-msg-1\n\nPrompt: Do not leak duplicate secret CHANNEL_ROLLCALL_DUPLICATE_SECRET\nInstructions:\n- Again",
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
		t.Fatalf("duplicate rollcall created more issues: %#v", github.Issues)
	}
	for _, issue := range github.Issues[1:] {
		if got := len(github.CommentsByIssue[issue.Number]); got != 1 {
			t.Fatalf("duplicate rollcall posted another outbound comment on issue %d: %d", issue.Number, got)
		}
	}
	duplicateReceipt := github.CommentsByIssue[280][1].Body
	for _, want := range []string{
		"channel_rollcall_status: `duplicate`",
		"rollcall_issue_created: `false`",
		"rollcall_invites_queued: `0`",
		"rollcall_invite_duplicates: `2`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate rollcall receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_ROLLCALL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate rollcall receipt leaked body:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelRollcallActionRequestParsesPromptAndInstructions(t *testing.T) {
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
			Body: `@gitclaw /channels check-in --route Team-Demo --rollcall-id Design.Checkin --message-id rollcall-27
Prompt: What shipped today?
Format:
- shipped
- blocked`,
		},
	}
	req, err := BuildChannelRollcallActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRollcallActionRequest returned error: %v", err)
	}
	if req.Subcommand != "check-in" || req.Options.Routes[0] != "team-demo" || req.Options.RollcallID != "design-checkin" || req.Options.MessageID != "rollcall-27" {
		t.Fatalf("unexpected rollcall target parsing: %#v", req)
	}
	if req.Options.Prompt != "What shipped today?" || !strings.Contains(req.Options.Instructions, "shipped") || !strings.Contains(req.Options.Instructions, "blocked") {
		t.Fatalf("unexpected rollcall body parsing: %#v", req)
	}
	if req.AutoRollcallID || req.AutoMessageID || req.PromptSHA == "" || req.InstructionsSHA == "" || req.RoutesSHA == "" {
		t.Fatalf("expected rollcall hashes and explicit ids: %#v", req)
	}
}
