package gitclaw

import (
	"fmt"
	"strings"
)

const (
	defaultApprovedLabel   = "gitclaw:approved"
	defaultNeedsHumanLabel = "gitclaw:needs-human"
)

func IsApprovalReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/approvals" || command == "/approval"
}

func RenderApprovalReport(ev Event, cfg Config, decision PreflightDecision, transcript []TranscriptMessage, writeRequested bool) string {
	return RenderApprovalReportWithComments(ev, cfg, decision, nil, transcript, writeRequested)
}

func RenderApprovalReportWithComments(ev Event, cfg Config, decision PreflightDecision, comments []Comment, transcript []TranscriptMessage, writeRequested bool) string {
	if isApprovalCatalogRequest(ev, cfg) {
		return RenderApprovalCatalogReport(ev, cfg)
	}
	if isApprovalRiskRequest(ev, cfg) {
		return renderApprovalRiskReport(ev, cfg, decision, transcript, writeRequested, true)
	}
	if isApprovalProvenanceRequest(ev, cfg) {
		return renderApprovalProvenanceReport(ev, cfg, decision, comments, transcript, writeRequested, true)
	}
	return renderApprovalReport(ev, cfg, decision, transcript, writeRequested, true)
}

func RenderApprovalCLIReport(cfg Config) string {
	return renderApprovalReport(Event{}, cfg, PreflightDecision{}, nil, false, false)
}

func renderApprovalReport(ev Event, cfg Config, decision PreflightDecision, transcript []TranscriptMessage, writeRequested bool, includeIssue bool) string {
	approvedLabelPresent := false
	needsHumanLabelPresent := false
	writeRequestedLabelPresent := false
	actorTrusted := false
	actor := "none"
	if includeIssue {
		approvedLabelPresent = hasLabel(ev.Issue.Labels, defaultApprovedLabel)
		needsHumanLabelPresent = hasLabel(ev.Issue.Labels, defaultNeedsHumanLabel)
		writeRequestedLabelPresent = hasLabel(ev.Issue.Labels, cfg.WriteRequestedLabel) || writeRequested
		actor = actorAssociation(ev)
		actorTrusted = trustedAssociation(actor, cfg)
	}

	var b strings.Builder
	b.WriteString("## GitClaw Approvals Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- preflight_allowed: `%t`\n", decision.Allowed)
		fmt.Fprintf(&b, "- preflight_code: `%s`\n", decision.Code)
		fmt.Fprintf(&b, "- actor_association: `%s`\n", actor)
		fmt.Fprintf(&b, "- actor_trusted: `%t`\n", actorTrusted)
		fmt.Fprintf(&b, "- triggered: `%t`\n", triggered(ev, cfg))
		fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.DisabledLabel))
		fmt.Fprintf(&b, "- write_request_detected: `%t`\n", writeRequested)
		fmt.Fprintf(&b, "- write_requested_label_present: `%t`\n", writeRequestedLabelPresent)
		fmt.Fprintf(&b, "- approved_label_present: `%t`\n", approvedLabelPresent)
		fmt.Fprintf(&b, "- needs_human_label_present: `%t`\n", needsHumanLabelPresent)
		fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- approval_status: `%s`\n", approvalStatus(includeIssue, writeRequested, approvedLabelPresent, actorTrusted))
	fmt.Fprintf(&b, "- approval_decision: `%s`\n", approvalDecision(writeRequested, approvedLabelPresent))
	fmt.Fprintf(&b, "- approval_store: `%s`\n", "github-issue-labels")
	fmt.Fprintf(&b, "- approval_scope: `%s`\n", "per-issue")
	fmt.Fprintf(&b, "- approval_label: `%s`\n", defaultApprovedLabel)
	fmt.Fprintf(&b, "- needs_human_label: `%s`\n", defaultNeedsHumanLabel)
	fmt.Fprintf(&b, "- write_requested_label: `%s`\n", cfg.WriteRequestedLabel)
	fmt.Fprintf(&b, "- write_actions_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_approval_payloads_included: `%t`\n", false)
	b.WriteByte('\n')

	b.WriteString("This report inspects the approval state GitClaw would require before future write-capable work. It does not enable writes, approve anything, execute commands, print issue bodies, print comments, or expose prompt text.\n\n")

	b.WriteString("### Approval Gates\n")
	if includeIssue {
		fmt.Fprintf(&b, "- gate=`trusted_actor` status=`%s` association=`%s`\n", gateStatus(actorTrusted), actor)
		fmt.Fprintf(&b, "- gate=`write_request` status=`%s` label=`%s`\n", writeRequestGateStatus(writeRequested), cfg.WriteRequestedLabel)
		fmt.Fprintf(&b, "- gate=`approval_label` status=`%s` label=`%s`\n", approvalLabelGateStatus(writeRequested, approvedLabelPresent), defaultApprovedLabel)
		fmt.Fprintf(&b, "- gate=`write_mode` status=`blocked` detail=`read_only_v1`\n")
	} else {
		b.WriteString("- gate=`trusted_actor` status=`configured` source=`authorization.allowed_associations`\n")
		b.WriteString("- gate=`write_request` status=`runtime_detected` label=`gitclaw:write-requested`\n")
		b.WriteString("- gate=`approval_label` status=`label_required_for_future_write_mode` label=`gitclaw:approved`\n")
		b.WriteString("- gate=`write_mode` status=`blocked` detail=`read_only_v1`\n")
	}

	b.WriteString("\n### Trusted Associations\n")
	for _, association := range sortedAllowedAssociations(cfg) {
		fmt.Fprintf(&b, "- `%s`\n", association)
	}

	b.WriteString("\n### Approval Labels\n")
	fmt.Fprintf(&b, "- `%s`\n", cfg.WriteRequestedLabel)
	fmt.Fprintf(&b, "- `%s`\n", defaultApprovedLabel)
	fmt.Fprintf(&b, "- `%s`\n", defaultNeedsHumanLabel)
	return strings.TrimSpace(b.String())
}

func approvalStatus(includeIssue, writeRequested, approvedLabelPresent, actorTrusted bool) string {
	if !includeIssue {
		return "static_policy"
	}
	if !writeRequested {
		return "not_requested"
	}
	if !actorTrusted {
		return "blocked_untrusted_actor"
	}
	if approvedLabelPresent {
		return "approved_but_write_mode_disabled"
	}
	return "waiting_for_approval"
}

func approvalDecision(writeRequested, approvedLabelPresent bool) string {
	if !writeRequested {
		return "no_write_requested"
	}
	if approvedLabelPresent {
		return "proposal_only_approved_label_seen"
	}
	return "proposal_only_needs_approval"
}

func gateStatus(ok bool) string {
	if ok {
		return "passed"
	}
	return "blocked"
}

func writeRequestGateStatus(writeRequested bool) string {
	if writeRequested {
		return "detected"
	}
	return "not_requested"
}

func approvalLabelGateStatus(writeRequested, approvedLabelPresent bool) string {
	if !writeRequested {
		return "not_required"
	}
	if approvedLabelPresent {
		return "present"
	}
	return "missing"
}
