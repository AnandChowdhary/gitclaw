package gitclaw

import (
	"strings"
	"testing"
)

func TestValidateToolSurfaceReportsProblemsWithoutBodies(t *testing.T) {
	contracts := []toolContract{
		{Name: "gitclaw.list_files", Mode: "read-only", Trigger: "always"},
		{Name: "gitclaw.list_files", Mode: "read-only", Trigger: "duplicate"},
		{Name: "gitclaw.write_file", Mode: "write", Trigger: "never"},
	}
	report := ValidateToolSurface(contracts, RepoContext{ToolOutputs: []ToolOutput{
		{Name: "gitclaw.unknown", Input: "SECRET_INPUT_TOKEN", Output: "SECRET_OUTPUT_TOKEN"},
		{Name: "gitclaw.list_files", Input: ".", Output: ""},
	}})
	if report.Status != "error" || report.Errors != 3 || report.Warnings != 2 || report.UnknownOutputs != 1 || report.UnsafeContracts != 1 || report.MissingGuidance != 1 || report.DuplicateContracts != 1 {
		t.Fatalf("unexpected validation report: %#v", report)
	}
	body := RenderToolsValidationReport(RepoContext{ToolOutputs: []ToolOutput{
		{Name: "gitclaw.unknown", Input: "SECRET_INPUT_TOKEN", Output: "SECRET_OUTPUT_TOKEN"},
	}})
	for _, want := range []string{
		"GitClaw Tools Validate Report",
		"scope: `local-cli`",
		"tool_validation_status: `error`",
		"tool_validation_errors: `1`",
		"tool_validation_warnings: `1`",
		"tool_contracts: `5`",
		"tool_active_outputs: `1`",
		"tool_guidance_files: `0`",
		"tool_unknown_outputs: `1`",
		"tool_missing_guidance: `1`",
		"tool_duplicate_contracts: `0`",
		"code=`unknown_tool_output`",
		"code=`missing_tool_guidance`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("validation report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SECRET_OUTPUT_TOKEN") || strings.Contains(body, "SECRET_INPUT_TOKEN") {
		t.Fatalf("validation report leaked tool body/input token:\n%s", body)
	}
}

func TestValidateToolsAcceptsCurrentToolShape(t *testing.T) {
	searchOutput := "[query \"fixture\"]\n" + strings.Repeat("docs/search-fixture.md:1:fixture\n", maxSearchMatches)
	report := ValidateTools(RepoContext{
		Documents: []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "Read-only tools."}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.list_files", Input: ".", Output: "go.mod\nREADME.md"},
			{Name: "gitclaw.search_files", Input: "fixture", Output: searchOutput},
			{Name: "gitclaw.read_file", Input: "go.mod", Output: "module github.com/AnandChowdhary/gitclaw"},
			{Name: "gitclaw.skill_index", Input: ".gitclaw/SKILLS", Output: "repo-reader sha256_12=abcdef123456"},
			{Name: "gitclaw.policy", Input: "write-request", Output: "Current GitClaw mode is read-only."},
		},
	})
	if report.Status != "ok" || report.Errors != 0 || report.Warnings != 0 || report.Contracts != 5 || report.ActiveOutputs != 5 || report.GuidanceFiles != 1 {
		t.Fatalf("unexpected validation report: %#v", report)
	}
	body := RenderToolsValidationReport(RepoContext{
		Documents:   []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "Read-only tools."}},
		ToolOutputs: []ToolOutput{{Name: "gitclaw.list_files", Input: ".", Output: "go.mod"}},
	})
	for _, want := range []string{"scope: `local-cli`", "tool_validation_status: `ok`", "tool_validation_errors: `0`", "tool_validation_warnings: `0`", "tool_missing_guidance: `0`", "tool_duplicate_contracts: `0`", "- none"} {
		if !strings.Contains(body, want) {
			t.Fatalf("validation report missing %q:\n%s", want, body)
		}
	}
}

