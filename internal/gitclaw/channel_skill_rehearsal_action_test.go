package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSkillRehearsalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---
Use repository file tools for grounded answers.

CHANNEL_SKILL_REHEARSAL_SKILL_BODY_SECRET
`)
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "slack",
		ThreadID: "channel-skill-rehearsal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 486,
			"title": "GitClaw slack thread channel-skill-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"slack\" thread_id=\"channel-skill-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48601,
			"body": "@gitclaw /channels rehearse-skill repo-reader --id channel-skill-review --message-id inbound-486 --notify-message-id notify-486\nPlease rehearse this channel-origin skill request.\nCHANNEL_SKILL_REHEARSAL_SOURCE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 486,
			Title:  "GitClaw slack thread channel-skill-rehearsal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{486: {{
			ID: 48600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "slack",
				ThreadID:  "channel-skill-rehearsal-thread-123",
				MessageID: "inbound-486",
				Author:    "slack",
				Body:      "Original mirrored message with CHANNEL_SKILL_REHEARSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{486: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel skill rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one skill rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsalIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:skill-rehearsal-issue",
		`id="channel-skill-review"`,
		`skill="repo-reader"`,
		"rehearsal_id: channel-skill-review",
		"requested_skill: repo-reader",
		"matched_skills: 1",
		"enabled_matches: 1",
		"source_issue: #486",
		"source_kind: channel_comment",
		"skill_install_allowed: false",
		"skill_update_allowed: false",
		"active_skill_write_performed: false",
		"raw_source_body_included: false",
		"raw_skill_body_included: false",
	} {
		if !strings.Contains(rehearsalIssue.Body, want) {
			t.Fatalf("skill rehearsal issue missing %q:\n%s", want, rehearsalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_REHEARSAL_SOURCE_SECRET", "CHANNEL_SKILL_REHEARSAL_INGEST_SECRET", "CHANNEL_SKILL_REHEARSAL_SKILL_BODY_SECRET", "Please rehearse this channel-origin"} {
		if strings.Contains(rehearsalIssue.Body, leaked) {
			t.Fatalf("skill rehearsal issue leaked %q:\n%s", leaked, rehearsalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[486]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="slack"`,
		`message_id="notify-486"`,
		"GitClaw channel skill rehearsal",
		"Rehearsal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Requested skill: repo-reader",
		"Matched skills: 1",
		"Enabled matches: 1",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel skill rehearsal notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_SKILL_REHEARSAL_SOURCE_SECRET") || strings.Contains(outbound, "CHANNEL_SKILL_REHEARSAL_INGEST_SECRET") || strings.Contains(outbound, "CHANNEL_SKILL_REHEARSAL_SKILL_BODY_SECRET") {
		t.Fatalf("channel skill rehearsal notification leaked source:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Skill Rehearsal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rehearse-skill`",
		"channel_skill_rehearsal_status: `created`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#486`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"requested_skill: `repo-reader`",
		"matched_skills: `1`",
		"enabled_matches: `1`",
		"rehearsal_mode: `github-issue-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"active_skill_write_performed: `false`",
		"raw_rehearsal_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_skill_body_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_skill_rehearsal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel skill rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_REHEARSAL_SOURCE_SECRET", "CHANNEL_SKILL_REHEARSAL_INGEST_SECRET", "CHANNEL_SKILL_REHEARSAL_SKILL_BODY_SECRET", "Please rehearse this channel-origin", "channel-skill-review", "channel-skill-rehearsal-thread-123", "inbound-486", "notify-486"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel skill rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 486,
			"title": "GitClaw slack thread channel-skill-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"slack\" thread_id=\"channel-skill-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48602,
			"body": "@gitclaw /channels rehearse-skill repo-reader --id channel-skill-review --message-id inbound-486 --notify-message-id notify-486\nDo not leak duplicate token CHANNEL_SKILL_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel skill rehearsal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[486]); got != 4 {
		t.Fatalf("duplicate channel skill rehearsal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[486])
	}
	duplicateReceipt := github.CommentsByIssue[486][3].Body
	for _, want := range []string{
		"channel_skill_rehearsal_status: `duplicate`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel skill rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_SKILL_REHEARSAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate channel skill rehearsal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelSkillRehearsalActionRequestParsesAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---
Use repository file tools for grounded answers.
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContext(root, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 33, Title: "Channel skill rehearsal"},
		Comment: &Comment{
			ID:   3301,
			Body: `@gitclaw /channel skill-trial --skill Repo-Reader --id Channel.Skill.Rehearsal --channel telegram --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelSkillRehearsalActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSkillRehearsalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "skill-trial" || req.Options.Channel != "telegram" || req.Options.RehearsalID != "channel-skill-rehearsal" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel skill rehearsal parsing: %#v", req)
	}
	if req.Rehearsal.RequestedSkill != "repo-reader" || req.Rehearsal.MatchedSkillCount != 1 || req.TargetFromIssue || req.AutoRehearsalID || req.AutoNotifyMessageID {
		t.Fatalf("unexpected skill rehearsal details: %#v", req)
	}
}
