package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type workflowPermissionContract struct {
	Job         string
	Permissions []string
}

var policyWorkflowPermissions = []workflowPermissionContract{
	{Job: "preflight", Permissions: []string{"contents:read", "issues:read"}},
	{Job: "handle", Permissions: []string{"contents:read", "issues:write", "models:read"}},
	{Job: "backup", Permissions: []string{"contents:write", "issues:read"}},
}

func IsPolicyReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/policy"
}

func RenderPolicyReport(ev Event, cfg Config, decision PreflightDecision, transcript []TranscriptMessage, repoContext RepoContext, writeRequested bool) string {
	return renderPolicyReport(ev, cfg, decision, transcript, repoContext, writeRequested, true)
}

func RenderPolicyCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPolicyReport(Event{}, cfg, PreflightDecision{}, nil, repoContext, false, false)
}

func renderPolicyReport(ev Event, cfg Config, decision PreflightDecision, transcript []TranscriptMessage, repoContext RepoContext, writeRequested bool, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Policy Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
		fmt.Fprintf(&b, "- preflight_allowed: `%t`\n", decision.Allowed)
		fmt.Fprintf(&b, "- preflight_code: `%s`\n", decision.Code)
		fmt.Fprintf(&b, "- actor_association: `%s`\n", actorAssociation(ev))
		fmt.Fprintf(&b, "- actor_trusted: `%t`\n", trustedAssociation(actorAssociation(ev), cfg))
		fmt.Fprintf(&b, "- triggered: `%t`\n", triggered(ev, cfg))
		fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.DisabledLabel))
		fmt.Fprintf(&b, "- pull_request: `%t`\n", ev.Issue.IsPullRequest)
		fmt.Fprintf(&b, "- write_request_detected: `%t`\n", writeRequested)
		fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- model: `%s`\n\n", cfg.Model)
	b.WriteString("Issue and comment bodies are not included in this report.\n\n")

	b.WriteString("### Trusted Associations\n")
	for _, association := range sortedAllowedAssociations(cfg) {
		fmt.Fprintf(&b, "- `%s`\n", association)
	}

	b.WriteString("\n### Managed Labels\n")
	for _, label := range managedPolicyLabels(cfg) {
		fmt.Fprintf(&b, "- `%s`\n", label)
	}

	if includeIssue {
		b.WriteString("\n### Event Labels\n")
		writeStringList(&b, sortedStrings(ev.Issue.Labels))
	}

	b.WriteString("\n### Expected Workflow Permissions\n")
	for _, contract := range policyWorkflowPermissions {
		fmt.Fprintf(&b, "- `%s`: `%s`\n", contract.Job, strings.Join(contract.Permissions, "`, `"))
	}

	b.WriteString("\n### Active Policy Outputs\n")
	writePolicyOutputList(&b, repoContext.ToolOutputs)

	return strings.TrimSpace(b.String())
}

func sortedAllowedAssociations(cfg Config) []string {
	var associations []string
	for association, allowed := range cfg.AllowedAssociations {
		if allowed {
			associations = append(associations, strings.ToUpper(association))
		}
	}
	return sortedStrings(associations)
}

func managedPolicyLabels(cfg Config) []string {
	return []string{
		cfg.TriggerLabel,
		cfg.RunningLabel,
		cfg.DoneLabel,
		cfg.ErrorLabel,
		cfg.DisabledLabel,
		cfg.WriteRequestedLabel,
		cfg.HeartbeatLabel,
		cfg.ChannelLabel,
		cfg.ProactiveLabel,
	}
}

func writePolicyOutputList(b *strings.Builder, outputs []ToolOutput) {
	wrote := false
	for _, output := range outputs {
		if output.Name != "gitclaw.policy" {
			continue
		}
		wrote = true
		fmt.Fprintf(b, "- `%s` input=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", output.Name, inlineCode(output.Input), len(output.Output), lineCount(output.Output), shortDocumentHash(output.Output))
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func writeStringList(b *strings.Builder, values []string) {
	if len(values) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, value := range values {
		fmt.Fprintf(b, "- `%s`\n", value)
	}
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
