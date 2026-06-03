package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelBundleMapQueuesSafeSequenceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".gitclaw", "SKILLS", "repo-reader")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skill dir returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: repo-reader
description: Use read-only repository context.
---
Full skill body secret CHANNEL_BUNDLE_MAP_SKILL_BODY_SECRET.
`), 0o644); err != nil {
		t.Fatalf("WriteFile skill returned error: %v", err)
	}
	bundleDir := filepath.Join(root, ".gitclaw", "skill-bundles")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll bundle dir returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "repo-context.yaml"), []byte(`name: repo-context
description: Repository context questions.
skills:
  - repo-reader
  - missing-skill
instruction: |
  Prefer repository context before answering.
  Hidden bundle instruction secret CHANNEL_BUNDLE_MAP_INSTRUCTION_SECRET.
`), 0o644); err != nil {
		t.Fatalf("WriteFile bundle returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-bundle-map-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 903,
			"title": "GitClaw telegram thread chat-bundle-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-bundle-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90301,
			"body": "@gitclaw /channels bundle-map repo-context --message-id bundle-map-inbound-903 --notify-message-id bundle-map-notify-903 --map-id bundle-map-secret-903\nNote: Keep bundle usage reviewed\nDo not include this command hidden token in the receipt: CHANNEL_BUNDLE_MAP_COMMAND_SECRET.",
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
			Number: 903,
			Title:  "GitClaw telegram thread chat-bundle-map-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{903: {{
			ID: 90300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-bundle-map-123",
				MessageID: "bundle-map-inbound-903",
				Author:    "telegram",
				Body:      "Original mirrored bundle-map command with CHANNEL_BUNDLE_MAP_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{903: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel bundle map action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("bundle map should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[903]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="bundle-map-notify-903"`,
		"GitClaw channel bundle map.",
		"Requested bundle: repo-context",
		"Available bundles: 1",
		"Matched bundles: 1",
		"Selected bundles for this turn: 0",
		"Bundle skill refs: 2",
		"Resolved bundle skills: 1",
		"Missing bundle skills: 1",
		"Bundles with instruction: 1",
		"Bundle parse errors: 0",
		"Bundle risk findings: 0",
		"Matched bundle: repo-context",
		"Bundle skills: missing-skill, repo-reader",
		"Resolved skills: repo-reader",
		"Missing skills: missing-skill",
		"Instruction present: true",
		"Parse error present: false",
		"Bundle sequence:",
		"`/channels profile --message-id <id> --notify-message-id <id>`",
		"`/bundles info repo-context`",
		"`/bundles risk repo-context`",
		"`/channels skill-map repo-reader --message-id <id> --notify-message-id <id>`",
		"`/bundles rehearse repo-context --id <rehearsal-id>`",
		"`/channels propose-bundle --bundle-id <id> --message-id <id> --notify-message-id <id>`",
		"Note: Keep bundle usage reviewed",
		"Note hash: ",
		"Bundle map hash: ",
		"Bundle step hash: ",
		"Map source: current GitHub Actions checkout skill-bundle metadata.",
		"Full bundle bodies: not included.",
		"Bundle instructions: not included.",
		"Full skill bodies: not included.",
		"Skill install: not performed by this action.",
		"Skill update: not performed by this action.",
		"Bundle enablement: not performed by this action.",
		"Bundle YAML write: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Installer scripts: not run by this action.",
		"Bundle proposal issue creation: not performed by this action.",
		"Bundle rehearsal issue creation: not performed by this action.",
		"Skill proposal issue creation: not performed by this action.",
		"Model call: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("bundle-map notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BUNDLE_MAP_INGEST_SECRET", "CHANNEL_BUNDLE_MAP_COMMAND_SECRET", "CHANNEL_BUNDLE_MAP_SKILL_BODY_SECRET", "CHANNEL_BUNDLE_MAP_INSTRUCTION_SECRET", "bundle-map-secret-903"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("bundle-map notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Bundle Map Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels bundle-map`",
		"channel_bundle_map_status: `queued`",
		"bundle_map_mode: `provider-facing-bundle-sequence`",
		"notification_target_issue: `#903`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"bundle_map_id_sha256_12: `",
		"bundle_map_id_auto: `false`",
		"requested_bundle_sha256_12: `",
		"normalized_bundle_sha256_12: `",
		"requested_bundle_bytes: `12`",
		"requested_bundle_terms: `1`",
		"bundle_source: `positional`",
		"bundle_map_note_sha256_12: `",
		"bundle_map_note_bytes: `26`",
		"bundle_map_note_lines: `1`",
		"bundle_map_note_source: `trailing-note`",
		"bundle_map_step_count: `6`",
		"bundle_map_step_sha256_12: `",
		"bundle_map_snapshot_sha256_12: `",
		"available_bundles: `1`",
		"matched_bundles: `1`",
		"selected_bundles: `0`",
		"bundle_skill_refs: `2`",
		"resolved_bundle_skills: `1`",
		"missing_bundle_skills: `1`",
		"bundles_with_instruction: `1`",
		"bundles_with_parse_errors: `0`",
		"bundles_with_risk_findings: `0`",
		"selected_bundle_skill_refs: `2`",
		"selected_bundle_resolved_skills: `1`",
		"selected_bundle_missing_skills: `1`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"bundle_enable_allowed: `false`",
		"bundle_yaml_write_allowed: `false`",
		"registry_contact_allowed: `false`",
		"installer_scripts_run: `false`",
		"bundle_proposal_issue_created: `false`",
		"bundle_rehearsal_issue_created: `false`",
		"skill_proposal_issue_created: `false`",
		"model_call_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_bundle_map_id_included: `false`",
		"raw_requested_bundle_included: `false`",
		"raw_bundle_map_note_included: `false`",
		"raw_bundle_map_steps_included: `false`",
		"raw_bundle_names_included: `false`",
		"raw_bundle_paths_included: `false`",
		"raw_bundle_descriptions_included: `false`",
		"raw_bundle_instructions_included: `false`",
		"raw_bundle_bodies_included: `false`",
		"raw_skill_names_included: `false`",
		"raw_skill_paths_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_bundle_map_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel bundle map receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BUNDLE_MAP_INGEST_SECRET", "CHANNEL_BUNDLE_MAP_COMMAND_SECRET", "CHANNEL_BUNDLE_MAP_SKILL_BODY_SECRET", "CHANNEL_BUNDLE_MAP_INSTRUCTION_SECRET", "chat-bundle-map-123", "bundle-map-inbound-903", "bundle-map-notify-903", "bundle-map-secret-903", "repo-context", "repo-reader", "missing-skill", "Keep bundle usage reviewed"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel bundle map receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 903,
			"title": "GitClaw telegram thread chat-bundle-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-bundle-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90302,
			"body": "@gitclaw /channels bundle-path repo-context --message-id bundle-map-inbound-903 --notify-message-id bundle-map-notify-903 --map-id bundle-map-secret-903\nNote: Keep bundle usage reviewed\nDo not leak duplicate token CHANNEL_BUNDLE_MAP_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate bundle map created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[903]); got != 4 {
		t.Fatalf("duplicate bundle map posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[903])
	}
	duplicateReceipt := github.CommentsByIssue[903][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels bundle-path`",
		"channel_bundle_map_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"bundle_enable_allowed: `false`",
		"bundle_yaml_write_allowed: `false`",
		"bundle_proposal_issue_created: `false`",
		"bundle_rehearsal_issue_created: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate bundle map receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BUNDLE_MAP_DUPLICATE_SECRET", "chat-bundle-map-123", "bundle-map-inbound-903", "bundle-map-notify-903", "bundle-map-secret-903", "repo-context", "repo-reader", "missing-skill", "Keep bundle usage reviewed"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate bundle map receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBundleMapActionRequestParsesRouteAlias(t *testing.T) {
	repoContext := RepoContext{SkillBundles: []SkillBundleSummary{{
		Name:               "repo-context",
		Path:               ".gitclaw/skill-bundles/repo-context.yaml",
		Skills:             []string{"repo-reader"},
		ResolvedSkills:     []string{"repo-reader"},
		InstructionPresent: true,
		InstructionSHA:     "abc123",
	}}}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel bundle map"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel bundle-runbook --route team-demo --bundle repo-context --message-id source-1 --notify-message-id notify-1 --map-id map-1 --note reviewed-only`,
		},
	}
	req, err := BuildChannelBundleMapActionRequest(ev, DefaultConfig(), repoContext)
	if err != nil {
		t.Fatalf("BuildChannelBundleMapActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "bundle-runbook" || req.Options.Route != "team-demo" || req.Options.RequestedBundle != "repo-context" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MapID != "map-1" || req.Options.Note != "reviewed-only" {
		t.Fatalf("unexpected channel bundle map parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMapID || req.RequestedRouteHash == "" || req.MapIDHash == "" || req.RequestedBundleHash == "" || req.StepSHA == "" || req.SnapshotSHA == "" {
		t.Fatalf("expected explicit route bundle-map hashes: %#v", req)
	}
}
