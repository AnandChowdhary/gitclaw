package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type workflowPermissionContract struct {
	Job         string
	Permissions []string
}

type policyVerifyReport struct {
	Status                      string
	WorkflowPath                string
	WorkflowPresent             bool
	WorkflowBytes               int
	WorkflowLines               int
	WorkflowSHA                 string
	ExpectedJobs                int
	JobsPresent                 int
	ExpectedPermissions         int
	PermissionsPresent          int
	MissingPermissions          int
	UnexpectedWritePermissions  int
	BackupConcurrencyGroup      bool
	BackupConcurrencyCancelSafe bool
	PolicyOutputsHashed         int
	Findings                    []policyVerifyFinding
	Jobs                        []policyVerifyJob
}

type policyVerifyJob struct {
	Name             string
	Present          bool
	Expected         []string
	Actual           []string
	Matched          []string
	Missing          []string
	UnexpectedWrites []string
}

type policyVerifyFinding struct {
	Severity   string
	Code       string
	Job        string
	Permission string
	Detail     string
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
	if isPolicyRiskRequest(ev, cfg) {
		return renderPolicyRiskReport(ev, cfg, decision, transcript, repoContext, writeRequested, true)
	}
	if isPolicyVerifyRequest(ev, cfg) {
		return renderPolicyVerifyReport(ev, cfg, repoContext, true)
	}
	return renderPolicyReport(ev, cfg, decision, transcript, repoContext, writeRequested, true)
}

func RenderPolicyCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPolicyReport(Event{}, cfg, PreflightDecision{}, nil, repoContext, false, false)
}

func RenderPolicyVerifyCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPolicyVerifyReport(Event{}, cfg, repoContext, false)
}

func RenderPolicyRiskCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPolicyRiskReport(Event{}, cfg, PreflightDecision{}, nil, repoContext, false, false)
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

func renderPolicyVerifyReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildPolicyVerifyReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Policy Verify Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- policy_verify_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", "workflow-permissions-and-policy-surface")
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- model: `%s`\n", cfg.Model)
	fmt.Fprintf(&b, "- trusted_associations: `%d`\n", len(sortedAllowedAssociations(cfg)))
	fmt.Fprintf(&b, "- managed_labels: `%d`\n", len(managedPolicyLabels(cfg)))
	fmt.Fprintf(&b, "- workflow_path: `%s`\n", report.WorkflowPath)
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", report.WorkflowPresent)
	if report.WorkflowPresent {
		fmt.Fprintf(&b, "- workflow_bytes: `%d`\n", report.WorkflowBytes)
		fmt.Fprintf(&b, "- workflow_lines: `%d`\n", report.WorkflowLines)
		fmt.Fprintf(&b, "- workflow_sha256_12: `%s`\n", report.WorkflowSHA)
	}
	fmt.Fprintf(&b, "- expected_jobs: `%d`\n", report.ExpectedJobs)
	fmt.Fprintf(&b, "- jobs_present: `%d`\n", report.JobsPresent)
	fmt.Fprintf(&b, "- expected_permissions: `%d`\n", report.ExpectedPermissions)
	fmt.Fprintf(&b, "- permissions_present: `%d`\n", report.PermissionsPresent)
	fmt.Fprintf(&b, "- missing_permissions: `%d`\n", report.MissingPermissions)
	fmt.Fprintf(&b, "- unexpected_write_permissions: `%d`\n", report.UnexpectedWritePermissions)
	fmt.Fprintf(&b, "- backup_concurrency_group: `%t`\n", report.BackupConcurrencyGroup)
	fmt.Fprintf(&b, "- backup_concurrency_cancel_safe: `%t`\n", report.BackupConcurrencyCancelSafe)
	fmt.Fprintf(&b, "- policy_outputs_hashed: `%d`\n", report.PolicyOutputsHashed)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n\n", false)
	b.WriteString("This report verifies the checked-in GitClaw workflow permission surface against the read-only policy contract. Workflow bodies, issue bodies, comments, prompts, policy output bodies, and raw policy inputs are not included.\n\n")

	b.WriteString("### Workflow Permission Cards\n")
	for _, job := range report.Jobs {
		fmt.Fprintf(
			&b,
			"- job=`%s` present=`%t` expected=`%s` actual=`%s` matched=`%d` missing=`%s` unexpected_write=`%s`\n",
			job.Name,
			job.Present,
			inlineList(job.Expected),
			inlineList(job.Actual),
			len(job.Matched),
			inlineListOrNone(job.Missing),
			inlineListOrNone(job.UnexpectedWrites),
		)
	}

	b.WriteString("\n### Active Policy Output Trust Cards\n")
	writePolicyOutputTrustCards(&b, repoContext.ToolOutputs)

	b.WriteString("\n### Verification Findings\n")
	writePolicyVerifyFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildPolicyVerifyReport(cfg Config, repoContext RepoContext) policyVerifyReport {
	report := inspectPolicyWorkflowPermissions(cfg.Workdir)
	report.PolicyOutputsHashed = countPolicyOutputs(repoContext.ToolOutputs)
	return report
}

