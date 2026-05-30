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