func TestRenderToolVerifyReportShowsTrustEnvelopeWithoutBodies(t *testing.T) {
	repoContext := RepoContext{
		Documents: []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "TOOL_VERIFY_GUIDANCE_SECRET: read-only tools."}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.list_files", Input: ". TOOL_VERIFY_INPUT_SECRET", Output: "go.mod\nREADME.md\nTOOL_VERIFY_LIST_OUTPUT_SECRET"},
			{Name: "gitclaw.read_file", Input: "go.mod TOOL_VERIFY_FILE_INPUT_SECRET", Output: "module github.com/AnandChowdhary/gitclaw\nTOOL_VERIFY_READ_OUTPUT_SECRET"},
		},
	}
	body := RenderToolVerifyReport(repoContext)
	for _, want := range []string{
		"GitClaw Tools Verify Report",
		"scope: `local-cli`",
		"tool_verify_status: `ok`",
		"verification_scope: `deterministic-tool-contracts`",
		"available_tools: `5`",
		"enabled_tools: `5`",
		"disabled_tools: `0`",
		"allowlist_blocked_tools: `0`",
		"read_only_contracts: `3`",
		"metadata_only_contracts: `2`",
		"mutating_contracts: `0`",
		"active_tool_outputs: `2`",
		"known_tool_outputs: `2`",
		"unknown_tool_outputs: `0`",
		"tool_guidance_files: `1`",
		"repo_local_guidance_files: `1`",
		"unknown_guidance_files: `0`",
		"tool_outputs_hashed: `2`",
		"tool_inputs_hashed: `2`",
		"registry_verification: `not_configured`",
		"runtime_permission_verification: `static_contracts_only`",
		"shell_execution_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_inputs_included: `false`",
		"llm_e2e_required_after_tool_verify_change: `true`",
		"tool_validation_status: `ok`",
		"tool_validation_errors: `0`",
		"tool_validation_warnings: `0`",
		"### Trust Cards",
		"kind=`contract` name=`gitclaw.list_files` source=`builtin-gitclaw` enabled=`true` disabled_by_config=`false` blocked_by_allowlist=`false` mode=`read-only` mutating=`false`",
		"kind=`contract` name=`gitclaw.policy` source=`builtin-gitclaw` enabled=`true` disabled_by_config=`false` blocked_by_allowlist=`false` mode=`metadata-only` mutating=`false`",
		"kind=`guidance` path=`.gitclaw/TOOLS.md` source=`repo-local`",
		"kind=`active-output` name=`gitclaw.read_file` contract_known=`true` input_sha256_12=",
		"output_sha256_12=",
		"### Verification Findings",
		"code=`tool_registry_verification_not_configured`",
		"code=`runtime_permission_verification_static_only`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tool verify report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_VERIFY_GUIDANCE_SECRET", "TOOL_VERIFY_INPUT_SECRET", "TOOL_VERIFY_FILE_INPUT_SECRET", "TOOL_VERIFY_LIST_OUTPUT_SECRET", "TOOL_VERIFY_READ_OUTPUT_SECRET", "module github.com/AnandChowdhary/gitclaw", "go.mod"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tool verify report leaked body/input token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderToolSearchReportFindsContractsAndOutputsWithoutBodies(t *testing.T) {
	repoContext := RepoContext{
		Documents: []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "TOOL_SEARCH_GUIDANCE_SECRET"}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.read_file", Input: "go.mod TOOL_SEARCH_INPUT_SECRET", Output: "TOOL_SEARCH_OUTPUT_SECRET"},
			{Name: "gitclaw.list_files", Input: ".", Output: "go.mod\n"},
		},
	}
	body := RenderToolSearchReport(Event{}, repoContext, "read_file TOOL_SEARCH_QUERY_SECRET", 5)
	for _, want := range []string{
		"GitClaw Tools Search Report",
		"scope: `local-cli`",
		"tool_search_status: `ok`",
		"query_sha256_12:",
		"query_terms:",
		"max_results: `5`",
		"available_tools: `5`",
		"active_tool_outputs: `2`",
		"matched_contracts: `1`",
		"matched_outputs: `1`",
		"results_returned: `2`",
		"raw_bodies_included: `false`",
		"raw_inputs_included: `false`",
		"kind=`contract` name=`gitclaw.read_file`",
		"mode=`read-only`",
		"trigger=`explicit repository-relative path`",
		"kind=`active-output` name=`gitclaw.read_file`",
		"input_sha256_12=",
		"output_sha256_12=",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tool search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_SEARCH_GUIDANCE_SECRET", "TOOL_SEARCH_INPUT_SECRET", "TOOL_SEARCH_OUTPUT_SECRET", "TOOL_SEARCH_QUERY_SECRET", "read_file TOOL_SEARCH_QUERY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tool search report leaked body/input/query token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderToolCatalogReportShowsCompactProgressiveDisclosureWithoutBodies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workdir = t.TempDir()
	repoContext := RepoContext{
		Documents: []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "TOOL_CATALOG_GUIDANCE_SECRET: read-only tools."}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.list_files", Input: ". TOOL_CATALOG_LIST_INPUT_SECRET", Output: "go.mod\nREADME.md\nTOOL_CATALOG_LIST_OUTPUT_SECRET"},
			{Name: "gitclaw.search_files", Input: "catalog TOOL_CATALOG_SEARCH_INPUT_SECRET", Output: "docs/search-fixture.md:1:GITCLAW_TOOLS_CATALOG_CONTEXT_V1 TOOL_CATALOG_SEARCH_OUTPUT_SECRET"},
		},
	}
	body := RenderToolCatalogCLIReport(cfg, repoContext)
	for _, want := range []string{
		"GitClaw Tools Catalog Report",
		"scope: `local-cli`",
		"tool_catalog_status: `ok`",
		"catalog_strategy: `compact-progressive-disclosure`",
		"catalog_scope: `deterministic-tools-toolsets-mcp`",
		"cataloged_entries: `5`",
		"direct_core_entries: `5`",
		"enabled_core_entries: `5`",
		"deferrable_candidate_entries: `0`",
		"toolset_catalog_entries: `0`",
		"mcp_catalog_entries: `0`",
		"planned_direct_entries: `5`",
		"planned_deferred_entries: `0`",
		"candidate_bridge_tools: `3`",
		"planned_bridge_tools: `0`",
		"activation_decision: `direct`",
		"activation_reason: `no_deferrable_catalog_entries`",
		"available_tools: `5`",
		"enabled_tools: `5`",
		"disabled_tools: `0`",
		"allowlist_blocked_tools: `0`",
		"active_tool_outputs: `2`",
		"model_callable_structured_tools: `false`",
		"tool_search_bridge_runtime_enabled: `false`",
		"schema_describe_required: `false`",
		"dynamic_mcp_discovery_allowed: `false`",
		"mcp_server_launch_allowed: `false`",
		"toolset_activation_supported: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_toolset_bodies_included: `false`",
		"raw_toolset_instructions_included: `false`",
		"raw_mcp_bodies_included: `false`",
		"raw_mcp_command_args_included: `false`",
		"raw_inputs_included: `false`",
		"raw_outputs_included: `false`",
		"llm_e2e_required_after_tool_catalog_change: `true`",
		"tool_validation_status: `ok`",
		"tool_risk_status: `ok`",
		"### Catalog Entries",
		"kind=`builtin-contract` name=`gitclaw.list_files` source=`builtin-gitclaw` path=`builtin` mode=`read-only` enabled=`true` direct_core=`true` deferrable_candidate=`false` planned_deferred=`false` catalog_mode=`direct-core` schema_visibility=`direct-contract` active_outputs=`1`",
		"reason_codes=`active_outputs, builtin_contract, direct_core, enabled, not_deferrable, planned_direct`",
		"kind=`builtin-contract` name=`gitclaw.policy`",
		"schema_visibility=`direct-contract` active_outputs=`0`",
		"reason_codes=`builtin_contract, direct_core, enabled, no_active_outputs, not_deferrable, planned_direct`",
		"### Catalog Gates",
		"validation_gate=`pass`",
		"risk_gate=`pass`",
		"activation_gate=`direct`",
		"tool_search_bridge_gate=`disabled`",
		"structured_tool_gate=`disabled`",
		"mcp_runtime_gate=`disabled`",
		"toolset_activation_gate=`disabled`",
		"schema_body_gate=`sha256_12`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tool catalog report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_CATALOG_GUIDANCE_SECRET", "TOOL_CATALOG_LIST_INPUT_SECRET", "TOOL_CATALOG_SEARCH_INPUT_SECRET", "TOOL_CATALOG_LIST_OUTPUT_SECRET", "TOOL_CATALOG_SEARCH_OUTPUT_SECRET", "GITCLAW_TOOLS_CATALOG_CONTEXT_V1", "go.mod"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tool catalog report leaked body/input/output token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderToolInfoReportShowsOneContractWithoutBodies(t *testing.T) {
	repoContext := RepoContext{
		Documents: []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "TOOL_INFO_GUIDANCE_SECRET"}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.read_file", Input: "go.mod TOOL_INFO_INPUT_SECRET", Output: "module github.com/AnandChowdhary/gitclaw\nTOOL_INFO_OUTPUT_SECRET"},
			{Name: "gitclaw.list_files", Input: ".", Output: "go.mod\nREADME.md"},
		},
	}
	body := RenderToolInfoCLIReport(repoContext, "read_file")
	for _, want := range []string{
		"GitClaw Tool Info Report",
		"scope: `local-cli`",
		"requested_tool: `read_file`",
		"tool_info_status: `ok`",
		"available_tools: `5`",
		"matched_tools: `1`",
		"active_outputs_for_tool: `1`",
		"run_mode: `read-only`",
		"raw_bodies_included: `false`",
		"raw_inputs_included: `false`",
		"### Matches",
		"name=`gitclaw.read_file` source=`builtin-gitclaw` enabled=`true` disabled_by_config=`false` blocked_by_allowlist=`false` mode=`read-only` mutating=`false`",
		"trigger=`explicit repository-relative path`",
		"active_outputs=`1`",
		"### Active Outputs For Tool",
		"name=`gitclaw.read_file` contract_known=`true` input_sha256_12=",
		"output_bytes=",
		"output_sha256_12=",
		"### Validation For Matches",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tool info report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_INFO_GUIDANCE_SECRET", "TOOL_INFO_INPUT_SECRET", "TOOL_INFO_OUTPUT_SECRET", "module github.com/AnandChowdhary/gitclaw", "go.mod TOOL_INFO_INPUT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tool info report leaked body/input token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderToolRunPlanReportShowsOneContractWithoutBodies(t *testing.T) {
	repoContext := RepoContext{
		Documents: []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "TOOL_RUN_PLAN_GUIDANCE_SECRET"}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.search_files", Input: "bounded phrase TOOL_RUN_PLAN_INPUT_SECRET", Output: "docs/search-fixture.md:7:GITCLAW_SEARCH_CONTEXT_V1 TOOL_RUN_PLAN_OUTPUT_SECRET"},
			{Name: "gitclaw.list_files", Input: ".", Output: "go.mod\nREADME.md"},
		},
	}
	body := RenderToolRunPlanCLIReport(repoContext, "search_files")
	for _, want := range []string{
		"GitClaw Tool Run Plan Report",
		"scope: `local-cli`",
		"tool_run_plan_status: `ok`",
		"requested_tool_sha256_12:",
		"normalized_tool: `gitclaw.search_files`",
		"matched_tools: `1`",
		"active_outputs_for_tool: `1`",
		"tool_enabled: `true`",
		"disabled_by_config: `false`",
		"blocked_by_allowlist: `false`",
		"tool_mode: `read-only`",
		"tool_trigger: `explicit quoted phrase or identifier`",
		"mutating_contract: `false`",
		"run_mode: `read-only`",
		"model_call_required: `false`",
		"shell_execution_allowed: `false`",
		"network_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_tool_name_included: `false`",
		"raw_inputs_included: `false`",
		"raw_outputs_included: `false`",
		"tool_validation_status: `ok`",
		"### Contract",
		"name=`gitclaw.search_files` source=`builtin-gitclaw` enabled=`true`",
		"active_outputs=`1`",
		"### Active Outputs For Tool",
		"name=`gitclaw.search_files` contract_known=`true` input_sha256_12=",
		"output_sha256_12=",
		"### Review Steps",
		"Use a live GitHub Models conversation E2E",
		"### Findings",
		"code=`deterministic_tool_contract`",
		"code=`shell_execution_disabled`",
		"code=`repository_mutation_disabled`",
		"code=`read_only_or_metadata_only`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tool run-plan report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_RUN_PLAN_GUIDANCE_SECRET", "TOOL_RUN_PLAN_INPUT_SECRET", "TOOL_RUN_PLAN_OUTPUT_SECRET", "GITCLAW_SEARCH_CONTEXT_V1", "bounded phrase"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tool run-plan report leaked body/input token %q:\n%s", leaked, body)
		}
	}
}
