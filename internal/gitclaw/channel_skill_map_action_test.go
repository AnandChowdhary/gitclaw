package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelSkillMapQueuesSafeSequenceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".gitclaw", "SKILLS", "repo-reader")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: repo-reader
description: Use read-only repository context.
---
Full skill body secret CHANNEL_SKILL_MAP_SKILL_BODY_SECRET.
`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-skill-map-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 902,
			"title": "GitClaw telegram thread chat-skill-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90201,
			"body": "@gitclaw /channels skill-map repo-reader --message-id skill-map-inbound-902 --notify-message-id skill-map-notify-902 --map-id skill-map-secret-902\nNote: Keep skill changes reviewed\nDo not include this command hidden token in the receipt: CHANNEL_SKILL_MAP_COMMAND_SECRET.",
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
			Number: 902,
			Title:  "GitClaw telegram thread chat-skill-map-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{902: {{
			ID: 90200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-skill-map-123",
				MessageID: "skill-map-inbound-902",
				Author:    "telegram",
				Body:      "Original mirrored skill-map command with CHANNEL_SKILL_MAP_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{902: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel skill map action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("skill map should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[902]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="skill-map-notify-902"`,
		"GitClaw channel skill map.",
		"Requested skill: repo-reader",
		"Available skills: 1",
		"Enabled skills: 1",
		"Disabled skills: 0",
		"Allowlist blocked skills: 0",
		"Selected skills for this turn: 0",
		"Matched skills: 1",
		"Skills with frontmatter: 1",
		"Skills with descriptions: 1",
		"Skills missing requirements: 0",
		"Validation status: ok",
		"Validation errors: 0",
		"Validation warnings: 0",
		"Skill sequence:",
		"`/channels skills --message-id <id> --notify-message-id <id>`",
		"`/channels skill-search repo-reader --message-id <id> --notify-message-id <id>`",
		"`/channels skill-info repo-reader --message-id <id> --notify-message-id <id>`",
		"`/channels propose-skill repo-reader --message-id <id> --notify-message-id <id>`",
		"`/channels rehearse-skill repo-reader --id <rehearsal-id> --message-id <id> --notify-message-id <id>`",
		"`/channels skill-note --skill repo-reader --note-id <note-id> --message-id <id> --notify-message-id <id>`",
		"Note: Keep skill changes reviewed",
		"Note hash: ",
		"Skill map hash: ",
		"Skill step hash: ",
		"Map source: current GitHub Actions checkout skill metadata.",
		"Full skill bodies: not included.",
		"Skill install: not performed by this action.",
		"Skill update: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Installer scripts: not run by this action.",
		"Skill proposal issue creation: not performed by this action.",
		"Skill rehearsal issue creation: not performed by this action.",
		"Skill-note issue creation: not performed by this action.",
		"Model call: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("skill-map notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_MAP_INGEST_SECRET", "CHANNEL_SKILL_MAP_COMMAND_SECRET", "CHANNEL_SKILL_MAP_SKILL_BODY_SECRET", "skill-map-secret-902"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("skill-map notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Skill Map Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels skill-map`",
		"channel_skill_map_status: `queued`",
		"skill_map_mode: `provider-facing-skill-sequence`",
		"notification_target_issue: `#902`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"skill_map_id_sha256_12: `",
		"skill_map_id_auto: `false`",
		"requested_skill_sha256_12: `",
		"normalized_skill_sha256_12: `",
		"requested_skill_bytes: `11`",
		"requested_skill_terms: `1`",
		"skill_source: `positional`",
		"skill_map_note_sha256_12: `",
		"skill_map_note_bytes: `27`",
		"skill_map_note_lines: `1`",
		"skill_map_note_source: `trailing-note`",
		"skill_map_step_count: `6`",
		"skill_map_step_sha256_12: `",
		"skill_map_snapshot_sha256_12: `",
		"available_skills: `1`",
		"enabled_skills: `1`",
		"disabled_skills: `0`",
		"allowlist_blocked_skills: `0`",
		"selected_skills: `0`",
		"skills_with_frontmatter: `1`",
		"skills_with_descriptions: `1`",
		"skills_missing_requirements: `0`",
		"matched_skills: `1`",
		"validation_status: `ok`",
		"validation_errors: `0`",
		"validation_warnings: `0`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"registry_contact_allowed: `false`",
		"installer_scripts_run: `false`",
		"skill_proposal_issue_created: `false`",
		"skill_rehearsal_issue_created: `false`",
		"skill_note_issue_created: `false`",
		"model_call_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_skill_map_id_included: `false`",
		"raw_requested_skill_included: `false`",
		"raw_skill_map_note_included: `false`",
		"raw_skill_map_steps_included: `false`",
		"raw_skill_names_included: `false`",
		"raw_skill_paths_included: `false`",
		"raw_skill_descriptions_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_skill_map_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel skill map receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_MAP_INGEST_SECRET", "CHANNEL_SKILL_MAP_COMMAND_SECRET", "CHANNEL_SKILL_MAP_SKILL_BODY_SECRET", "Use read-only repository context.", "chat-skill-map-123", "skill-map-inbound-902", "skill-map-notify-902", "skill-map-secret-902", "repo-reader", "Keep skill changes reviewed"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel skill map receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 902,
			"title": "GitClaw telegram thread chat-skill-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-skill-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90202,
			"body": "@gitclaw /channels skill-path repo-reader --message-id skill-map-inbound-902 --notify-message-id skill-map-notify-902 --map-id skill-map-secret-902\nNote: Keep skill changes reviewed\nDo not leak duplicate token CHANNEL_SKILL_MAP_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate skill map created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[902]); got != 4 {
		t.Fatalf("duplicate skill map posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[902])
	}
	duplicateReceipt := github.CommentsByIssue[902][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels skill-path`",
		"channel_skill_map_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"skill_proposal_issue_created: `false`",
		"skill_rehearsal_issue_created: `false`",
		"skill_note_issue_created: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate skill map receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_MAP_DUPLICATE_SECRET", "chat-skill-map-123", "skill-map-inbound-902", "skill-map-notify-902", "skill-map-secret-902", "repo-reader", "Keep skill changes reviewed"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate skill map receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSkillMapActionRequestParsesRouteAlias(t *testing.T) {
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
		Issue:     Issue{Number: 42, Title: "Channel skill map"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel skill-runbook --route team-demo --skill repo-reader --message-id source-1 --notify-message-id notify-1 --map-id map-1 --note reviewed-only`,
		},
	}
	req, err := BuildChannelSkillMapActionRequest(ev, DefaultConfig(), repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSkillMapActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "skill-runbook" || req.Options.Route != "team-demo" || req.Options.RequestedSkill != "repo-reader" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MapID != "map-1" || req.Options.Note != "reviewed-only" {
		t.Fatalf("unexpected channel skill map parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMapID || req.RequestedRouteHash == "" || req.MapIDHash == "" || req.RequestedSkillHash == "" || req.StepSHA == "" || req.SnapshotSHA == "" {
		t.Fatalf("expected explicit route skill-map hashes: %#v", req)
	}
}
