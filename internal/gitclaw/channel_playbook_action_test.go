package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelPlaybookCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-playbook-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-playbook-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-playbook-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels playbook --playbook-id playbook-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness playbook\nSteps:\nVisible steps token CHANNEL_PLAYBOOK_STEPS_SECRET.\nChecks:\nVisible check token CHANNEL_PLAYBOOK_CHECKS_SECRET.\nRollback:\nVisible rollback token CHANNEL_PLAYBOOK_ROLLBACK_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 384,
			Title:  "GitClaw telegram thread chat-playbook-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-playbook-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_PLAYBOOK_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel playbook action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one playbook issue: %#v", len(github.Issues), github.Issues)
	}
	playbook := github.Issues[1]
	if !HasChannelPlaybookMarker(playbook.Body) || !strings.Contains(playbook.Body, `playbook_id="playbook-1"`) {
		t.Fatalf("playbook issue missing channel-playbook marker:\n%s", playbook.Body)
	}
	for _, want := range []string{
		"GitClaw channel playbook",
		"playbook_id: playbook-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"playbook_mode: github-issue-playbook",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness playbook",
		"## Steps",
		"Visible steps token CHANNEL_PLAYBOOK_STEPS_SECRET.",
		"## Checks",
		"Visible check token CHANNEL_PLAYBOOK_CHECKS_SECRET.",
		"## Rollback",
		"Visible rollback token CHANNEL_PLAYBOOK_ROLLBACK_SECRET.",
	} {
		if !strings.Contains(playbook.Body, want) {
			t.Fatalf("playbook issue missing %q:\n%s", want, playbook.Body)
		}
	}
	if strings.Contains(playbook.Body, "chat-playbook-123") || strings.Contains(playbook.Body, "inbound-384") || strings.Contains(playbook.Body, "CHANNEL_PLAYBOOK_INGEST_SECRET") {
		t.Fatalf("playbook issue leaked provider IDs or channel body:\n%s", playbook.Body)
	}
	if !hasLabel(github.IssueLabels[playbook.Number], "gitclaw") {
		t.Fatalf("playbook issue missing gitclaw trigger label: %#v", github.IssueLabels[playbook.Number])
	}

	sourceComments := github.CommentsByIssue[384]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-384"`,
		"GitClaw channel playbook recorded.",
		"Playbook: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness playbook",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("playbook notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_PLAYBOOK_STEPS_SECRET", "CHANNEL_PLAYBOOK_CHECKS_SECRET", "CHANNEL_PLAYBOOK_ROLLBACK_SECRET", "CHANNEL_PLAYBOOK_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("playbook notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Playbook Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels playbook`",
		"channel_playbook_status: `recorded`",
		"playbook_issue: `#101`",
		"playbook_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_playbook_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_playbook_title_included: `false`",
		"raw_playbook_steps_included: `false`",
		"raw_playbook_checks_included: `false`",
		"raw_playbook_rollback_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_playbook_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel playbook receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PLAYBOOK_INGEST_SECRET", "CHANNEL_PLAYBOOK_STEPS_SECRET", "CHANNEL_PLAYBOOK_CHECKS_SECRET", "CHANNEL_PLAYBOOK_ROLLBACK_SECRET", "Launch readiness playbook", "playbook-1", "chat-playbook-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel playbook receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-playbook-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-playbook-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels playbook --playbook-id playbook-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness playbook\nSteps:\nDo not leak duplicate token CHANNEL_PLAYBOOK_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate playbook created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate playbook posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_playbook_status: `duplicate`",
		"playbook_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate playbook receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_PLAYBOOK_DUPLICATE_SECRET") {
		t.Fatalf("duplicate playbook receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelPlaybookActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel playbook"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel runbook --route team-demo --playbook-id Weekly.Playbook --message-id source-1 --notify-message-id notify-1
Playbook: The channel reached launch readiness
Steps:
- Design is stable.
Checks:
- Release checklist moved late.
Rollback:
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelPlaybookActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPlaybookActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "runbook" || req.Options.Route != "team-demo" || req.Options.PlaybookID != "weekly-playbook" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel playbook parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.Steps, "Design is stable") || !strings.Contains(req.Options.Checks, "Release checklist") || !strings.Contains(req.Options.Rollback, "Follow-up moves") {
		t.Fatalf("unexpected playbook sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoPlaybookID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.StepsSHA == "" || req.ChecksSHA == "" || req.RollbackSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route playbook hashes: %#v", req)
	}
}

func TestIsChannelPlaybookActionFieldsKeepsDigestAliasesSeparate(t *testing.T) {
	if isChannelPlaybookActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a playbook alias")
	}
	if !isChannelPlaybookActionFields([]string{"/channels", "runbook"}) {
		t.Fatalf("runbook should be accepted as a playbook alias")
	}
}
