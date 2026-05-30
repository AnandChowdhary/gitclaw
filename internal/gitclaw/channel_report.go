package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const channelIngestWorkflowPath = ".github/workflows/gitclaw-channel-ingest.yml"

var channelReportProviders = []string{
	"telegram",
	"slack",
	"generic",
}

type channelSurface struct {
	Workflow channelWorkflow
}

type channelWorkflow struct {
	Path             string
	Present          bool
	Bytes            int
	Lines            int
	SHA              string
	WorkflowDispatch bool
	ActionsWrite     bool
	IssuesWrite      bool
	Inputs           int
}

func IsChannelReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/channel" || command == "/channels"
}

func RenderChannelReport(ev Event, cfg Config, comments []Comment) string {
	if isChannelVerifyRequest(ev, cfg) {
		return renderChannelVerifyReport(ev, cfg, comments, true)
	}
	return renderChannelReport(ev, cfg, comments, true)
}

func RenderChannelCLIReport(cfg Config) string {
	return renderChannelReport(Event{}, cfg, nil, false)
}

func RenderChannelVerifyCLIReport(cfg Config) string {
	return renderChannelVerifyReport(Event{}, cfg, nil, false)
}

func renderChannelReport(ev Event, cfg Config, comments []Comment, includeIssue bool) string {
	surface := inspectChannelSurface(cfg.Workdir)
	channelMessages := countChannelMessages(comments)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- channel_label: `%s`\n", cfg.ChannelLabel)
	fmt.Fprintf(&b, "- trigger_label: `%s`\n", cfg.TriggerLabel)
	fmt.Fprintf(&b, "- workflow_path: `%s`\n", channelIngestWorkflowPath)
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", surface.Workflow.Present)
	fmt.Fprintf(&b, "- workflow_dispatch_trigger: `%t`\n", surface.Workflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- permissions_actions_write: `%t`\n", surface.Workflow.ActionsWrite)
	fmt.Fprintf(&b, "- permissions_issues_write: `%t`\n", surface.Workflow.IssuesWrite)
	fmt.Fprintf(&b, "- workflow_inputs: `%d`\n", surface.Workflow.Inputs)
	if includeIssue {
		fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- channel_message_comments_now: `%d`\n", channelMessages)
	}
	fmt.Fprintf(&b, "- supported_providers: `%s`\n", strings.Join(channelReportProviders, ", "))
	fmt.Fprintf(&b, "- wake_strategy: `%s`\n", "workflow_dispatch")
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Channel ingress mirrors external messages into canonical GitHub issues, then wakes the normal handler with `workflow_dispatch`. Channel message bodies, issue bodies, and tokens are not included in this report.\n\n")

	b.WriteString("### Workflow\n")
	if !surface.Workflow.Present {
		b.WriteString("- none\n")
	} else {
		fmt.Fprintf(
			&b,
			"- `%s` bytes=`%d` lines=`%d` workflow_dispatch=`%t` actions_write=`%t` issues_write=`%t` inputs=`%d` sha256_12=`%s`\n",
			surface.Workflow.Path,
			surface.Workflow.Bytes,
			surface.Workflow.Lines,
			surface.Workflow.WorkflowDispatch,
			surface.Workflow.ActionsWrite,
			surface.Workflow.IssuesWrite,
			surface.Workflow.Inputs,
			surface.Workflow.SHA,
		)
	}

	b.WriteString("\n### Providers\n")
	for _, provider := range channelReportProviders {
		fmt.Fprintf(&b, "- `%s`\n", provider)
	}

	b.WriteString("\n### Ingest Contract\n")
	b.WriteString("- `gitclaw channel-ingest --channel <provider> --thread-id <thread> --message-id <message> --body <text>`\n")
	b.WriteString("- `gitclaw channel-state --channel <provider> --account-id <account> --offset <offset>` stores durable provider offsets as hashes\n")
	b.WriteString("- one canonical issue per `channel + thread_id`\n")
	b.WriteString("- one mirrored comment per `channel + message_id`\n")
	b.WriteString("- dispatch id: `<channel>-<message_id>`\n")

	return strings.TrimSpace(b.String())
}

func renderChannelVerifyReport(ev Event, cfg Config, comments []Comment, includeIssue bool) string {
	surface := inspectChannelSurface(cfg.Workdir)
	findings := channelVerifyFindings(surface)
	status := "ok"
	if len(findings) > 0 {
		status = "warn"
	}
	channelMessages := countChannelMessages(comments)

	var b strings.Builder
	b.WriteString("## GitClaw Channel Verify Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- channel_verify_status: `%s`\n", status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", "workflow_dispatch_channel_bridge")
	fmt.Fprintf(&b, "- channel_label: `%s`\n", cfg.ChannelLabel)
	fmt.Fprintf(&b, "- trigger_label: `%s`\n", cfg.TriggerLabel)
	fmt.Fprintf(&b, "- workflow_path: `%s`\n", channelIngestWorkflowPath)
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", surface.Workflow.Present)
	fmt.Fprintf(&b, "- workflow_dispatch_trigger: `%t`\n", surface.Workflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- permissions_actions_write: `%t`\n", surface.Workflow.ActionsWrite)
	fmt.Fprintf(&b, "- permissions_issues_write: `%t`\n", surface.Workflow.IssuesWrite)
	fmt.Fprintf(&b, "- workflow_inputs: `%d`\n", surface.Workflow.Inputs)
	fmt.Fprintf(&b, "- required_workflow_inputs: `%d`\n", 5)
	fmt.Fprintf(&b, "- supported_providers: `%s`\n", strings.Join(channelReportProviders, ", "))
	fmt.Fprintf(&b, "- wake_strategy: `%s`\n", "workflow_dispatch")
	if includeIssue {
		fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- channel_message_comments_now: `%d`\n", channelMessages)
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n\n", false)

	b.WriteString("This report verifies the GitHub-native channel bridge surface for Telegram/Slack-style ingress. It checks the workflow-dispatch bridge and permissions only; channel message bodies, issue bodies, command bodies, and workflow bodies are not included.\n\n")

	b.WriteString("### Verification Findings\n")
	if len(findings) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, finding := range findings {
			fmt.Fprintf(&b, "- severity=`warn` code=`%s`\n", finding)
		}
	}

	b.WriteString("\n### Required Bridge Shape\n")
	b.WriteString("- workflow has `workflow_dispatch`\n")
	b.WriteString("- workflow can write Actions dispatches with `actions: write`\n")
	b.WriteString("- workflow can create/update GitHub issues with `issues: write`\n")
	b.WriteString("- workflow accepts `channel`, `thread_id`, `message_id`, `author`, and `body` inputs\n")
	b.WriteString("- downstream wakeup uses dispatch id `<channel>-<message_id>`\n")

	return strings.TrimSpace(b.String())
}

func channelVerifyFindings(surface channelSurface) []string {
	var findings []string
	if !surface.Workflow.Present {
		return []string{"channel_ingest_workflow_missing"}
	}
	if !surface.Workflow.WorkflowDispatch {
		findings = append(findings, "workflow_dispatch_missing")
	}
	if !surface.Workflow.ActionsWrite {
		findings = append(findings, "actions_write_permission_missing")
	}
	if !surface.Workflow.IssuesWrite {
		findings = append(findings, "issues_write_permission_missing")
	}
	if surface.Workflow.Inputs < 5 {
		findings = append(findings, "required_workflow_inputs_missing")
	}
	return findings
}

func isChannelVerifyRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && (fields[0] == "/channel" || fields[0] == "/channels") && strings.EqualFold(fields[1], "verify")
}

func inspectChannelSurface(root string) channelSurface {
	if root == "" {
		root = "."
	}
	surface := channelSurface{
		Workflow: channelWorkflow{Path: channelIngestWorkflowPath},
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(channelIngestWorkflowPath)))
	if err != nil {
		return surface
	}
	text := string(body)
	surface.Workflow.Present = true
	surface.Workflow.Bytes = len(body)
	surface.Workflow.Lines = lineCount(text)
	surface.Workflow.SHA = shortDocumentHash(text)
	surface.Workflow.WorkflowDispatch = strings.Contains(text, "workflow_dispatch:")
	surface.Workflow.ActionsWrite = strings.Contains(text, "actions: write")
	surface.Workflow.IssuesWrite = strings.Contains(text, "issues: write")
	surface.Workflow.Inputs = countWorkflowInputKeys(text)
	return surface
}

func countChannelMessages(comments []Comment) int {
	count := 0
	for _, comment := range comments {
		if HasChannelMessageMarker(comment.Body) {
			count++
		}
	}
	return count
}

func countWorkflowInputKeys(text string) int {
	inInputs := false
	count := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "inputs:" {
			inInputs = true
			continue
		}
		if !inInputs {
			continue
		}
		if strings.HasPrefix(line, "      ") && !strings.HasPrefix(line, "        ") && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			count++
			continue
		}
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") && trimmed != "" {
			break
		}
	}
	return count
}
