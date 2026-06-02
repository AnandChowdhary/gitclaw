package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSkillInfoQueuesFocusedCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelSkillInfoFixture(t, root)
	t.Setenv("GITCLAW_CHANNEL_SKILL_INFO_ENV", "CHANNEL_SKILL_INFO_ENV_VALUE_SECRET")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-skill-info-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 908,
			"title": "GitClaw telegram thread chat-skill-info-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-info-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90801,
			"body": "@gitclaw /channels skill-info repo-reader --message-id skill-info-inbound-908 --notify-message-id skill-info-notify-908 --skill-info-id Skill.Info.Secret.908\nDo not include this command hidden token in the receipt: CHANNEL_SKILL_INFO_COMMAND_MARKER.",
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
			Number: 908,
			Title:  "GitClaw telegram thread chat-skill-info-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{908: {{
			ID: 90800,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-skill-info-123",
				MessageID: "skill-info-inbound-908",
				Author:    "telegram",
				Body:      "Original mirrored skill info command with CHANNEL_SKILL_INFO_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{908: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel skill info action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("skill info action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[908]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="skill-info-notify-908"`,
		"GitClaw channel skill info",
		"Skill info status: ok",
		"Requested skill hash: ",
		"Normalized skill hash: ",
		"Available skills: 1",
		"Enabled skills: 1",
		"Matched skills: 1",
		"Validation status: ok",
		"Validation errors: 0",
		"Validation warnings: 0",
		"Skill info id hash: ",
		"Skills:",
		"skill_name=repo-reader",
		"path=.gitclaw/SKILLS/repo-reader/SKILL.md",
		"folder=repo-reader",
		"enabled=true",
		"disabled_by_config=false",
		"blocked_by_allowlist=false",
		"selected_for_this_turn=false",
		"always=false",
		"frontmatter=true",
		"description_present=true",
		"requires_env=1",
		"requires_bins=1",
		"missing_env=0",
		"missing_bins=0",
		"Raw skill bodies, skill descriptions, channel bodies, issue bodies, comment bodies, prompts, tool outputs, and raw requested skill text are not included.",
		"Skill install: not performed by this action.",
		"Skill update: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Installer scripts: not run by this action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("skill info notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_INFO_INGEST_MARKER", "CHANNEL_SKILL_INFO_COMMAND_MARKER", "CHANNEL_SKILL_INFO_FILE_SECRET", "CHANNEL_SKILL_INFO_BODY_SECRET", "Skill.Info.Secret.908", "Use read-only repository files.", "CHANNEL_SKILL_INFO_ENV_VALUE_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("skill info notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Skill Info Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels skill-info`",
		"channel_skill_info_status: `queued`",
		"skill_info_status: `ok`",
		"info_mode: `repo-local-skill-metadata-card`",
		"notification_target_issue: `#908`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"skill_info_id_sha256_12: `",
		"skill_info_id_auto: `false`",
		"requested_skill_sha256_12: `",
		"normalized_skill_sha256_12: `",
		"requested_skill_bytes: `11`",
		"skill_source: `positional`",
		"available_skills: `1`",
		"enabled_skills: `1`",
		"matched_skills: `1`",
		"validation_status: `ok`",
		"validation_errors: `0`",
		"validation_warnings: `0`",
		"matched_skill_names_sha256_12: `",
		"matched_skill_paths_sha256_12: `",
		"skill_info_index_sha256_12: `",
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
		"raw_requested_skill_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_skill_info_id_included: `false`",
		"raw_skill_names_included: `false`",
		"raw_skill_paths_included: `false`",
		"raw_skill_descriptions_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_skill_info_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel skill info receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"repo-reader", ".gitclaw/SKILLS/repo-reader/SKILL.md", "GITCLAW_CHANNEL_SKILL_INFO_ENV", "CHANNEL_SKILL_INFO_INGEST_MARKER", "CHANNEL_SKILL_INFO_COMMAND_MARKER", "CHANNEL_SKILL_INFO_FILE_SECRET", "CHANNEL_SKILL_INFO_BODY_SECRET", "chat-skill-info-123", "skill-info-inbound-908", "skill-info-notify-908", "Skill.Info.Secret.908"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel skill info receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 908,
			"title": "GitClaw telegram thread chat-skill-info-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-info-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90802,
			"body": "@gitclaw /channels describe-skill repo-reader --message-id skill-info-inbound-908 --notify-message-id skill-info-notify-908 --skill-info-id Skill.Info.Secret.908\nDo not include duplicate hidden token CHANNEL_SKILL_INFO_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[908]); got != 4 {
		t.Fatalf("duplicate skill info posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[908])
	}
	duplicateReceipt := github.CommentsByIssue[908][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels describe-skill`",
		"channel_skill_info_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate skill info receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"repo-reader", ".gitclaw/SKILLS/repo-reader/SKILL.md", "CHANNEL_SKILL_INFO_DUPLICATE_MARKER", "chat-skill-info-123", "skill-info-inbound-908", "skill-info-notify-908", "Skill.Info.Secret.908"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate skill info receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSkillInfoActionRequestParsesRouteAliasAndTrailingSkill(t *testing.T) {
	root := t.TempDir()
	writeChannelSkillInfoFixture(t, root)
	t.Setenv("GITCLAW_CHANNEL_SKILL_INFO_ENV", "CHANNEL_SKILL_INFO_ENV_VALUE_SECRET")
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel skill info"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel skill-capability-describe --route team-demo --message-id source-1 --notify-message-id notify-1 --id Skill.Info.One
Skill: repo-reader`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel skill-capability-describe repo-reader"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelSkillInfoActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSkillInfoActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "skill-capability-describe" || req.Options.Route != "team-demo" || req.Options.RequestedSkill != "repo-reader" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.InfoID != "skill-info-one" {
		t.Fatalf("unexpected channel skill info parsing: %#v", req)
	}
	if req.SkillSource != "trailing-skill" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoInfoID {
		t.Fatalf("unexpected channel skill info defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.InfoIDHash == "" || req.RequestedSkillHash == "" || req.NormalizedSkillHash == "" || req.NotificationBodySHA == "" || req.Info.MatchedSkills != 1 {
		t.Fatalf("expected route info hashes and match: %#v", req)
	}
	if !IsChannelSkillInfoActionRequest(ev, cfg) {
		t.Fatalf("expected channel skill-capability-describe alias to be recognized")
	}
}

func writeChannelSkillInfoFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "README.md", "Channel skill info fixture with CHANNEL_SKILL_INFO_FILE_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
metadata:
  openclaw:
    requires:
      env:
        - GITCLAW_CHANNEL_SKILL_INFO_ENV
      bins: [git]
---

# Repo Reader
CHANNEL_SKILL_INFO_BODY_SECRET
`)
}
