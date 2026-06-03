package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const channelIngestWorkflowPath = ".github/workflows/gitclaw-channel-ingest.yml"
const channelSendWorkflowPath = ".github/workflows/gitclaw-channel-send.yml"
const channelStateWorkflowPath = ".github/workflows/gitclaw-channel-state.yml"
const channelGatewayWorkflowPath = ".github/workflows/gitclaw-channel-gateway.yml"
const channelDeliveryWorkflowPath = ".github/workflows/gitclaw-channel-delivery.yml"
const channelOutboxWorkflowPath = ".github/workflows/gitclaw-channel-outbox.yml"

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
	SendWorkflow     channelWorkflow
	StateWorkflow    channelWorkflow
	GatewayWorkflow  channelWorkflow
	DeliveryWorkflow channelWorkflow
	OutboxWorkflow   channelWorkflow
	Routebook        configSurfaceFile
	Routes           int
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
	IssuesRead       bool
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
	fmt.Fprintf(&b, "- send_workflow_path: `%s`\n", channelSendWorkflowPath)
	fmt.Fprintf(&b, "- send_workflow_present: `%t`\n", surface.SendWorkflow.Present)
	fmt.Fprintf(&b, "- send_workflow_dispatch_trigger: `%t`\n", surface.SendWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- send_workflow_permissions_issues_write: `%t`\n", surface.SendWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- send_workflow_inputs: `%d`\n", surface.SendWorkflow.Inputs)
	fmt.Fprintf(&b, "- routebook_path: `%s`\n", channelRoutesPath)
	fmt.Fprintf(&b, "- routebook_present: `%t`\n", surface.Routebook.Present)
	fmt.Fprintf(&b, "- named_routes: `%d`\n", surface.Routes)
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
	fmt.Fprintf(&b, "- outbox_workflow_path: `%s`\n", channelOutboxWorkflowPath)
	fmt.Fprintf(&b, "- outbox_workflow_present: `%t`\n", surface.OutboxWorkflow.Present)
	fmt.Fprintf(&b, "- outbox_workflow_dispatch_trigger: `%t`\n", surface.OutboxWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- outbox_workflow_permissions_issues_read: `%t`\n", surface.OutboxWorkflow.IssuesRead)
	fmt.Fprintf(&b, "- outbox_workflow_inputs: `%d`\n", surface.OutboxWorkflow.Inputs)
	if includeIssue {
		fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- channel_message_comments_now: `%d`\n", channelMessages)
	}
	fmt.Fprintf(&b, "- supported_providers: `%s`\n", strings.Join(channelReportProviders, ", "))
	fmt.Fprintf(&b, "- wake_strategy: `%s`\n", "workflow_dispatch")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_report_change: `%t`\n", true)
	if includeIssue && isChannelListRequest(ev, cfg) {
		fmt.Fprintf(&b, "- llm_e2e_required_after_channel_list_change: `%t`\n", true)
	}
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
	if surface.SendWorkflow.Present {
		fmt.Fprintf(
			&b,
			"- `%s` bytes=`%d` lines=`%d` workflow_dispatch=`%t` issues_write=`%t` inputs=`%d` sha256_12=`%s`\n",
			surface.SendWorkflow.Path,
			surface.SendWorkflow.Bytes,
			surface.SendWorkflow.Lines,
			surface.SendWorkflow.WorkflowDispatch,
			surface.SendWorkflow.IssuesWrite,
			surface.SendWorkflow.Inputs,
			surface.SendWorkflow.SHA,
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
	if surface.OutboxWorkflow.Present {
		fmt.Fprintf(
			&b,
			"- `%s` bytes=`%d` lines=`%d` workflow_dispatch=`%t` issues_read=`%t` inputs=`%d` sha256_12=`%s`\n",
			surface.OutboxWorkflow.Path,
			surface.OutboxWorkflow.Bytes,
			surface.OutboxWorkflow.Lines,
			surface.OutboxWorkflow.WorkflowDispatch,
			surface.OutboxWorkflow.IssuesRead,
			surface.OutboxWorkflow.Inputs,
			surface.OutboxWorkflow.SHA,
		)
	}

	b.WriteString("\n### Providers\n")
	for _, provider := range channelReportProviders {
		fmt.Fprintf(&b, "- `%s`\n", provider)
	}

	b.WriteString("\n### Ingest Contract\n")
	b.WriteString("- `/channels send --route <name>` queues a reviewed outbound channel message from an issue or comment\n")
	b.WriteString("- `/channels deliverable --deliverable-id <id> --message-id <message> --filename <name>` queues a provider-native file/link deliverable from a mirrored channel thread\n")
	b.WriteString("- `/channels task --task-id <id> --message-id <message>` creates a GitHub task from a mirrored channel thread\n")
	b.WriteString("- `/channels watch --watch-id <id> --cadence <cadence> --message-id <message>` creates a proactive GitHub watch issue from a mirrored channel thread\n")
	b.WriteString("- `/channels clip --clip-id <id> --message-id <message>` saves a durable GitHub clip from a mirrored channel thread\n")
	b.WriteString("- `/channels open-loop --loop-id <id> --message-id <message>` captures a loose end from a mirrored channel thread as a GitHub issue\n")
	b.WriteString("- `/channels attachment --attachment-id <id> --message-id <message> --filename <name>` records channel file/media metadata as a GitHub issue\n")
	b.WriteString("- `/channels link --link-id <id> --url <url> --message-id <message>` records channel link-card metadata as a GitHub issue\n")
	b.WriteString("- `/channels kudos --kudos-id <id> --message-id <message>` records channel appreciation as a GitHub issue and queues an acknowledgement\n")
	b.WriteString("- `/channels retro --retro-id <id> --message-id <message>` records channel retrospectives as GitHub issues and queues retro links\n")
	b.WriteString("- `/channels playbook --playbook-id <id> --message-id <message>` records reusable channel procedures as GitHub playbook issues and queues playbook links\n")
	b.WriteString("- `/channels insight --insight-id <id> --message-id <message>` records channel observations as GitHub insight issues and queues insight links\n")
	b.WriteString("- `/channels board-card --card-id <id> --lane <lane> --message-id <message>` records channel board cards as GitHub issues and queues board-card links\n")
	b.WriteString("- `/channels checklist --checklist-id <id> --message-id <message>` records channel checklists as GitHub issues and queues checklist links\n")
	b.WriteString("- `/channels agenda --agenda-id <id> --message-id <message>` records channel agendas as GitHub issues and queues agenda links\n")
	b.WriteString("- `/channels journal --journal-id <id> --date <date> --message-id <message>` records channel journal entries as GitHub issues and queues journal links without mutating memory\n")
	b.WriteString("- `/channels quote --quote-id <id> --message-id <message>` preserves channel quotes as GitHub issues and queues quote links without mutating memory\n")
	b.WriteString("- `/channels glossary --glossary-id <id> --message-id <message>` preserves channel terms and definitions as GitHub issues and queues glossary links without mutating memory\n")
	b.WriteString("- `/channels propose-workspace --workspace-id <id> --target <path> --message-id <message>` records channel workspace-context proposals as GitHub issues and queues proposal links without writing workspace files\n")
	b.WriteString("- `/channels access-request --access-id <id> --message-id <message>` creates a GitHub review issue for channel access or pairing requests without granting access\n")
	b.WriteString("- `/channels availability --message-id <message>` queues a provider-facing availability/presence card without probing provider sockets, reading session stores as liveness, calling provider APIs, calling a model, or mutating the repository\n")
	b.WriteString("- `/channels topic --topic-id <id>` queues a provider-facing thread topic/title update from a mirrored channel thread without renaming GitHub issues, calling provider APIs, calling a model, or mutating the repository\n")
	b.WriteString("- `/channels activity <activity> --activity-id <id>` queues a transient provider-facing chat activity signal from a mirrored channel thread without opening sockets, calling provider APIs, calling a model, or mutating the repository\n")
	b.WriteString("- `/channels platform <provider> --state <state> --message-id <message>` queues a provider-facing platform-status message without controlling adapters\n")
	b.WriteString("- `/channels model --message-id <message>` queues a provider-facing model-status message without calling or switching models\n")
	b.WriteString("- `/channels skills --message-id <message>` queues a provider-facing skill-status message without calling a model or installing skills\n")
	b.WriteString("- `/channels skill-search <query> --message-id <message>` queues provider-facing body-free skill metadata matches without registry contact or skill installation\n")
	b.WriteString("- `/channels skill-info <skill> --message-id <message>` queues one provider-facing focused skill card without loading skill bodies or installing skills\n")
	b.WriteString("- `/channels tools --message-id <message>` queues a provider-facing tool-status message without executing tools or exposing raw schemas\n")
	b.WriteString("- `/channels tool-search <query> --message-id <message>` queues provider-facing tool capability matches without executing tools or exposing raw schemas\n")
	b.WriteString("- `/channels tool-info <tool> --message-id <message>` queues one provider-facing focused tool card without executing tools or exposing raw schemas\n")
	b.WriteString("- `/channels backup --message-id <message>` queues a provider-facing backup-status message without reading backup payloads or restoring files\n")
	b.WriteString("- `/channels backup-search <query> --message-id <message>` queues provider-facing body-free recall results from the fetched gitclaw-backups archive\n")
	b.WriteString("- `/channels backup-info <issue> --message-id <message>` queues one provider-facing focused backup metadata card from the fetched gitclaw-backups archive without reading raw payload bodies or restoring files\n")
	b.WriteString("- `/channels checkpoint-status --message-id <message>` queues a provider-facing checkpoint and rollback-readiness card without raw diffs, file bodies, commit subjects, restore, reset, clean, checkout, provider API calls, or repo mutation\n")
	b.WriteString("- `/channels profile-status --message-id <message>` queues a provider-facing repo-profile snapshot without exporting, importing, switching, or mutating profiles\n")
	b.WriteString("- `/channels soul-status --message-id <message>` queues a provider-facing high-authority soul snapshot without registry contact, profile export, soul writes, or raw bodies\n")
	b.WriteString("- `/channels soul-info <path> --message-id <message>` queues one provider-facing focused high-authority context card without registry contact, profile export, soul writes, raw file bodies, or source-receipt paths\n")
	b.WriteString("- `/channels soul-risk --message-id <message>` queues one provider-facing high-authority persistent-state risk card without registry contact, profile export, soul writes, memory writes, model calls, raw bodies, or source-receipt paths\n")
	b.WriteString("- `/channels soul-search <query> --message-id <message>` queues provider-facing body-free high-authority context recall results from repo-local soul files\n")
	b.WriteString("- `/channels memory-status --message-id <message>` queues a provider-facing durable-memory snapshot without memory writes, external provider access, embedding vectors, or raw memory bodies\n")
	b.WriteString("- `/channels memory-search <query> --message-id <message>` queues provider-facing body-free durable-memory recall results from repo-local memory files\n")
	b.WriteString("- `/channels whoami --identity-id <id> --message-id <message>` queues a provider-facing identity-status message without granting access\n")
	b.WriteString("- `/channels contact --contact-id <id> --message-id <message>` saves a GitHub contact-card issue for a channel identity without granting access\n")
	b.WriteString("- `/channels roll --dice <expr> --message-id <message>` queues a provider-facing deterministic dice/coin result without model calls or external randomness\n")
	b.WriteString("- `/channels choose --message-id <message>` queues a provider-facing deterministic option pick without model calls or external randomness\n")
	b.WriteString("- `/channels mood <mood> --message-id <message>` queues a provider-facing presence update without model calls, provider API calls, or repo mutation\n")
	b.WriteString("- `/channels tool-result --tool <tool> --result-id <id> --status <status> --message-id <message>` records externally observed tool results as GitHub issues without executing tools\n")
	b.WriteString("- `/channels session-search <query> --message-id <message>` queues provider-facing body-free session recall results from the current GitHub-backed channel transcript\n")
	b.WriteString("- `/channels approval-plan <tool> --id <id> --message-id <message>` creates a GitHub tool approval-plan issue from a mirrored channel thread\n")
	b.WriteString("- `/channels propose-toolset --toolset-id <id> --message-id <message>` creates a GitHub toolset proposal issue from a mirrored channel thread\n")
	b.WriteString("- `/channels propose-prompt --prompt-id <id> --message-id <message>` creates a GitHub prompt-pack proposal issue from a mirrored channel thread\n")
	b.WriteString("- `/channels propose-bundle --bundle-id <id> --message-id <message>` creates a GitHub skill-bundle proposal issue from a mirrored channel thread\n")
	b.WriteString("- `/channels propose-skill <name> --message-id <message>` creates a GitHub skill proposal issue from a mirrored channel thread\n")
	b.WriteString("- `/channels propose-soul --target <path> --id <id> --message-id <message>` creates a GitHub soul proposal issue from a mirrored channel thread\n")
	b.WriteString("- `/channels propose-memory --target <target> --id <id> --message-id <message>` creates a GitHub memory proposal issue from a mirrored channel thread\n")
	b.WriteString("- `/channels room <routes> --room-id <id> --message-id <message>` creates a durable GitHub room issue and invites reviewed routes\n")
	b.WriteString("- `/channels huddle <routes> --huddle-id <id> --message-id <message>` creates a GitHub huddle issue and invites reviewed routes\n")
	b.WriteString("- `/channels poll-vote --poll-id <id> --message-id <message> --notify-message-id <message> --choice <choice>` records a channel-origin poll vote and queues an acknowledgement\n")
	b.WriteString("- `/channels rsvp <routes> --rsvp-id <id> --message-id <message>` creates a GitHub RSVP issue and invites reviewed routes\n")
	b.WriteString("- `/channels rsvp-response --rsvp-id <id> --message-id <message> --notify-message-id <message> --response yes` records a channel-origin RSVP response and queues an acknowledgement\n")
	b.WriteString("- `gitclaw channel-ingest --channel <provider> --thread-id <thread> --message-id <message> --body <text>`\n")
	b.WriteString("- `gitclaw channel-send --channel <provider> --thread-id <thread> --message-id <message> --body <text>` queues a GitHub-originated outbound message\n")
	b.WriteString("- `gitclaw channel-send --route <name> --message-id <message> --body <text>` resolves a repo-reviewed named route\n")
	b.WriteString("- `gitclaw channel-state --channel <provider> --account-id <account> --offset <offset>` stores durable provider offsets as hashes\n")
	b.WriteString("- `gitclaw channel-gateway --channel <provider> --account-id <account> [--renew]` records a gateway lease and can self-renew through workflow dispatch\n")
	b.WriteString("- `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>` returns pending assistant replies for delivery\n")
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
	writeChannelWorkflowInfo(&b, "send", surface.SendWorkflow)
	writeChannelWorkflowInfo(&b, "state", surface.StateWorkflow)
	writeChannelWorkflowInfo(&b, "gateway", surface.GatewayWorkflow)
	writeChannelWorkflowInfo(&b, "delivery", surface.DeliveryWorkflow)
	writeChannelWorkflowInfo(&b, "outbox", surface.OutboxWorkflow)
	writeConfigSurfaceFile(&b, surface.Routebook)

	b.WriteString("\n### Commands\n")
	if ok {
		fmt.Fprintf(&b, "- `gitclaw channel-ingest --channel %s --thread-id <thread> --message-id <message> --body <text>`\n", info.Name)
		b.WriteString("- `/channels send --route <name>`\n")
		b.WriteString("- `/channels room <routes> --room-id <id> --message-id <message>`\n")
		fmt.Fprintf(&b, "- `gitclaw channel-send --channel %s --thread-id <thread> --message-id <message> --body <text>`\n", info.Name)
		b.WriteString("- `gitclaw channel-send --route <name> --message-id <message> --body <text>`\n")
		fmt.Fprintf(&b, "- `gitclaw channel-state --channel %s --account-id <account> --offset <offset>`\n", info.Name)
		fmt.Fprintf(&b, "- `gitclaw channel-gateway --channel %s --account-id <account> --renew`\n", info.Name)
		fmt.Fprintf(&b, "- `gitclaw channel-outbox --channel %s --account-id <account> --issue-number <issue> --out <file>`\n", info.Name)
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
	fmt.Fprintf(&b, "- send_workflow_path: `%s`\n", channelSendWorkflowPath)
	fmt.Fprintf(&b, "- send_workflow_present: `%t`\n", surface.SendWorkflow.Present)
	fmt.Fprintf(&b, "- send_workflow_dispatch_trigger: `%t`\n", surface.SendWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- send_workflow_permissions_issues_write: `%t`\n", surface.SendWorkflow.IssuesWrite)
	fmt.Fprintf(&b, "- send_workflow_inputs: `%d`\n", surface.SendWorkflow.Inputs)
	fmt.Fprintf(&b, "- routebook_path: `%s`\n", channelRoutesPath)
	fmt.Fprintf(&b, "- routebook_present: `%t`\n", surface.Routebook.Present)
	fmt.Fprintf(&b, "- named_routes: `%d`\n", surface.Routes)
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
	fmt.Fprintf(&b, "- outbox_workflow_path: `%s`\n", channelOutboxWorkflowPath)
	fmt.Fprintf(&b, "- outbox_workflow_present: `%t`\n", surface.OutboxWorkflow.Present)
	fmt.Fprintf(&b, "- outbox_workflow_dispatch_trigger: `%t`\n", surface.OutboxWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- outbox_workflow_permissions_issues_read: `%t`\n", surface.OutboxWorkflow.IssuesRead)
	fmt.Fprintf(&b, "- outbox_workflow_inputs: `%d`\n", surface.OutboxWorkflow.Inputs)
	fmt.Fprintf(&b, "- supported_providers: `%s`\n", strings.Join(channelReportProviders, ", "))
	fmt.Fprintf(&b, "- wake_strategy: `%s`\n", "workflow_dispatch")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_verify_change: `%t`\n", true)
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
	b.WriteString("- channel-send workflow can queue GitHub-originated outbound messages with `issues: write`\n")
	b.WriteString("- `/channels send` can queue a reviewed outbound message from a trusted issue/comment without calling a model\n")
	b.WriteString("- `/channels deliverable` can queue a provider-native file/link deliverable from a mirrored channel thread without calling a model or uploading files\n")
	b.WriteString("- `/channels task` can create a GitHub task issue from a mirrored channel thread and queue a task-link notification without calling a model\n")
	b.WriteString("- `/channels watch` can create a proactive GitHub watch issue from a mirrored channel thread and queue a watch-link notification without calling a model or opening a socket\n")
	b.WriteString("- `/channels clip` can create a GitHub clip issue from a mirrored channel thread and queue a clip-link notification without calling a model\n")
	b.WriteString("- `/channels open-loop` can create a GitHub open-loop issue from a mirrored channel thread and queue an open-loop link without calling a model, scheduling work, or mutating the repository\n")
	b.WriteString("- `/channels attachment` can create a GitHub attachment metadata issue from a mirrored channel thread and queue an attachment-link notification without calling a model or fetching file bytes\n")
	b.WriteString("- `/channels link` can create a GitHub link-card issue from a mirrored channel thread and queue a link-card notification without calling a model or fetching/expanding URLs\n")
	b.WriteString("- `/channels kudos` can create a GitHub appreciation issue from a mirrored channel thread and queue an acknowledgement without calling a model\n")
	b.WriteString("- `/channels retro` can create a GitHub retrospective issue from a mirrored channel thread and queue a retro-link notification without calling a model\n")
	b.WriteString("- `/channels playbook` can create a GitHub playbook issue from a mirrored channel thread and queue a playbook-link notification without calling a model or installing a skill\n")
	b.WriteString("- `/channels insight` can create a GitHub insight issue from a mirrored channel thread and queue an insight-link notification without calling a model or mutating memory/soul/skills\n")
	b.WriteString("- `/channels board-card` can create a GitHub board-card issue from a mirrored channel thread and queue a board-card link without calling a model, mutating the repository, or moving cards outside GitHub review\n")
	b.WriteString("- `/channels checklist` can create a GitHub checklist issue from a mirrored channel thread and queue a checklist link without calling a model, mutating the repository, or changing task state outside GitHub review\n")
	b.WriteString("- `/channels agenda` can create a GitHub agenda issue from a mirrored channel thread and queue an agenda link without calling a model, mutating the repository, or treating agenda items as completed tasks\n")
	b.WriteString("- `/channels journal` can create a GitHub journal issue from a mirrored channel thread and queue a journal link without calling a model, mutating the repository, or writing `.gitclaw/MEMORY.md`\n")
	b.WriteString("- `/channels quote` can preserve a channel quote as a GitHub issue and queue a quote link without calling a model, mutating memory, mutating the repository, or copying raw quote text into the source receipt\n")
	b.WriteString("- `/channels glossary` can preserve a channel term and definition as a GitHub issue and queue a glossary link without calling a model, mutating memory, mutating the repository, or copying raw definition text into the source receipt\n")
	b.WriteString("- `/channels propose-workspace` can create a GitHub workspace proposal issue from a mirrored channel thread and queue a proposal-link notification without calling a model, mutating the repository, or writing workspace files\n")
	b.WriteString("- `/channels access-request` can create a GitHub access-review issue from a mirrored channel thread and queue a review-link notification without granting access, mutating allowlists, or issuing pairing codes\n")
	b.WriteString("- `/channels availability` can queue a provider-facing availability/presence card from a mirrored channel thread without probing provider sockets, treating session rows as socket liveness, calling provider APIs, calling a model, editing workflows, or mutating the repository\n")
	b.WriteString("- `/channels topic` can queue a provider-facing thread topic/title update from a mirrored channel thread without renaming GitHub issues, calling provider APIs, calling a model, editing workflows, or mutating the repository\n")
	b.WriteString("- `/channels activity` can queue a transient provider-facing chat activity signal from a mirrored channel thread without opening sockets, long-running gateways, provider API calls, model calls, workflow edits, or repository mutation\n")
	b.WriteString("- `/channels tool-result` can create a GitHub tool-result issue from a mirrored channel thread and queue a tool-result link without executing tools, calling a model, mutating the repository, or copying raw tool output into the source receipt\n")
	b.WriteString("- `/channels platform` can queue a provider-facing platform-status message from a mirrored channel thread without pausing/resuming adapters, mutating breaker state, changing home channels, starting gateways, or calling provider APIs\n")
	b.WriteString("- `/channels model` can queue a provider-facing model-status message from a mirrored channel thread without calling a model, switching models, writing model config, or mutating the repository\n")
	b.WriteString("- `/channels skills` can queue a provider-facing skill-status message from a mirrored channel thread without calling a model, installing skills, updating skills, contacting registries, or mutating the repository\n")
	b.WriteString("- `/channels skill-info` can queue a focused provider-facing skill metadata card from a mirrored channel thread without calling a model, loading `SKILL.md` bodies, installing skills, updating skills, contacting registries, or mutating the repository\n")
	b.WriteString("- `/channels tools` can queue a provider-facing tool-status message from a mirrored channel thread without executing tools, launching MCP servers, activating toolsets, exposing raw schemas, or mutating the repository\n")
	b.WriteString("- `/channels backup` can queue a provider-facing backup-status message from a mirrored channel thread without fetching the backup branch, reading backup payloads, restoring files, replaying GitHub APIs, or mutating the repository\n")
	b.WriteString("- `/channels backup-info` can queue a focused provider-facing backup metadata card from a mirrored channel thread by fetching `gitclaw-backups` read-only when needed, without writing the backup branch, restoring files, replaying GitHub APIs, printing raw backup payloads, or mutating the repository\n")
	b.WriteString("- `/channels checkpoint-status` can queue a provider-facing checkpoint and rollback-readiness card from a mirrored channel thread without generating raw diffs, printing file bodies or commit subjects, restoring, resetting, cleaning, checking out, calling provider APIs, or mutating the repository\n")
	b.WriteString("- `/channels profile-status` can queue a provider-facing repo-profile snapshot from a mirrored channel thread without exporting profiles, importing profiles, switching profiles, reading external agent homes, exposing raw profile bodies, or mutating the repository\n")
	b.WriteString("- `/channels soul-status` can queue a provider-facing high-authority soul snapshot from a mirrored channel thread without registry contact, profile export, soul writes, raw soul/identity/user/memory/tool/heartbeat bodies, or repository mutation\n")
	b.WriteString("- `/channels soul-info` can queue a focused provider-facing high-authority context metadata card from a mirrored channel thread without registry contact, profile export, soul writes, memory writes, model calls, raw context paths in the source receipt, raw soul/identity/user/memory/tool/heartbeat bodies, or repository mutation\n")
	b.WriteString("- `/channels soul-risk` can queue a provider-facing high-authority persistent-state risk card from a mirrored channel thread without registry contact, profile export, soul writes, memory writes, model calls, raw context paths in the source receipt, raw soul/identity/user/memory/tool/heartbeat bodies, or repository mutation\n")
	b.WriteString("- `/channels soul-search` can queue provider-facing high-authority context recall results from a mirrored channel thread without registry contact, profile export, soul writes, model calls, raw query text, raw soul/identity/user/memory/tool/heartbeat bodies, or repository mutation\n")
	b.WriteString("- `/channels memory-status` can queue a provider-facing durable-memory snapshot from a mirrored channel thread without memory writes, background promotion, external provider access, embedding vectors, raw memory/issue/comment/prompt/session bodies, or repository mutation\n")
	b.WriteString("- `/channels roll` can queue a deterministic dice/coin result from a mirrored channel thread without calling a model, using external randomness, calling provider APIs, mutating the repo, or exposing raw roll ids/expressions in the source receipt\n")
	b.WriteString("- `/channels choose` can queue a deterministic option pick from a mirrored channel thread without calling a model, using external randomness, calling provider APIs, mutating the repo, or exposing raw choices in the source receipt\n")
	b.WriteString("- `/channels whoami` can queue a provider-facing identity-status message from a mirrored channel thread without creating artifacts, granting access, mutating allowlists, or issuing pairing codes\n")
	b.WriteString("- `/channels contact` can create a GitHub contact-card issue from a mirrored channel thread and queue a contact-card notification without granting access, mutating allowlists, or issuing pairing codes\n")
	b.WriteString("- `/channels approval-plan` can create a GitHub tool approval-plan issue from a mirrored channel thread and queue an approval-link notification without approving or executing the tool\n")
	b.WriteString("- `/channels propose-toolset` can create a GitHub toolset proposal issue from a mirrored channel thread and queue a proposal-link notification without enabling toolsets, executing tools, writing tool configuration, or mutating the repository\n")
	b.WriteString("- `/channels propose-prompt` can create a GitHub prompt-pack proposal issue from a mirrored channel thread and queue a proposal-link notification without enabling prompts, running prompt tests, writing prompt configuration, or mutating the repository\n")
	b.WriteString("- `/channels propose-bundle` can create a GitHub skill-bundle proposal issue from a mirrored channel thread and queue a proposal-link notification without installing skills, enabling bundles, writing bundle YAML, or mutating the repository\n")
	b.WriteString("- `/channels propose-skill` can create a GitHub skill proposal issue from a mirrored channel thread and queue a proposal-link notification without generating or installing a skill\n")
	b.WriteString("- `/channels propose-soul` can create a GitHub soul proposal issue from a mirrored channel thread and queue a proposal-link notification without generating or applying high-authority context\n")
	b.WriteString("- `/channels propose-memory` can create a GitHub memory proposal issue from a mirrored channel thread and queue a proposal-link notification without generating or writing memory\n")
	b.WriteString("- `/channels room` can create a durable GitHub room issue and queue reviewed route invitations without calling a model\n")
	b.WriteString("- `/channels huddle` can create a GitHub huddle issue and queue reviewed route invitations without calling a model\n")
	b.WriteString("- `/channels poll-vote` can record a channel-origin poll vote on the GitHub poll issue and queue an acknowledgement without calling a model\n")
	b.WriteString("- `/channels rsvp` can create a GitHub RSVP issue and queue reviewed route invitations without calling a model\n")
	b.WriteString("- `/channels rsvp-response` can record a channel-origin RSVP response on the GitHub RSVP issue and queue an acknowledgement without calling a model\n")
	b.WriteString("- channel-send workflow accepts optional named routes from `.gitclaw/channels/routes.yaml`\n")
	b.WriteString("- channel state and gateway workflows are callable with `workflow_dispatch`\n")
	b.WriteString("- gateway workflow can dispatch its renewal with `actions: write`\n")
	b.WriteString("- outbox workflow can read pending assistant replies with `issues: read`\n")
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
	if !surface.SendWorkflow.Present {
		findings = append(findings, "channel_send_workflow_missing")
	} else {
		if !surface.SendWorkflow.WorkflowDispatch {
			findings = append(findings, "send_workflow_dispatch_missing")
		}
		if !surface.SendWorkflow.IssuesWrite {
			findings = append(findings, "send_workflow_issues_write_missing")
		}
		if surface.SendWorkflow.Inputs < 5 {
			findings = append(findings, "send_workflow_inputs_missing")
		}
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
	if !surface.OutboxWorkflow.Present {
		findings = append(findings, "channel_outbox_workflow_missing")
	} else {
		if !surface.OutboxWorkflow.WorkflowDispatch {
			findings = append(findings, "outbox_workflow_dispatch_missing")
		}
		if !surface.OutboxWorkflow.IssuesRead && !surface.OutboxWorkflow.IssuesWrite {
			findings = append(findings, "outbox_workflow_issues_read_missing")
		}
		if surface.OutboxWorkflow.Inputs < 5 {
			findings = append(findings, "outbox_workflow_inputs_missing")
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

func isChannelListRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && (fields[0] == "/channel" || fields[0] == "/channels") && strings.EqualFold(fields[1], "list")
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
		SendWorkflow:     channelWorkflow{Path: channelSendWorkflowPath},
		StateWorkflow:    channelWorkflow{Path: channelStateWorkflowPath},
		GatewayWorkflow:  channelWorkflow{Path: channelGatewayWorkflowPath},
		DeliveryWorkflow: channelWorkflow{Path: channelDeliveryWorkflowPath},
		OutboxWorkflow:   channelWorkflow{Path: channelOutboxWorkflowPath},
		Routebook:        configSurfaceFile{Path: channelRoutesPath},
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	surface.IngestWorkflow = inspectChannelWorkflow(absRoot, channelIngestWorkflowPath)
	surface.SendWorkflow = inspectChannelWorkflow(absRoot, channelSendWorkflowPath)
	surface.StateWorkflow = inspectChannelWorkflow(absRoot, channelStateWorkflowPath)
	surface.GatewayWorkflow = inspectChannelWorkflow(absRoot, channelGatewayWorkflowPath)
	surface.DeliveryWorkflow = inspectChannelWorkflow(absRoot, channelDeliveryWorkflowPath)
	surface.OutboxWorkflow = inspectChannelWorkflow(absRoot, channelOutboxWorkflowPath)
	surface.Routebook = inspectConfigSurfaceFile(absRoot, channelRoutesPath)
	if routes, err := LoadChannelRoutes(absRoot); err == nil {
		surface.Routes = len(routes)
	}
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
	workflow.IssuesRead = strings.Contains(text, "issues: read")
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
		"- `%s` path=`%s` present=`true` bytes=`%d` lines=`%d` workflow_dispatch=`%t` actions_write=`%t` issues_read=`%t` issues_write=`%t` inputs=`%d` sha256_12=`%s`\n",
		name,
		workflow.Path,
		workflow.Bytes,
		workflow.Lines,
		workflow.WorkflowDispatch,
		workflow.ActionsWrite,
		workflow.IssuesRead,
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