func inspectPolicyWorkflowPermissions(root string) policyVerifyReport {
	if root == "" {
		root = "."
	}
	report := policyVerifyReport{
		Status:       "ok",
		WorkflowPath: ".github/workflows/gitclaw.yml",
		ExpectedJobs: len(policyWorkflowPermissions),
	}
	for _, contract := range policyWorkflowPermissions {
		report.ExpectedPermissions += len(contract.Permissions)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		report.addPolicyVerifyFinding("error", "workflow_root_unreadable", "", "", "workflow root could not be resolved")
		return report
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(report.WorkflowPath)))
	if err != nil {
		report.addPolicyVerifyFinding("error", "workflow_missing", "", "", "main GitClaw workflow is missing")
		return report
	}
	text := string(body)
	report.WorkflowPresent = true
	report.WorkflowBytes = len(body)
	report.WorkflowLines = lineCount(text)
	report.WorkflowSHA = shortDocumentHash(text)

	for _, contract := range policyWorkflowPermissions {
		job := inspectPolicyWorkflowJob(text, contract)
		report.Jobs = append(report.Jobs, job)
		if job.Present {
			report.JobsPresent++
		}
		report.PermissionsPresent += len(job.Matched)
		report.MissingPermissions += len(job.Missing)
		report.UnexpectedWritePermissions += len(job.UnexpectedWrites)
		if !job.Present {
			report.addPolicyVerifyFinding("error", "workflow_job_missing", job.Name, "", "expected workflow job is missing")
		}
		for _, permission := range job.Missing {
			report.addPolicyVerifyFinding("error", "workflow_permission_missing", job.Name, permission, "expected workflow permission is missing")
		}
		for _, permission := range job.UnexpectedWrites {
			report.addPolicyVerifyFinding("error", "unexpected_write_permission", job.Name, permission, "workflow job grants an uncontracted write permission")
		}
	}
	if block, ok := workflowJobBlock(text, "backup"); ok {
		report.BackupConcurrencyGroup = workflowJobBlockContains(block, "group:", "gitclaw-backups-")
		report.BackupConcurrencyCancelSafe = workflowJobBlockContains(block, "cancel-in-progress:", "false")
	}
	if !report.BackupConcurrencyGroup {
		report.addPolicyVerifyFinding("error", "backup_concurrency_missing", "backup", "", "backup job must use a repo-wide concurrency group for the shared backup branch")
	}
	if !report.BackupConcurrencyCancelSafe {
		report.addPolicyVerifyFinding("error", "backup_concurrency_cancel_unsafe", "backup", "", "backup job must keep cancel-in-progress false to avoid dropping backups")
	}
	sort.Slice(report.Jobs, func(i, j int) bool { return report.Jobs[i].Name < report.Jobs[j].Name })
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Severity != report.Findings[j].Severity {
			return report.Findings[i].Severity < report.Findings[j].Severity
		}
		if report.Findings[i].Code != report.Findings[j].Code {
			return report.Findings[i].Code < report.Findings[j].Code
		}
		if report.Findings[i].Job != report.Findings[j].Job {
			return report.Findings[i].Job < report.Findings[j].Job
		}
		return report.Findings[i].Permission < report.Findings[j].Permission
	})
	if len(report.Findings) > 0 {
		report.Status = "error"
	}
	return report
}

