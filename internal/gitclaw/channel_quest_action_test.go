package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelQuestCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-quest-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-quest-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-quest-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels quest --quest-id quest-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness quest\nObjective:\nVisible objective token CHANNEL_QUEST_OBJECTIVE_SECRET.\nFirst Move:\nVisible first move token CHANNEL_QUEST_FIRST_MOVE_SECRET.\nWin Condition:\nVisible win condition token CHANNEL_QUEST_WIN_CONDITION_SECRET.",
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
			Title:  "GitClaw telegram thread chat-quest-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-quest-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_QUEST_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel quest action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one quest issue: %#v", len(github.Issues), github.Issues)
	}
	quest := github.Issues[1]
	if !HasChannelQuestMarker(quest.Body) || !strings.Contains(quest.Body, `quest_id="quest-1"`) {
		t.Fatalf("quest issue missing channel-quest marker:\n%s", quest.Body)
	}
	for _, want := range []string{
		"GitClaw channel quest",
		"quest_id: quest-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"quest_mode: github-issue-quest",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness quest",
		"## Objective",
		"Visible objective token CHANNEL_QUEST_OBJECTIVE_SECRET.",
		"## First Move",
		"Visible first move token CHANNEL_QUEST_FIRST_MOVE_SECRET.",
		"## Win Condition",
		"Visible win condition token CHANNEL_QUEST_WIN_CONDITION_SECRET.",
	} {
		if !strings.Contains(quest.Body, want) {
			t.Fatalf("quest issue missing %q:\n%s", want, quest.Body)
		}
	}
	if strings.Contains(quest.Body, "chat-quest-123") || strings.Contains(quest.Body, "inbound-384") || strings.Contains(quest.Body, "CHANNEL_QUEST_INGEST_SECRET") {
		t.Fatalf("quest issue leaked provider IDs or channel body:\n%s", quest.Body)
	}
	if !hasLabel(github.IssueLabels[quest.Number], "gitclaw") {
		t.Fatalf("quest issue missing gitclaw trigger label: %#v", github.IssueLabels[quest.Number])
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
		"GitClaw channel quest recorded.",
		"Quest: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness quest",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("quest notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_QUEST_OBJECTIVE_SECRET", "CHANNEL_QUEST_FIRST_MOVE_SECRET", "CHANNEL_QUEST_WIN_CONDITION_SECRET", "CHANNEL_QUEST_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("quest notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Quest Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels quest`",
		"channel_quest_status: `recorded`",
		"quest_issue: `#101`",
		"quest_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_quest_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_quest_title_included: `false`",
		"raw_quest_objective_included: `false`",
		"raw_quest_first_move_included: `false`",
		"raw_quest_win_condition_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_quest_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel quest receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_QUEST_INGEST_SECRET", "CHANNEL_QUEST_OBJECTIVE_SECRET", "CHANNEL_QUEST_FIRST_MOVE_SECRET", "CHANNEL_QUEST_WIN_CONDITION_SECRET", "Launch readiness quest", "quest-1", "chat-quest-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel quest receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-quest-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-quest-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels quest --quest-id quest-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness quest\nObjective:\nDo not leak duplicate token CHANNEL_QUEST_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate quest created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate quest posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_quest_status: `duplicate`",
		"quest_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate quest receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_QUEST_DUPLICATE_SECRET") {
		t.Fatalf("duplicate quest receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelQuestActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel quest"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel challenge --route team-demo --quest-id Weekly.Quest --message-id source-1 --notify-message-id notify-1
Quest: The channel reached launch readiness
Objective:
- Design is stable.
First Move:
- Release checklist moved late.
Win Condition:
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelQuestActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelQuestActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "challenge" || req.Options.Route != "team-demo" || req.Options.QuestID != "weekly-quest" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel quest parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.Objective, "Design is stable") || !strings.Contains(req.Options.FirstMove, "Release checklist") || !strings.Contains(req.Options.WinCondition, "Follow-up moves") {
		t.Fatalf("unexpected quest sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoQuestID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.ObjectiveSHA == "" || req.FirstMoveSHA == "" || req.WinConditionSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route quest hashes: %#v", req)
	}
}

func TestIsChannelQuestActionFieldsKeepsPlaybookAndDigestAliasesSeparate(t *testing.T) {
	if isChannelQuestActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a quest alias")
	}
	if isChannelQuestActionFields([]string{"/channels", "runbook"}) {
		t.Fatalf("runbook should remain a playbook alias, not a quest alias")
	}
	if !isChannelQuestActionFields([]string{"/channels", "challenge"}) {
		t.Fatalf("challenge should be accepted as a quest alias")
	}
}
