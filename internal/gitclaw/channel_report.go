package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const channelIngestWorkflowPath = ".github/workflows/gitclaw-channel-ingest.yml"
const channelStateWorkflowPath = ".github/workflows/gitclaw-channel-state.yml"
const channelGatewayWorkflowPath = ".github/workflows/gitclaw-channel-gateway.yml"
const channelDeliveryWorkflowPath = ".github/workflows/gitclaw-channel-delivery.yml"

var channelReportProviders = []string{
	"telegram",
	"slack",
	"generic",
}

type channelProviderInfo struct {
	Name             string
	IngressStrategy  string
	GatewayStrategy  string
	OffsetKey        string
	ThreadKey        string
	MessageKey       string
	OutboundDelivery string
	RequiredSecrets  []string
}

var channelProviderCatalog = []channelProviderInfo{
	{
		Name:             "telegram",
		IngressStrategy:  "getUpdates polling or long-running gateway lease",
		GatewayStrategy:  "self-renewing GitHub Actions gateway with durable offset hashes",
		OffsetKey:        "update_id",
		ThreadKey:        "chat_id",
		MessageKey:       "update_id or message_id",
		OutboundDelivery: "sendMessage then channel-delivery receipt",
		RequiredSecrets:  []string{"TELEGRAM_BOT_TOKEN"},
	},
	{
		Name:             "slack",
		IngressStrategy:  "Socket Mode gateway or low-volume conversations polling",
		GatewayStrategy:  "self-renewing GitHub Actions gateway with durable event hashes",
		OffsetKey:        "event_id or channel timestamp",
		ThreadKey:        "channel_id + thread_ts",
		MessageKey:       "event_id or message timestamp",
		OutboundDelivery: "chat.postMessage then channel-delivery receipt",
		RequiredSecrets:  []string{"SLACK_APP_TOKEN", "SLACK_BOT_TOKEN"},
	},
	{
		Name:             "generic",
		IngressStrategy:  "manual workflow_dispatch or tiny external dispatcher",
		GatewayStrategy:  "optional; external caller can invoke channel-ingest directly",
		OffsetKey:        "external offset",
		ThreadKey:        "thread_id",
		MessageKey:       "message_id",
		OutboundDelivery: "external sender then channel-delivery receipt",
		RequiredSecrets:  nil,
	},
}

type channelSurface struct {
	IngestWorkflow   channelWorkflow
	StateWorkflow    channelWorkflow
	GatewayWorkflow  channelWorkflow
	DeliveryWorkflow channelWorkflow
}

