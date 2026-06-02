package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSkillSearchQueuesRecallWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelSkillSearchFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-skill-search-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 905,
			"title": "GitClaw telegram thread chat-skill-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90501,
			"body": "@gitclaw /channels skill-search repo-reader --message-id skill-search-inbound-905 --notify-message-id skill-search-notify-905 --search-id Skill.Search.Secret.905 --max-results 1\nDo not include this command hidden token in the receipt: CHANNEL_SKILL_SEARCH_COMMAND_MARKER.",
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
			Number: 905,
			Title:  "GitClaw telegram thread chat-skill-search-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{905: {{
			ID: 90500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-skill-search-123",
				MessageID: "skill-search-inbound-905",
				Author:    "telegram",
				Body:      "Original mirrored skill search command with CHANNEL_SKILL_SEARCH_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{905: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel skill search action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("skill search action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[905]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="skill-search-notify-905"`,
		"GitClaw channel skill search",
		"Search status: ok",
		"Query hash: ",
		"Query terms: 1",
		"Max results: 1",
		"Available skills: 2",
		"Enabled skills: 2",
		"Matched skills: 1",
		"Results returned: 1",
		"Search id hash: ",
		"skill_name=repo-reader",
		"path=.gitclaw/SKILLS/repo-reader/SKILL.md",
		"folder=repo-reader",
		"match_fields=",
		"enabled=true",
		"frontmatter=true",
		"description_present=true",
		"sha256_12=",
		"Raw skill bodies, skill descriptions, channel bodies, issue bodies, comment bodies, prompts, tool outputs, and raw search queries are not included.",
		"Skill install: not performed by this action.",
		"Skill update: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Installer scripts: not run by this action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("skill search notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_SEARCH_INGEST_MARKER", "CHANNEL_SKILL_SEARCH_COMMAND_MARKER", "CHANNEL_SKILL_SEARCH_DESCRIPTION_SECRET", "CHANNEL_SKILL_SEARCH_BODY_SECRET", "Skill.Search.Secret.905", "Use read-only repository context."} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("skill search notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Skill Search Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels skill-search`",
		"channel_skill_search_status: `queued`",
		"skill_search_status: `ok`",
		"search_mode: `repo-local-skill-metadata-lexical`",
		"notification_target_issue: `#905`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"skill_search_id_sha256_12: `",
		"skill_search_id_auto: `false`",
		"query_sha256_12: `",
		"query_terms: `1`",
		"query_bytes: `11`",
		"query_source: `positional`",
		"max_results: `1`",
		"available_skills: `2`",
		"enabled_skills: `2`",
		"matched_skills: `1`",
		"results_returned: `1`",
		"matched_skill_names_sha256_12: `",
		"matched_skill_paths_sha256_12: `",
		"matched_skill_index_sha256_12: `",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"progressive_disclosure_enabled: `true`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"registry_contact_allowed: `false`",
		"installer_scripts_run: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_query_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_skill_search_id_included: `false`",
		"raw_skill_names_included: `false`",
		"raw_skill_paths_included: `false`",
		"raw_skill_descriptions_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_skill_search_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel skill search receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"repo-reader", ".gitclaw/SKILLS/repo-reader/SKILL.md", "CHANNEL_SKILL_SEARCH_INGEST_MARKER", "CHANNEL_SKILL_SEARCH_COMMAND_MARKER", "CHANNEL_SKILL_SEARCH_DESCRIPTION_SECRET", "CHANNEL_SKILL_SEARCH_BODY_SECRET", "Use read-only repository context.", "chat-skill-search-123", "skill-search-inbound-905", "skill-search-notify-905", "Skill.Search.Secret.905"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel skill search receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 905,
			"title": "GitClaw telegram thread chat-skill-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90502,
			"body": "@gitclaw /channels search-skills repo-reader --message-id skill-search-inbound-905 --notify-message-id skill-search-notify-905 --search-id Skill.Search.Secret.905 --max-results 1\nDo not include duplicate hidden token CHANNEL_SKILL_SEARCH_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[905]); got != 4 {
		t.Fatalf("duplicate skill search posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[905])
	}
	duplicateReceipt := github.CommentsByIssue[905][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels search-skills`",
		"channel_skill_search_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate skill search receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"repo-reader", "CHANNEL_SKILL_SEARCH_DUPLICATE_MARKER", "chat-skill-search-123", "skill-search-inbound-905", "skill-search-notify-905", "Skill.Search.Secret.905"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate skill search receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSkillSearchActionRequestParsesRouteAliasAndTrailingQuery(t *testing.T) {
	root := t.TempDir()
	writeChannelSkillSearchFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel skill search"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel capability-search --route team-demo --message-id source-1 --notify-message-id notify-1 --id Skill.Search.One --max-results 5
Skill: repo-reader`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel capability-search"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelSkillSearchActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSkillSearchActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "capability-search" || req.Options.Route != "team-demo" || req.Options.Query != "repo-reader" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SearchID != "skill-search-one" || req.Options.MaxResults != 5 {
		t.Fatalf("unexpected channel skill search parsing: %#v", req)
	}
	if req.QuerySource != "trailing-query" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSearchID {
		t.Fatalf("unexpected channel skill search defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SearchIDHash == "" || req.QuerySHA == "" || req.NotificationBodySHA == "" || req.Search.ResultsReturned != 1 {
		t.Fatalf("expected route search hashes and result: %#v", req)
	}
	if !IsChannelSkillSearchActionRequest(ev, cfg) {
		t.Fatalf("expected channel capability-search alias to be recognized")
	}
}

func writeChannelSkillSearchFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context. CHANNEL_SKILL_SEARCH_DESCRIPTION_SECRET.
---
Full skill body secret CHANNEL_SKILL_SEARCH_BODY_SECRET.
`)
	writeTestFile(t, root, ".gitclaw/SKILLS/weekly-review/SKILL.md", `---
name: weekly-review
description: Summarize weekly planning notes.
---
Full skill body secret CHANNEL_SKILL_SEARCH_OTHER_BODY_SECRET.
`)
}
