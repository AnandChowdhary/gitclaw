package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelSkillStatusQueuesSnapshotWithoutLLM(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".gitclaw", "SKILLS", "repo-reader")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: repo-reader
description: Use read-only repository context.
---
Full skill body secret CHANNEL_SKILL_STATUS_SKILL_BODY_SECRET.
`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-skill-status-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 886,
			"title": "GitClaw telegram thread chat-skill-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88601,
			"body": "@gitclaw /channels skills --message-id skill-status-inbound-886 --notify-message-id skill-status-notify-886 --status-id skill-status-secret-886\nDo not include this command hidden token in the receipt: CHANNEL_SKILL_STATUS_COMMAND_SECRET.",
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
			Number: 886,
			Title:  "GitClaw telegram thread chat-skill-status-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{886: {{
			ID: 88600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-skill-status-123",
				MessageID: "skill-status-inbound-886",
				Author:    "telegram",
				Body:      "Original mirrored skills command with CHANNEL_SKILL_STATUS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{886: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel skill status action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("skill status should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[886]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="skill-status-notify-886"`,
		"GitClaw channel skill status.",
		"Available skills: 1",
		"Enabled skills: 1",
		"Disabled skills: 0",
		"Allowlist blocked skills: 0",
		"Selected skills for this turn: 0",
		"Enabled skill names: repo-reader",
		"Skills with frontmatter: 1",
		"Skills with descriptions: 1",
		"Skills missing requirements: 0",
		"Progressive disclosure: enabled",
		"Snapshot source: current GitHub Actions checkout",
		"Full skill bodies: not included.",
		"Skill install: not performed by this action.",
		"Skill update: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Installer scripts: not run by this action.",
		"Model call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("skill-status notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_STATUS_INGEST_SECRET", "CHANNEL_SKILL_STATUS_COMMAND_SECRET", "CHANNEL_SKILL_STATUS_SKILL_BODY_SECRET", "Use read-only repository context.", "skill-status-secret-886"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("skill-status notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Skill Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels skills`",
		"channel_skill_status_status: `queued`",
		"skill_snapshot_mode: `provider-facing-skill-status`",
		"notification_target_issue: `#886`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"available_skills: `1`",
		"enabled_skills: `1`",
		"disabled_skills: `0`",
		"allowlist_blocked_skills: `0`",
		"selected_skills: `0`",
		"skills_with_frontmatter: `1`",
		"skills_with_descriptions: `1`",
		"skills_missing_requirements: `0`",
		"progressive_disclosure_enabled: `true`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"registry_contact_allowed: `false`",
		"installer_scripts_run: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_skill_status_id_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_skill_descriptions_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_skill_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel skill status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_STATUS_INGEST_SECRET", "CHANNEL_SKILL_STATUS_COMMAND_SECRET", "CHANNEL_SKILL_STATUS_SKILL_BODY_SECRET", "Use read-only repository context.", "chat-skill-status-123", "skill-status-inbound-886", "skill-status-notify-886", "skill-status-secret-886"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel skill status receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 886,
			"title": "GitClaw telegram thread chat-skill-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88602,
			"body": "@gitclaw /channels skill-status --message-id skill-status-inbound-886 --notify-message-id skill-status-notify-886 --status-id skill-status-secret-886\nDo not leak duplicate token CHANNEL_SKILL_STATUS_DUPLICATE_SECRET.",
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
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate skill status created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[886]); got != 4 {
		t.Fatalf("duplicate skill status posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[886])
	}
	duplicateReceipt := github.CommentsByIssue[886][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels skill-status`",
		"channel_skill_status_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"registry_contact_allowed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate skill status receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_STATUS_DUPLICATE_SECRET", "chat-skill-status-123", "skill-status-inbound-886", "skill-status-notify-886", "skill-status-secret-886"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate skill status receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSkillStatusActionRequestParsesRouteAlias(t *testing.T) {
	repoContext := RepoContext{SkillSummaries: []SkillSummary{{
		Name:               "repo-reader",
		Path:               ".gitclaw/SKILLS/repo-reader/SKILL.md",
		FrontmatterPresent: true,
		SHA:                "abc123",
	}}}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel skills"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel capabilities --route team-demo --message-id source-1 --notify-message-id notify-1 --status-id skills-1`,
		},
	}
	req, err := BuildChannelSkillStatusActionRequest(ev, DefaultConfig(), repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSkillStatusActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "capabilities" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StatusID != "skills-1" {
		t.Fatalf("unexpected channel skill status parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStatusID || req.RequestedRouteHash == "" || req.StatusIDHash == "" || req.EnabledSkillNamesHash == "" || req.SkillPathsHash == "" || req.SkillIndexHash == "" {
		t.Fatalf("expected explicit route skill-status hashes: %#v", req)
	}
}