func inspectPolicyWorkflowJob(text string, contract workflowPermissionContract) policyVerifyJob {
	job := policyVerifyJob{Name: contract.Job, Expected: append([]string(nil), contract.Permissions...)}
	block, ok := workflowJobBlock(text, contract.Job)
	if !ok {
		job.Missing = append([]string(nil), contract.Permissions...)
		return job
	}
	job.Present = true
	job.Actual = workflowJobPermissions(block)
	actual := map[string]bool{}
	for _, permission := range job.Actual {
		actual[permission] = true
	}
	for _, permission := range contract.Permissions {
		if actual[permission] {
			job.Matched = append(job.Matched, permission)
		} else {
			job.Missing = append(job.Missing, permission)
		}
	}
	expected := map[string]bool{}
	for _, permission := range contract.Permissions {
		expected[permission] = true
	}
	for _, permission := range job.Actual {
		if strings.HasSuffix(permission, ":write") && !expected[permission] {
			job.UnexpectedWrites = append(job.UnexpectedWrites, permission)
		}
	}
	sort.Strings(job.Actual)
	sort.Strings(job.Matched)
	sort.Strings(job.Missing)
	sort.Strings(job.UnexpectedWrites)
	return job
}

func workflowJobBlockContains(block []string, key, value string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.ToLower(strings.TrimSpace(value))
	for _, line := range block {
		trimmed := strings.ToLower(strings.TrimSpace(strings.SplitN(line, "#", 2)[0]))
		if strings.Contains(trimmed, key) && strings.Contains(trimmed, value) {
			return true
		}
	}
	return false
}

func workflowJobBlock(text, jobName string) ([]string, bool) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if leadingSpaces(line) == 2 && strings.TrimSpace(line) == jobName+":" {
			start = i
			break
		}
	}
	if start == -1 {
		return nil, false
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if leadingSpaces(lines[i]) == 2 && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			end = i
			break
		}
	}
	return lines[start:end], true
}

func workflowJobPermissions(block []string) []string {
	permissionsIndex := -1
	permissionsIndent := 0
	for i, line := range block {
		if strings.TrimSpace(line) == "permissions:" {
			permissionsIndex = i
			permissionsIndent = leadingSpaces(line)
			break
		}
	}
	if permissionsIndex == -1 {
		return nil
	}
	var permissions []string
	for _, line := range block[permissionsIndex+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := leadingSpaces(line)
		if indent <= permissionsIndent {
			break
		}
		if indent != permissionsIndent+2 || !strings.Contains(trimmed, ":") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.ToLower(strings.Trim(strings.TrimSpace(strings.SplitN(parts[1], "#", 2)[0]), `"'`))
		if key == "" || value == "" {
			continue
		}
		permissions = append(permissions, key+":"+value)
	}
	return uniqueSortedStrings(permissions)
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

func isPolicyVerifyRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/policy" && strings.EqualFold(fields[1], "verify")
}

func isPolicyRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/policy" && (strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
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

func writePolicyOutputTrustCards(b *strings.Builder, outputs []ToolOutput) {
	wrote := false
	for _, output := range outputs {
		if output.Name != "gitclaw.policy" {
			continue
		}
		wrote = true
		fmt.Fprintf(b, "- name=`%s` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s`\n",
			output.Name,
			shortDocumentHash(output.Input),
			len(output.Output),
			lineCount(output.Output),
			shortDocumentHash(output.Output),
		)
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func writePolicyVerifyFindings(b *strings.Builder, findings []policyVerifyFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` job=`%s` permission=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Job, finding.Permission, inlineCode(finding.Detail))
	}
}

func countPolicyOutputs(outputs []ToolOutput) int {
	count := 0
	for _, output := range outputs {
		if output.Name == "gitclaw.policy" && strings.TrimSpace(output.Output) != "" {
			count++
		}
	}
	return count
}

func inlineListOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return inlineList(values)
}

func (r *policyVerifyReport) addPolicyVerifyFinding(severity, code, job, permission, detail string) {
	r.Findings = append(r.Findings, policyVerifyFinding{
		Severity:   severity,
		Code:       code,
		Job:        job,
		Permission: permission,
		Detail:     detail,
	})
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