type channelWorkflow struct {
	Path             string
	Present          bool
	Body             string
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
	if provider := requestedChannelInfoProvider(ev, cfg); provider != "" {
		return renderChannelInfoReport(ev, cfg, comments, provider, true)
	}
	if isChannelRiskRequest(ev, cfg) {
		return renderChannelRiskReport(ev, cfg, comments, true)
	}
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

func RenderChannelRiskCLIReport(cfg Config) string {
	return renderChannelRiskReport(Event{}, cfg, nil, false)
}

func RenderChannelInfoCLIReport(cfg Config, provider string) string {
	return renderChannelInfoReport(Event{}, cfg, nil, provider, false)
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
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", surface.IngestWorkflow.Present)
	fmt.Fprintf(&b, "- workflow_dispatch_trigger: `%t`\n", surface.IngestWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- permissions_actions_write: `%t`\n", surface.IngestWorkflow.ActionsWrite)
	fmt.Fprintf(&b, "- permissions_issues_write: `%t`\n", surface.IngestWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- workflow_inputs: `%d`\n", surface.IngestWorkflow.Inputs)
	fmt.Fprintf(&b, "- state_workflow_path: `%s`\n", channelStateWorkflowPath)
	fmt.Fprintf(&b, "- state_workflow_present: `%t`\n", surface.StateWorkflow.Present)
	fmt.Fprintf(&b, "- state_workflow_dispatch_trigger: `%t`\n", surface.StateWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- state_workflow_permissions_issues_write: `%t`\n", surface.StateWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- state_workflow_inputs: `%d`\n", surface.StateWorkflow.Inputs)
	fmt.Fprintf(&b, "- gateway_workflow_path: `%s`\n", channelGatewayWorkflowPath)
	fmt.Fprintf(&b, "- gateway_workflow_present: `%t`\n", surface.GatewayWorkflow.Present)
	fmt.Fprintf(&b, "- gateway_workflow_dispatch_trigger: `%t`\n", surface.GatewayWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- gateway_workflow_permissions_actions_write: `%t`\n", surface.GatewayWorkflow.ActionsWrite)
	fmt.Fprintf(&b, "- gateway_workflow_permissions_issues_write: `%t`\n", surface.GatewayWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- gateway_workflow_inputs: `%d`\n", surface.GatewayWorkflow.Inputs)
	fmt.Fprintf(&b, "- delivery_workflow_path: `%s`\n", channelDeliveryWorkflowPath)
	fmt.Fprintf(&b, "- delivery_workflow_present: `%t`\n", surface.DeliveryWorkflow.Present)
	fmt.Fprintf(&b, "- delivery_workflow_dispatch_trigger: `%t`\n", surface.DeliveryWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- delivery_workflow_permissions_issues_write: `%t`\n", surface.DeliveryWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- delivery_workflow_inputs: `%d`\n", surface.DeliveryWorkflow.Inputs)
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
	if !surface.IngestWorkflow.Present {
		b.WriteString("- none\n")
	} else {
		fmt.Fprintf(
			&b,
			"- `%s` bytes=`%d` lines=`%d` workflow_dispatch=`%t` actions_write=`%t` issues_write=`%t` inputs=`%d` sha256_12=`%s`\n",
			surface.IngestWorkflow.Path,
			surface.IngestWorkflow.Bytes,
			surface.IngestWorkflow.Lines,
			surface.IngestWorkflow.WorkflowDispatch,
			surface.IngestWorkflow.ActionsWrite,
			surface.IngestWorkflow.IssuesWrite,
			surface.IngestWorkflow.Inputs,
			surface.IngestWorkflow.SHA,
		)
	}
	if surface.StateWorkflow.Present {
		fmt.Fprintf(
			&b,
			"- `%s` bytes=`%d` lines=`%d` workflow_dispatch=`%t` issues_write=`%t` inputs=`%d` sha256_12=`%s`\n",
			surface.StateWorkflow.Path,
			surface.StateWorkflow.Bytes,
			surface.StateWorkflow.Lines,
			surface.StateWorkflow.WorkflowDispatch,
			surface.StateWorkflow.IssuesWrite,
			surface.StateWorkflow.Inputs,
			surface.StateWorkflow.SHA,
		)
	}
	if surface.GatewayWorkflow.Present {
		fmt.Fprintf(
			&b,
			"- `%s` bytes=`%d` lines=`%d` workflow_dispatch=`%t` actions_write=`%t` issues_write=`%t` inputs=`%d` sha256_12=`%s`\n",
			surface.GatewayWorkflow.Path,
			surface.GatewayWorkflow.Bytes,
			surface.GatewayWorkflow.Lines,
			surface.GatewayWorkflow.WorkflowDispatch,
			surface.GatewayWorkflow.ActionsWrite,
			surface.GatewayWorkflow.IssuesWrite,
			surface.GatewayWorkflow.Inputs,
			surface.GatewayWorkflow.SHA,
		)
	}
	if surface.DeliveryWorkflow.Present {
		fmt.Fprintf(
			&b,
			"- `%s` bytes=`%d` lines=`%d` workflow_dispatch=`%t` issues_write=`%t` inputs=`%d` sha256_12=`%s`\n",
			surface.DeliveryWorkflow.Path,
			surface.DeliveryWorkflow.Bytes,
			surface.DeliveryWorkflow.Lines,
			surface.DeliveryWorkflow.WorkflowDispatch,
			surface.DeliveryWorkflow.IssuesWrite,
			surface.DeliveryWorkflow.Inputs,
			surface.DeliveryWorkflow.SHA,
		)
	}

	b.WriteString("\n### Providers\n")
	for _, provider := range channelReportProviders {
		fmt.Fprintf(&b, "- `%s`\n", provider)
	}

	b.WriteString("\n### Ingest Contract\n")
	b.WriteString("- `gitclaw channel-ingest --channel <provider> --thread-id <thread> --message-id <message> --body <text>`\n")
	b.WriteString("- `gitclaw channel-state --channel <provider> --account-id <account> --offset <offset>` stores durable provider offsets as hashes\n")
	b.WriteString("- `gitclaw channel-gateway --channel <provider> --account-id <account> [--renew]` records a gateway lease and can self-renew through workflow dispatch\n")
	b.WriteString("- `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>` records outbound delivery receipts\n")
	b.WriteString("- one canonical issue per `channel + thread_id`\n")
	b.WriteString("- one mirrored comment per `channel + message_id`\n")
	b.WriteString("- dispatch id: `<channel>-<message_id>`\n")

	return strings.TrimSpace(b.String())
}

func renderChannelInfoReport(ev Event, cfg Config, comments []Comment, provider string, includeIssue bool) string {
	provider = cleanChannelProviderName(provider)
	surface := inspectChannelSurface(cfg.Workdir)
	info, ok := lookupChannelProvider(provider)
	status := "unsupported"
	if ok {
		status = "ok"
	}
	channelMessages := countChannelMessages(comments)

	var b strings.Builder
	b.WriteString("## GitClaw Channel Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- requested_provider: `%s`\n", inlineCode(provider))
	fmt.Fprintf(&b, "- channel_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- supported_providers: `%s`\n", strings.Join(channelReportProviders, ", "))
	fmt.Fprintf(&b, "- channel_label: `%s`\n", cfg.ChannelLabel)
	fmt.Fprintf(&b, "- trigger_label: `%s`\n", cfg.TriggerLabel)
	fmt.Fprintf(&b, "- wake_strategy: `%s`\n", "workflow_dispatch")
	fmt.Fprintf(&b, "- state_storage: `%s`\n", "gitclaw:channel-state issue")
	fmt.Fprintf(&b, "- gateway_runtime: `%s`\n", "GitHub Actions workflow_dispatch")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_info_change: `%t`\n", true)
	if ok {
		fmt.Fprintf(&b, "- required_secrets: `%s`\n", inlineList(info.RequiredSecrets))
		fmt.Fprintf(&b, "- offset_key: `%s`\n", info.OffsetKey)
		fmt.Fprintf(&b, "- thread_key: `%s`\n", info.ThreadKey)
		fmt.Fprintf(&b, "- message_key: `%s`\n", info.MessageKey)
	}
	if includeIssue {
		fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- channel_message_comments_now: `%d`\n", channelMessages)
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report shows one channel provider bridge contract. Channel message bodies, issue bodies, workflow bodies, provider credentials, and secret values are not included.\n\n")

	b.WriteString("### Provider Contract\n")
	if !ok {
		b.WriteString("- none\n")
	} else {
		fmt.Fprintf(&b, "- provider=`%s` ingress=`%s`\n", info.Name, info.IngressStrategy)
		fmt.Fprintf(&b, "- gateway=`%s`\n", info.GatewayStrategy)
		fmt.Fprintf(&b, "- outbound=`%s`\n", info.OutboundDelivery)
		fmt.Fprintf(&b, "- offset_key=`%s` thread_key=`%s` message_key=`%s`\n", info.OffsetKey, info.ThreadKey, info.MessageKey)
		fmt.Fprintf(&b, "- required_secret_names=`%s`\n", inlineList(info.RequiredSecrets))
	}

	b.WriteString("\n### Workflow Surface\n")
	writeChannelWorkflowInfo(&b, "ingest", surface.IngestWorkflow)
	writeChannelWorkflowInfo(&b, "state", surface.StateWorkflow)
	writeChannelWorkflowInfo(&b, "gateway", surface.GatewayWorkflow)
	writeChannelWorkflowInfo(&b, "delivery", surface.DeliveryWorkflow)

	b.WriteString("\n### Commands\n")
	if ok {
		fmt.Fprintf(&b, "- `gitclaw channel-ingest --channel %s --thread-id <thread> --message-id <message> --body <text>`\n", info.Name)
		fmt.Fprintf(&b, "- `gitclaw channel-state --channel %s --account-id <account> --offset <offset>`\n", info.Name)
		fmt.Fprintf(&b, "- `gitclaw channel-gateway --channel %s --account-id <account> --renew`\n", info.Name)
		fmt.Fprintf(&b, "- `gitclaw channel-delivery --channel %s --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n", info.Name)
		fmt.Fprintf(&b, "- dispatch id: `%s-<message_id>`\n", info.Name)
	} else {
		b.WriteString("- none\n")
	}

	if !ok {
		b.WriteString("\n### Available Providers\n")
		for _, item := range channelProviderCatalog {
			fmt.Fprintf(&b, "- `%s`\n", item.Name)
		}
	}
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
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", surface.IngestWorkflow.Present)
	fmt.Fprintf(&b, "- workflow_dispatch_trigger: `%t`\n", surface.IngestWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- permissions_actions_write: `%t`\n", surface.IngestWorkflow.ActionsWrite)
	fmt.Fprintf(&b, "- permissions_issues_write: `%t`\n", surface.IngestWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- workflow_inputs: `%d`\n", surface.IngestWorkflow.Inputs)
	fmt.Fprintf(&b, "- required_workflow_inputs: `%d`\n", 5)
	fmt.Fprintf(&b, "- state_workflow_path: `%s`\n", channelStateWorkflowPath)
	fmt.Fprintf(&b, "- state_workflow_present: `%t`\n", surface.StateWorkflow.Present)
	fmt.Fprintf(&b, "- state_workflow_dispatch_trigger: `%t`\n", surface.StateWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- state_workflow_permissions_issues_write: `%t`\n", surface.StateWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- state_workflow_inputs: `%d`\n", surface.StateWorkflow.Inputs)
	fmt.Fprintf(&b, "- gateway_workflow_path: `%s`\n", channelGatewayWorkflowPath)
	fmt.Fprintf(&b, "- gateway_workflow_present: `%t`\n", surface.GatewayWorkflow.Present)
	fmt.Fprintf(&b, "- gateway_workflow_dispatch_trigger: `%t`\n", surface.GatewayWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- gateway_workflow_permissions_actions_write: `%t`\n", surface.GatewayWorkflow.ActionsWrite)
	fmt.Fprintf(&b, "- gateway_workflow_permissions_issues_write: `%t`\n", surface.GatewayWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- gateway_workflow_inputs: `%d`\n", surface.GatewayWorkflow.Inputs)
	fmt.Fprintf(&b, "- delivery_workflow_path: `%s`\n", channelDeliveryWorkflowPath)
	fmt.Fprintf(&b, "- delivery_workflow_present: `%t`\n", surface.DeliveryWorkflow.Present)
	fmt.Fprintf(&b, "- delivery_workflow_dispatch_trigger: `%t`\n", surface.DeliveryWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- delivery_workflow_permissions_issues_write: `%t`\n", surface.DeliveryWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- delivery_workflow_inputs: `%d`\n", surface.DeliveryWorkflow.Inputs)
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
	b.WriteString("- channel state and gateway workflows are callable with `workflow_dispatch`\n")
	b.WriteString("- gateway workflow can dispatch its renewal with `actions: write`\n")
	b.WriteString("- delivery workflow records outbound receipts with `issues: write`\n")
	b.WriteString("- downstream wakeup uses dispatch id `<channel>-<message_id>`\n")

	return strings.TrimSpace(b.String())
}

func channelVerifyFindings(surface channelSurface) []string {
	var findings []string
	if !surface.IngestWorkflow.Present {
		return []string{"channel_ingest_workflow_missing"}
	}
	if !surface.IngestWorkflow.WorkflowDispatch {
		findings = append(findings, "workflow_dispatch_missing")
	}
	if !surface.IngestWorkflow.ActionsWrite {
		findings = append(findings, "actions_write_permission_missing")
	}
	if !surface.IngestWorkflow.IssuesWrite {
		findings = append(findings, "issues_write_permission_missing")
	}
	if surface.IngestWorkflow.Inputs < 5 {
		findings = append(findings, "required_workflow_inputs_missing")
	}
	if !surface.StateWorkflow.Present {
		findings = append(findings, "channel_state_workflow_missing")
	} else {
		if !surface.StateWorkflow.WorkflowDispatch {
			findings = append(findings, "state_workflow_dispatch_missing")
		}
		if !surface.StateWorkflow.IssuesWrite {
			findings = append(findings, "state_workflow_issues_write_missing")
		}
		if surface.StateWorkflow.Inputs < 4 {
			findings = append(findings, "state_workflow_inputs_missing")
		}
	}
	if !surface.GatewayWorkflow.Present {
		findings = append(findings, "channel_gateway_workflow_missing")
	} else {
		if !surface.GatewayWorkflow.WorkflowDispatch {
			findings = append(findings, "gateway_workflow_dispatch_missing")
		}
		if !surface.GatewayWorkflow.ActionsWrite {
			findings = append(findings, "gateway_workflow_actions_write_missing")
		}
		if !surface.GatewayWorkflow.IssuesWrite {
			findings = append(findings, "gateway_workflow_issues_write_missing")
		}
		if surface.GatewayWorkflow.Inputs < 6 {
			findings = append(findings, "gateway_workflow_inputs_missing")
		}
	}
	if !surface.DeliveryWorkflow.Present {
		findings = append(findings, "channel_delivery_workflow_missing")
	} else {
		if !surface.DeliveryWorkflow.WorkflowDispatch {
			findings = append(findings, "delivery_workflow_dispatch_missing")
		}
		if !surface.DeliveryWorkflow.IssuesWrite {
			findings = append(findings, "delivery_workflow_issues_write_missing")
		}
		if surface.DeliveryWorkflow.Inputs < 6 {
			findings = append(findings, "delivery_workflow_inputs_missing")
		}
	}
	return findings
}

func isChannelVerifyRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && (fields[0] == "/channel" || fields[0] == "/channels") && strings.EqualFold(fields[1], "verify")
}

func isChannelRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && (fields[0] == "/channel" || fields[0] == "/channels") && (strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
}

func requestedChannelInfoProvider(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || (fields[0] != "/channel" && fields[0] != "/channels") || !strings.EqualFold(fields[1], "info") {
		return ""
	}
	return cleanChannelProviderName(fields[2])
}

func lookupChannelProvider(provider string) (channelProviderInfo, bool) {
	provider = cleanChannelProviderName(provider)
	for _, item := range channelProviderCatalog {
		if item.Name == provider {
			return item, true
		}
	}
	return channelProviderInfo{}, false
}

func cleanChannelProviderName(provider string) string {
	provider = strings.ToLower(strings.Trim(strings.TrimSpace(provider), " \t\r\n.,:;!?`\"'"))
	switch provider {
	case "tg":
		return "telegram"
	default:
		return provider
	}
}

func inspectChannelSurface(root string) channelSurface {
	if root == "" {
		root = "."
	}
	surface := channelSurface{
		IngestWorkflow:   channelWorkflow{Path: channelIngestWorkflowPath},
		StateWorkflow:    channelWorkflow{Path: channelStateWorkflowPath},
		GatewayWorkflow:  channelWorkflow{Path: channelGatewayWorkflowPath},
		DeliveryWorkflow: channelWorkflow{Path: channelDeliveryWorkflowPath},
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	surface.IngestWorkflow = inspectChannelWorkflow(absRoot, channelIngestWorkflowPath)
	surface.StateWorkflow = inspectChannelWorkflow(absRoot, channelStateWorkflowPath)
	surface.GatewayWorkflow = inspectChannelWorkflow(absRoot, channelGatewayWorkflowPath)
	surface.DeliveryWorkflow = inspectChannelWorkflow(absRoot, channelDeliveryWorkflowPath)
	return surface
}

func inspectChannelWorkflow(absRoot, rel string) channelWorkflow {
	workflow := channelWorkflow{Path: rel}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(rel)))
	if err != nil {
		return workflow
	}
	text := string(body)
	workflow.Present = true
	workflow.Body = text
	workflow.Bytes = len(body)
	workflow.Lines = lineCount(text)
	workflow.SHA = shortDocumentHash(text)
	workflow.WorkflowDispatch = strings.Contains(text, "workflow_dispatch:")
	workflow.ActionsWrite = strings.Contains(text, "actions: write")
	workflow.IssuesWrite = strings.Contains(text, "issues: write")
	workflow.Inputs = countWorkflowInputKeys(text)
	return workflow
}

func writeChannelWorkflowInfo(b *strings.Builder, name string, workflow channelWorkflow) {
	if !workflow.Present {
		fmt.Fprintf(b, "- `%s` path=`%s` present=`false`\n", name, workflow.Path)
		return
	}
	fmt.Fprintf(
		b,
		"- `%s` path=`%s` present=`true` bytes=`%d` lines=`%d` workflow_dispatch=`%t` actions_write=`%t` issues_write=`%t` inputs=`%d` sha256_12=`%s`\n",
		name,
		workflow.Path,
		workflow.Bytes,
		workflow.Lines,
		workflow.WorkflowDispatch,
		workflow.ActionsWrite,
		workflow.IssuesWrite,
		workflow.Inputs,
		workflow.SHA,
	)
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
