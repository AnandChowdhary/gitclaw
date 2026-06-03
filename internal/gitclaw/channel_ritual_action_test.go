package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelRitualCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-ritual-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-ritual-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-ritual-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels ritual --ritual-id ritual-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness ritual\nCadence:\nVisible cadence token CHANNEL_RITUAL_CADENCE_SECRET.\nTrigger:\nVisible trigger token CHANNEL_RITUAL_TRIGGER_SECRET.\nPractice:\nVisible practice token CHANNEL_RITUAL_PRACTICE_SECRET.\nReview:\nVisible review token CHANNEL_RITUAL_REVIEW_SECRET.",
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
			Title:  "GitClaw telegram thread chat-ritual-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-ritual-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_RITUAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel ritual action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one ritual issue: %#v", len(github.Issues), github.Issues)
	}
	ritual := github.Issues[1]
	if !HasChannelRitualMarker(ritual.Body) || !strings.Contains(ritual.Body, `ritual_id="ritual-1"`) {
		t.Fatalf("ritual issue missing channel-ritual marker:\n%s", ritual.Body)
	}
	for _, want := range []string{
		"GitClaw channel ritual",
		"ritual_id: ritual-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"ritual_mode: github-issue-ritual",
		"scheduled_workflow_created: false",
		"reminder_created: false",
		"standing_order_created: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness ritual",
		"## Cadence",
		"Visible cadence token CHANNEL_RITUAL_CADENCE_SECRET.",
		"## Trigger",
		"Visible trigger token CHANNEL_RITUAL_TRIGGER_SECRET.",
		"## Practice",
		"Visible practice token CHANNEL_RITUAL_PRACTICE_SECRET.",
		"## Review",
		"Visible review token CHANNEL_RITUAL_REVIEW_SECRET.",
	} {
		if !strings.Contains(ritual.Body, want) {
			t.Fatalf("ritual issue missing %q:\n%s", want, ritual.Body)
		}
	}
	if strings.Contains(ritual.Body, "chat-ritual-123") || strings.Contains(ritual.Body, "inbound-384") || strings.Contains(ritual.Body, "CHANNEL_RITUAL_INGEST_SECRET") {
		t.Fatalf("ritual issue leaked provider IDs or channel body:\n%s", ritual.Body)
	}
	if !hasLabel(github.IssueLabels[ritual.Number], "gitclaw") {
		t.Fatalf("ritual issue missing gitclaw trigger label: %#v", github.IssueLabels[ritual.Number])
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
		"GitClaw channel ritual recorded.",
		"Ritual: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness ritual",
		"Cadence: Visible cadence token CHANNEL_RITUAL_CADENCE_SECRET.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("ritual notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_RITUAL_TRIGGER_SECRET", "CHANNEL_RITUAL_PRACTICE_SECRET", "CHANNEL_RITUAL_REVIEW_SECRET", "CHANNEL_RITUAL_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("ritual notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Ritual Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels ritual`",
		"channel_ritual_status: `recorded`",
		"ritual_issue: `#101`",
		"ritual_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_ritual_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_ritual_title_included: `false`",
		"raw_ritual_cadence_included: `false`",
		"raw_ritual_trigger_included: `false`",
		"raw_ritual_practice_included: `false`",
		"raw_ritual_review_included: `false`",
		"raw_channel_message_body_included: `false`",
		"scheduled_workflow_created: `false`",
		"reminder_created: `false`",
		"standing_order_created: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_ritual_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel ritual receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_RITUAL_INGEST_SECRET", "CHANNEL_RITUAL_CADENCE_SECRET", "CHANNEL_RITUAL_TRIGGER_SECRET", "CHANNEL_RITUAL_PRACTICE_SECRET", "CHANNEL_RITUAL_REVIEW_SECRET", "Launch readiness ritual", "ritual-1", "chat-ritual-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel ritual receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-ritual-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-ritual-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels ritual --ritual-id ritual-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness ritual\nPractice:\nDo not leak duplicate token CHANNEL_RITUAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate ritual created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate ritual posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_ritual_status: `duplicate`",
		"ritual_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate ritual receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_RITUAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate ritual receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelRitualActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel ritual"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel routine --route team-demo --ritual-id Weekly.Ritual --message-id source-1 --notify-message-id notify-1
Ritual: The channel reached launch readiness
Cadence:
- Every Friday.
Trigger:
- Release checklist moved late.
Practice:
- Summarize the open loop.
Review:
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelRitualActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRitualActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "routine" || req.Options.Route != "team-demo" || req.Options.RitualID != "weekly-ritual" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel ritual parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.Cadence, "Every Friday") || !strings.Contains(req.Options.Trigger, "Release checklist") || !strings.Contains(req.Options.Practice, "Summarize") || !strings.Contains(req.Options.Review, "Follow-up moves") {
		t.Fatalf("unexpected ritual sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoRitualID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.CadenceSHA == "" || req.TriggerSHA == "" || req.PracticeSHA == "" || req.ReviewSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route ritual hashes: %#v", req)
	}
}

func TestIsChannelRitualActionFieldsKeepsPlaybookAndDigestAliasesSeparate(t *testing.T) {
	if isChannelRitualActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a ritual alias")
	}
	if isChannelRitualActionFields([]string{"/channels", "runbook"}) {
		t.Fatalf("runbook should remain a playbook alias, not a ritual alias")
	}
	if !isChannelRitualActionFields([]string{"/channels", "practice"}) {
		t.Fatalf("practice should be accepted as a ritual alias")
	}
}
