package gitclaw

import (
	"fmt"
	"strings"
)

func IsSandboxReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/sandbox" || command == "/sandboxes" || command == "/exec-policy"
}

func RenderSandboxReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderSandboxReport(ev, cfg, repoContext, true)
}

func RenderSandboxCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSandboxReport(Event{}, cfg, repoContext, false)
}

func renderSandboxReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	toolValidation := ValidateTools(repoContext)
	policyVerify := BuildPolicyVerifyReport(cfg, repoContext)
	mutatingContracts := mutatingToolContractCount()
	requiredBins, missingBins := skillRequiredBinCounts(repoContext.SkillSummaries)

	var b strings.Builder
	b.WriteString("## GitClaw Sandbox Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
		fmt.Fprintf(&b, "- event_action: `%s`\n", ev.Action)
		fmt.Fprintf(&b, "- active_command: `%s`\n", activeSlashCommand(ev, cfg))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- sandbox_status: `%s`\n", sandboxStatus(toolValidation, policyVerify, mutatingContracts))
	fmt.Fprintf(&b, "- runtime_boundary: `%s`\n", "github-actions-ephemeral-runner")
	fmt.Fprintf(&b, "- sandbox_backend: `%s`\n", "github-actions")
	fmt.Fprintf(&b, "- host_exec_policy: `%s`\n", "deny")
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- write_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- approval_mode: `%s`\n", "not_applicable_no_exec_tool")
	fmt.Fprintf(&b, "- approval_store: `%s`\n", "not_configured")
	fmt.Fprintf(&b, "- elevated_mode_available: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_cli_auto_allow: `%t`\n", false)
	fmt.Fprintf(&b, "- inline_eval_policy: `%s`\n", "not_applicable_no_exec_tool")
	fmt.Fprintf(&b, "- network_egress_policy: `%s`\n", "github-actions-default")
	fmt.Fprintf(&b, "- available_tools: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", enabledToolCount(repoContext))
	fmt.Fprintf(&b, "- disabled_tools: `%d`\n", disabledToolCount(repoContext))
	fmt.Fprintf(&b, "- allowlist_blocked_tools: `%d`\n", allowlistBlockedToolCount(repoContext))
	fmt.Fprintf(&b, "- read_only_tool_contracts: `%d`\n", readOnlyToolContractCount())
	fmt.Fprintf(&b, "- metadata_only_tool_contracts: `%d`\n", metadataOnlyToolContractCount())
	fmt.Fprintf(&b, "- mutating_tool_contracts: `%d`\n", mutatingContracts)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", toolValidation.Status)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", toolValidation.Errors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", toolValidation.Warnings)
	fmt.Fprintf(&b, "- workflow_permission_status: `%s`\n", policyVerify.Status)
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", policyVerify.WorkflowPresent)
	fmt.Fprintf(&b, "- unexpected_write_permissions: `%d`\n", policyVerify.UnexpectedWritePermissions)
	fmt.Fprintf(&b, "- backup_write_permission_scope: `%s`\n", "backup-job-only")
	fmt.Fprintf(&b, "- skill_required_bins: `%d`\n", requiredBins)
	fmt.Fprintf(&b, "- skill_missing_bins: `%d`\n", missingBins)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workflow_included: `%t`\n", false)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report explains GitClaw's current execution boundary. It reports policy, workflow, tool, and skill metadata only; issue bodies, comments, prompts, workflow bodies, tool output bodies, and secrets are not included.\n\n")

	b.WriteString("### Execution Boundary\n")
	b.WriteString("- agent_runtime=`GitHub Actions job`\n")
	b.WriteString("- model_runtime=`GitHub Models or configured OpenAI-compatible endpoint`\n")
	b.WriteString("- shell_tool=`absent`\n")
	b.WriteString("- file_write_tool=`absent`\n")
	b.WriteString("- pull_request_tool=`absent`\n")
	b.WriteString("- backup_writer=`separate backup job after successful handle`\n")

	b.WriteString("\n### Tool Contracts\n")
	for _, contract := range toolReportContracts {
		enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		fmt.Fprintf(&b, "- name=`%s` mode=`%s` mutating=`%t` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` trigger=`%s`\n", contract.Name, contract.Mode, isMutatingToolContract(contract), enabled, disabled, blocked, contract.Trigger)
	}

	b.WriteString("\n### Active Tool Outputs\n")
	writeSandboxToolOutputs(&b, repoContext.ToolOutputs)

	b.WriteString("\n### Workflow Permission Boundary\n")
	if !policyVerify.WorkflowPresent {
		b.WriteString("- workflow_present=`false`\n")
	} else {
		for _, job := range policyVerify.Jobs {
			fmt.Fprintf(&b, "- job=`%s` present=`%t` expected=`%s` actual=`%s` unexpected_write=`%s`\n", job.Name, job.Present, inlineList(job.Expected), inlineList(job.Actual), inlineListOrNone(job.UnexpectedWrites))
		}
	}

	b.WriteString("\n### Sandbox Notes\n")
	b.WriteString("- host exec is denied because no shell/exec tool is exposed in GitClaw v1\n")
	b.WriteString("- tool outputs are deterministic prompt inputs, not arbitrary process execution\n")
	b.WriteString("- future host exec support must add explicit allowlists, approval storage, hard blocklists, and body-free audit cards before enabling execution\n")

	return strings.TrimSpace(b.String())
}

func sandboxStatus(tools ToolValidationReport, policy policyVerifyReport, mutatingContracts int) string {
	if tools.Errors > 0 || policy.Status == "error" || mutatingContracts > 0 {
		return "error"
	}
	if tools.Warnings > 0 || policy.Status == "warn" {
		return "warn"
	}
	return "locked_down"
}

func mutatingToolContractCount() int {
	count := 0
	for _, contract := range toolReportContracts {
		if isMutatingToolContract(contract) {
			count++
		}
	}
	return count
}

func readOnlyToolContractCount() int {
	count := 0
	for _, contract := range toolReportContracts {
		if contract.Mode == "read-only" {
			count++
		}
	}
	return count
}

func metadataOnlyToolContractCount() int {
	count := 0
	for _, contract := range toolReportContracts {
		if contract.Mode == "metadata-only" {
			count++
		}
	}
	return count
}

func skillRequiredBinCounts(skills []SkillSummary) (required int, missing int) {
	for _, skill := range skills {
		required += len(skill.RequiredBins)
		missing += len(skill.MissingBins)
	}
	return required, missing
}

func writeSandboxToolOutputs(b *strings.Builder, outputs []ToolOutput) {
	if len(outputs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, output := range outputs {
		fmt.Fprintf(b, "- name=`%s` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s`\n", output.Name, shortDocumentHash(output.Input), len(output.Output), lineCount(output.Output), shortDocumentHash(output.Output))
	}
}
