package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelFAQCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-faq-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-faq-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-faq-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48401,
			"body": "@gitclaw /channels faq --faq-id faq-1 --message-id inbound-484 --notify-message-id notify-484\nQuestion: How should channel FAQs be captured?\nAnswer:\nVisible FAQ answer with CHANNEL_FAQ_ANSWER_SECRET.",
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
			Number: 484,
			Title:  "GitClaw telegram thread chat-faq-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{484: {{
			ID: 48400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-faq-123",
				MessageID: "inbound-484",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_FAQ_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{484: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel faq action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one faq issue: %#v", len(github.Issues), github.Issues)
	}
	faq := github.Issues[1]
	if !HasChannelFAQMarker(faq.Body) || !strings.Contains(faq.Body, `faq_id="faq-1"`) {
		t.Fatalf("faq issue missing channel-faq marker:\n%s", faq.Body)
	}
	for _, want := range []string{
		"GitClaw channel FAQ entry",
		"faq_id: faq-1",
		"source_channel: telegram",
		"source_issue: #484",
		"source_message_id_sha256_12:",
		"faq_mode: github-issue-faq",
		"memory_mutation_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"How should channel FAQs be captured?",
		"Visible FAQ answer with CHANNEL_FAQ_ANSWER_SECRET.",
	} {
		if !strings.Contains(faq.Body, want) {
			t.Fatalf("faq issue missing %q:\n%s", want, faq.Body)
		}
	}
	if strings.Contains(faq.Body, "chat-faq-123") || strings.Contains(faq.Body, "inbound-484") || strings.Contains(faq.Body, "CHANNEL_FAQ_INGEST_SECRET") {
		t.Fatalf("faq issue leaked provider IDs or channel body:\n%s", faq.Body)
	}
	if !hasLabel(github.IssueLabels[faq.Number], "gitclaw") {
		t.Fatalf("faq issue missing gitclaw trigger label: %#v", github.IssueLabels[faq.Number])
	}

	sourceComments := github.CommentsByIssue[484]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-484"`,
		"GitClaw channel FAQ entry captured.",
		"FAQ entry: #101",
		"https://github.com/owner/repo/issues/101",
		"Question: How should channel FAQs be captured?",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("faq notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_FAQ_ANSWER_SECRET") || strings.Contains(outbound, "CHANNEL_FAQ_INGEST_SECRET") {
		t.Fatalf("faq notification leaked answer or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel FAQ Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels faq`",
		"channel_faq_status: `captured`",
		"faq_issue: `#101`",
		"faq_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#484`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_faq_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_faq_question_included: `false`",
		"raw_faq_answer_included: `false`",
		"raw_channel_message_body_included: `false`",
		"memory_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_faq_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel faq receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_FAQ_INGEST_SECRET", "CHANNEL_FAQ_ANSWER_SECRET", "How should channel FAQs be captured?", "faq-1", "chat-faq-123", "inbound-484", "notify-484"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel faq receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-faq-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-faq-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48402,
			"body": "@gitclaw /channels faq --faq-id faq-1 --message-id inbound-484 --notify-message-id notify-484\nQuestion: How should channel FAQs be captured?\nAnswer:\nDo not leak duplicate token CHANNEL_FAQ_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate faq created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[484]); got != 4 {
		t.Fatalf("duplicate faq posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[484])
	}
	duplicateReceipt := github.CommentsByIssue[484][3].Body
	for _, want := range []string{
		"channel_faq_status: `duplicate`",
		"faq_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate faq receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_FAQ_DUPLICATE_SECRET") {
		t.Fatalf("duplicate faq receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelFAQActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel FAQ"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel qna --route team-demo --question-id Roadmap.Spark --message-id source-1 --notify-message-id notify-1
Question: How should channel FAQs stay lightweight?
Answer:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable review surface.`,
		},
	}
	req, err := BuildChannelFAQActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelFAQActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "qna" || req.Options.Route != "team-demo" || req.Options.FAQID != "roadmap-spark" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel faq parsing: %#v", req)
	}
	if req.Options.Question != "How should channel FAQs stay lightweight?" || !strings.Contains(req.Options.Answer, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected question/answer: %#v", req)
	}
	if req.TargetFromIssue || req.AutoFAQID || req.AutoNotifyMessageID || req.QuestionSHA == "" || req.AnswerSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route faq hashes: %#v", req)
	}
}
