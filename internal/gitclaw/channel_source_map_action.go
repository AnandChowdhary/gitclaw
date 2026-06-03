package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSourceMapOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MapID             string
	RequestedSource   string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSourceMapResult struct {
	Notification                 ChannelSendResult
	RouteName                    string
	RouteHash                    string
	Channel                      string
	ThreadHash                   string
	MessageHash                  string
	NotifyHash                   string
	MapIDHash                    string
	RequestedSourceHash          string
	NoteHash                     string
	BodyHash                     string
	StepHash                     string
	SnapshotHash                 string
	StepCount                    int
	SkillSourceStatus            string
	AvailableSourcePins          int
	ParsedSourcePins             int
	MatchedSourcePins            int
	MissingSkillMatches          int
	HashPinnedSources            int
	HashMatchedSources           int
	HashMismatchedSources        int
	RepoLocalSourceRefs          int
	RemoteSourceRefs             int
	SourcesRequiringApproval     int
	RemoteFetchAllowedSpecs      int
	SourcesWithRiskFindings      int
	HighRiskFindings             int
	WarningRiskFindings          int
	InfoRiskFindings             int
	SelectedSourcePins           int
	SelectedSkillMatched         int
	SelectedHashPinned           int
	SelectedHashMatched          int
	SelectedHashMismatched       int
	SelectedRequiresApproval     int
	SelectedRemoteFetchAllowed   int
	SelectedRiskFindings         int
	SelectedSourceNamesHash      string
	SelectedSourcePathsHash      string
	SelectedSkillPathsHash       string
	SelectedSourceKindsHash      string
	SelectedTrustLevelsHash      string
	SelectedInstallModesHash     string
	SelectedSourceRefHashesHash  string
	SelectedExpectedHashesHash   string
	SelectedCurrentSkillHashHash string
}

type ChannelSourceMapActionRequest struct {
	Options                      ChannelSourceMapOptions
	Command                      string
	Subcommand                   string
	AutoSourceMessageID          bool
	AutoNotifyMessageID          bool
	AutoMapID                    bool
	TargetFromIssue              bool
	NoteSource                   string
	SourcePinSource              string
	RequestedRouteHash           string
	RequestedThreadHash          string
	RequestedMsgHash             string
	NotifyMessageHash            string
	MapIDHash                    string
	RequestedSourceHash          string
	NormalizedSourceHash         string
	RequestedSourceBytes         int
	RequestedSourceTerms         int
	NoteSHA                      string
	NoteBytes                    int
	NoteLines                    int
	StepSHA                      string
	SnapshotSHA                  string
	StepCount                    int
	NotificationBodySHA          string
	SkillSourceStatus            string
	AvailableSourcePins          int
	ParsedSourcePins             int
	MatchedSourcePins            int
	MissingSkillMatches          int
	HashPinnedSources            int
	HashMatchedSources           int
	HashMismatchedSources        int
	RepoLocalSourceRefs          int
	RemoteSourceRefs             int
	SourcesRequiringApproval     int
	RemoteFetchAllowedSpecs      int
	SourcesWithRiskFindings      int
	HighRiskFindings             int
	WarningRiskFindings          int
	InfoRiskFindings             int
	SelectedSourcePins           int
	SelectedSkillMatched         int
	SelectedHashPinned           int
	SelectedHashMatched          int
	SelectedHashMismatched       int
	SelectedRequiresApproval     int
	SelectedRemoteFetchAllowed   int
	SelectedRiskFindings         int
	SelectedSourceNamesHash      string
	SelectedSourcePathsHash      string
	SelectedSkillPathsHash       string
	SelectedSourceKindsHash      string
	SelectedTrustLevelsHash      string
	SelectedInstallModesHash     string
	SelectedSourceRefHashesHash  string
	SelectedExpectedHashesHash   string
	SelectedCurrentSkillHashHash string
}

type channelSourceMapSnapshot struct {
	SkillSourceStatus          string
	AvailableSourcePins        int
	ParsedSourcePins           int
	MatchedSourcePins          int
	MissingSkillMatches        int
	HashPinnedSources          int
	HashMatchedSources         int
	HashMismatchedSources      int
	RepoLocalSourceRefs        int
	RemoteSourceRefs           int
	SourcesRequiringApproval   int
	RemoteFetchAllowedSpecs    int
	SourcesWithRiskFindings    int
	HighRiskFindings           int
	WarningRiskFindings        int
	InfoRiskFindings           int
	SelectedSourcePins         int
	SelectedSkillMatched       int
	SelectedHashPinned         int
	SelectedHashMatched        int
	SelectedHashMismatched     int
	SelectedRequiresApproval   int
	SelectedRemoteFetchAllowed int
	SelectedRiskFindings       int
	SelectedSourceNames        []string
	SelectedSourcePaths        []string
	SelectedSkillPaths         []string
	SelectedSourceKinds        []string
	SelectedTrustLevels        []string
	SelectedInstallModes       []string
	SelectedSourceRefHashes    []string
	SelectedExpectedHashes     []string
	SelectedCurrentSkillHashes []string
	StepCount                  int
	StepHash                   string
	SnapshotHash               string
}

type channelSourceMapStep struct {
	Command string
	Reason  string
}

func IsChannelSourceMapActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelSourceMapActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelSourceMapActionFields(fields)
}

func isChannelSourceMapActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSourceMapSubcommand(fields[1]) {
	case "source-map", "sources-map", "skill-source-map", "skill-sources-map", "source-pin-map", "source-path", "source-runbook", "source-safety", "safe-source":
		return true
	default:
		return false
	}
}

func BuildChannelSourceMapActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSourceMapActionRequest, error) {
	fields, trailing, ok := channelSourceMapActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSourceMapActionRequest{}, fmt.Errorf("missing channel source map command")
	}
	req := ChannelSourceMapActionRequest{
		Options: ChannelSourceMapOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSourceMapSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var sourceParts []string
	var positional []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--map-id", "--source-map-id", "--runbook-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MapID = cleanChannelSourceMapID(fields[i+1])
			i++
		case "--source", "--pin", "--name", "-s":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			sourceParts = append(sourceParts, fields[i+1])
			req.SourcePinSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSourceMapActionRequest{}, fmt.Errorf("unknown channel source map argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelSourceMapIssueTargetIfPresent(ev, &req)
	if req.SourcePinSource == "" {
		req.SourcePinSource = "positional"
	}
	if err := applyChannelSourceMapPositionals(&req, positional, &sourceParts); err != nil {
		return ChannelSourceMapActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RequestedSource) == "" {
		req.Options.RequestedSource = cleanChannelSourceMapName(strings.Join(sourceParts, " "))
	}
	if strings.TrimSpace(req.Options.RequestedSource) == "" {
		req.Options.RequestedSource = parseChannelSourceMapTrailingSource(trailing)
		if req.Options.RequestedSource != "" {
			req.SourcePinSource = "trailing-source"
		}
	}
	if err := applyChannelSourceMapIssueTarget(ev, &req); err != nil {
		return ChannelSourceMapActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelSourceMapTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSourceMapSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MapID) == "" {
		req.Options.MapID = autoChannelSourceMapID(ev, req.Options)
		req.AutoMapID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSourceMapNotifyMessageID(ev, req.Options.MapID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSourceMapOptions(req.Options)
	if err := validateChannelSourceMapActionRequestOptions(req.Options); err != nil {
		return ChannelSourceMapActionRequest{}, err
	}
	snapshot := buildChannelSourceMapSnapshot(cfg, repoContext, req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MapIDHash = shortDocumentHash(req.Options.MapID)
	req.RequestedSourceHash = shortDocumentHash(req.Options.RequestedSource)
	req.NormalizedSourceHash = shortDocumentHash(normalizeSkillSourceName(req.Options.RequestedSource))
	req.RequestedSourceBytes = len(req.Options.RequestedSource)
	req.RequestedSourceTerms = len(memorySearchTerms(req.Options.RequestedSource))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelSourceMapNotificationBody(cfg, req.Options, repoContext))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelSourceMap(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSourceMapOptions, repoContext RepoContext) (ChannelSourceMapResult, error) {
	opts = normalizeChannelSourceMapOptions(opts)
	var err error
	opts, err = applyChannelSourceMapRoute(cfg, opts)
	if err != nil {
		return ChannelSourceMapResult{}, err
	}
	if err := validateChannelSourceMapOptions(opts); err != nil {
		return ChannelSourceMapResult{}, err
	}
	body := renderChannelSourceMapNotificationBody(cfg, opts, repoContext)
	snapshot := buildChannelSourceMapSnapshot(cfg, repoContext, opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelSourceMapResult{}, fmt.Errorf("queue channel source map notification: %w", err)
	}
	result := ChannelSourceMapResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		MapIDHash:           shortDocumentHash(opts.MapID),
		RequestedSourceHash: shortDocumentHash(opts.RequestedSource),
		NoteHash:            shortDocumentHash(opts.Note),
		BodyHash:            shortDocumentHash(body),
		StepHash:            snapshot.StepHash,
		SnapshotHash:        snapshot.SnapshotHash,
		StepCount:           snapshot.StepCount,
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelSourceMapActionReport(ev Event, req ChannelSourceMapActionRequest, result ChannelSourceMapResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	mapIDHash := firstNonEmpty(result.MapIDHash, req.MapIDHash)
	requestedSourceHash := firstNonEmpty(result.RequestedSourceHash, req.RequestedSourceHash)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	stepHash := firstNonEmpty(result.StepHash, req.StepSHA)
	snapshotHash := firstNonEmpty(result.SnapshotHash, req.SnapshotSHA)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Source Map Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_source_map_status: `%s`\n", status)
	fmt.Fprintf(&b, "- source_map_mode: `%s`\n", "provider-facing-skill-source-sequence")
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- source_message_id_auto: `%t`\n", req.AutoSourceMessageID)
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- source_map_id_sha256_12: `%s`\n", noneIfEmpty(mapIDHash))
	fmt.Fprintf(&b, "- source_map_id_auto: `%t`\n", req.AutoMapID)
	fmt.Fprintf(&b, "- requested_source_sha256_12: `%s`\n", noneIfEmpty(requestedSourceHash))
	fmt.Fprintf(&b, "- normalized_source_sha256_12: `%s`\n", noneIfEmpty(req.NormalizedSourceHash))
	fmt.Fprintf(&b, "- requested_source_bytes: `%d`\n", req.RequestedSourceBytes)
	fmt.Fprintf(&b, "- requested_source_terms: `%d`\n", req.RequestedSourceTerms)
	fmt.Fprintf(&b, "- source_pin_source: `%s`\n", noneIfEmpty(req.SourcePinSource))
	fmt.Fprintf(&b, "- source_map_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- source_map_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- source_map_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- source_map_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- source_map_step_count: `%d`\n", nonzeroOrReq(result.StepCount, req.StepCount))
	fmt.Fprintf(&b, "- source_map_step_sha256_12: `%s`\n", noneIfEmpty(stepHash))
	fmt.Fprintf(&b, "- source_map_snapshot_sha256_12: `%s`\n", noneIfEmpty(snapshotHash))
	fmt.Fprintf(&b, "- skill_source_status: `%s`\n", firstNonEmpty(result.SkillSourceStatus, req.SkillSourceStatus, "unknown"))
	fmt.Fprintf(&b, "- available_source_pins: `%d`\n", nonzeroOrReq(result.AvailableSourcePins, req.AvailableSourcePins))
	fmt.Fprintf(&b, "- parsed_source_pins: `%d`\n", result.ParsedSourcePins)
	fmt.Fprintf(&b, "- matched_source_pins: `%d`\n", result.MatchedSourcePins)
	fmt.Fprintf(&b, "- missing_skill_matches: `%d`\n", result.MissingSkillMatches)
	fmt.Fprintf(&b, "- hash_pinned_sources: `%d`\n", result.HashPinnedSources)
	fmt.Fprintf(&b, "- hash_matched_sources: `%d`\n", result.HashMatchedSources)
	fmt.Fprintf(&b, "- hash_mismatched_sources: `%d`\n", result.HashMismatchedSources)
	fmt.Fprintf(&b, "- repo_local_source_refs: `%d`\n", result.RepoLocalSourceRefs)
	fmt.Fprintf(&b, "- remote_source_refs: `%d`\n", result.RemoteSourceRefs)
	fmt.Fprintf(&b, "- sources_requiring_approval: `%d`\n", result.SourcesRequiringApproval)
	fmt.Fprintf(&b, "- remote_fetch_allowed_specs: `%d`\n", result.RemoteFetchAllowedSpecs)
	fmt.Fprintf(&b, "- sources_with_risk_findings: `%d`\n", result.SourcesWithRiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", result.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", result.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", result.InfoRiskFindings)
	fmt.Fprintf(&b, "- selected_source_pins: `%d`\n", result.SelectedSourcePins)
	fmt.Fprintf(&b, "- selected_skill_matched: `%d`\n", result.SelectedSkillMatched)
	fmt.Fprintf(&b, "- selected_hash_pinned: `%d`\n", result.SelectedHashPinned)
	fmt.Fprintf(&b, "- selected_hash_matched: `%d`\n", result.SelectedHashMatched)
	fmt.Fprintf(&b, "- selected_hash_mismatched: `%d`\n", result.SelectedHashMismatched)
	fmt.Fprintf(&b, "- selected_requires_approval: `%d`\n", result.SelectedRequiresApproval)
	fmt.Fprintf(&b, "- selected_remote_fetch_allowed: `%d`\n", result.SelectedRemoteFetchAllowed)
	fmt.Fprintf(&b, "- selected_risk_findings: `%d`\n", result.SelectedRiskFindings)
	fmt.Fprintf(&b, "- selected_source_names_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedSourceNamesHash, req.SelectedSourceNamesHash)))
	fmt.Fprintf(&b, "- selected_source_paths_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedSourcePathsHash, req.SelectedSourcePathsHash)))
	fmt.Fprintf(&b, "- selected_skill_paths_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedSkillPathsHash, req.SelectedSkillPathsHash)))
	fmt.Fprintf(&b, "- selected_source_kinds_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedSourceKindsHash, req.SelectedSourceKindsHash)))
	fmt.Fprintf(&b, "- selected_trust_levels_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedTrustLevelsHash, req.SelectedTrustLevelsHash)))
	fmt.Fprintf(&b, "- selected_install_modes_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedInstallModesHash, req.SelectedInstallModesHash)))
	fmt.Fprintf(&b, "- selected_source_ref_hashes_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedSourceRefHashesHash, req.SelectedSourceRefHashesHash)))
	fmt.Fprintf(&b, "- selected_expected_hashes_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedExpectedHashesHash, req.SelectedExpectedHashesHash)))
	fmt.Fprintf(&b, "- selected_current_skill_hashes_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedCurrentSkillHashHash, req.SelectedCurrentSkillHashHash)))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- remote_fetch_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_pin_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- dependency_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_proposal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_map_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_source_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_map_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_map_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_refs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_source_map_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing skill-source map on the canonical channel issue. This is a safe source-pin sequence card for Slack or Telegram users who want to move from source provenance to reviewed verification and proposal workflows: it reports compact source-pin metadata and the next reviewed commands, but it does not contact registries, fetch remote sources, install or update skills, write source pins, run installers, install dependencies, create source proposal issues, call a model, mutate workflows, mutate the repository, or call provider APIs. The source receipt keeps thread ids, message ids, map ids, requested source names, notes, step text, raw source names, paths, refs, bodies, skill paths, skill bodies, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read source-map cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent source-map cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate source-map cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelSourceMapActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSourceMapActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSourceMapIssueTarget(ev Event, req *ChannelSourceMapActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel source map requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelSourceMapIssueTargetIfPresent(ev Event, req *ChannelSourceMapActionRequest) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
}

func applyChannelSourceMapPositionals(req *ChannelSourceMapActionRequest, positional []string, sourceParts *[]string) error {
	if req == nil {
		return nil
	}
	for i, value := range positional {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !req.TargetFromIssue && req.Options.Route == "" && req.Options.Channel == "" && len(*sourceParts) == 0 && len(positional)-i > 1 {
			req.Options.Route = value
			continue
		}
		if len(*sourceParts) == 0 {
			*sourceParts = append(*sourceParts, value)
			continue
		}
		if req.Options.MapID == "" {
			req.Options.MapID = cleanChannelSourceMapID(value)
			continue
		}
		return fmt.Errorf("unexpected channel source map argument %q", value)
	}
	return nil
}

func normalizeChannelSourceMapOptions(opts ChannelSourceMapOptions) ChannelSourceMapOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MapID = cleanChannelSourceMapID(opts.MapID)
	opts.RequestedSource = cleanChannelSourceMapName(opts.RequestedSource)
	opts.Note = cleanChannelSourceMapNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSourceMapRoute(cfg Config, opts ChannelSourceMapOptions) (ChannelSourceMapOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      "GitClaw channel skill source map.",
	})
	if err != nil {
		return opts, err
	}
	opts.Route = routeOpts.Route
	opts.Channel = routeOpts.Channel
	opts.ThreadID = routeOpts.ThreadID
	opts.Author = routeOpts.Author
	return opts, nil
}

func validateChannelSourceMapOptions(opts ChannelSourceMapOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.MapID == "" {
		return fmt.Errorf("missing source map id")
	}
	if !skillNamePattern.MatchString(opts.MapID) {
		return fmt.Errorf("invalid source map id %q", opts.MapID)
	}
	if opts.RequestedSource == "" {
		return fmt.Errorf("missing requested source")
	}
	return nil
}

func validateChannelSourceMapActionRequestOptions(opts ChannelSourceMapOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Route == "" && (opts.Channel == "" || opts.ThreadID == "") {
		return fmt.Errorf("missing channel route or channel thread target")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.MapID == "" {
		return fmt.Errorf("missing source map id")
	}
	if !skillNamePattern.MatchString(opts.MapID) {
		return fmt.Errorf("invalid source map id %q", opts.MapID)
	}
	if opts.RequestedSource == "" {
		return fmt.Errorf("missing requested source")
	}
	return nil
}

func cleanChannelSourceMapSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSourceMapID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSourceMapName(value string) string {
	value = normalizeSkillSourceName(value)
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func cleanChannelSourceMapNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelSourceMapTrailingSource(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"source:", "skill-source:", "pin:", "name:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelSourceMapName(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func parseChannelSourceMapTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelSourceMapTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelSourceMapNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelSourceMapTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "source:") ||
		strings.HasPrefix(lower, "skill-source:") ||
		strings.HasPrefix(lower, "pin:") ||
		strings.HasPrefix(lower, "name:") ||
		strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func autoChannelSourceMapSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-source-map-source-%s", eventID(ev))
}

func autoChannelSourceMapID(ev Event, opts ChannelSourceMapOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.RequestedSource, opts.Note}, "|")
	return cleanChannelSourceMapID(fmt.Sprintf("source-map-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSourceMapNotifyMessageID(ev Event, mapID string) string {
	seed := strings.Join([]string{eventID(ev), mapID}, "|")
	return fmt.Sprintf("gitclaw-channel-source-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelSourceMapNotificationBody(cfg Config, opts ChannelSourceMapOptions, repoContext RepoContext) string {
	opts = normalizeChannelSourceMapOptions(opts)
	snapshot := buildChannelSourceMapSnapshot(cfg, repoContext, opts)
	steps := channelSourceMapStepsForSource(opts.RequestedSource, snapshot)
	report := BuildSkillSourceReport(cfg, repoContext)
	matches := matchingSkillSourceCards(report.Cards, opts.RequestedSource)
	var b strings.Builder
	b.WriteString("GitClaw channel skill source map.\n\n")
	fmt.Fprintf(&b, "Requested source: %s\n", opts.RequestedSource)
	fmt.Fprintf(&b, "Skill source status: %s\n", report.Status)
	fmt.Fprintf(&b, "Available source pins: %d\n", report.Specs)
	fmt.Fprintf(&b, "Parsed source pins: %d\n", report.ParsedSpecs)
	fmt.Fprintf(&b, "Matched source pins: %d\n", len(matches))
	fmt.Fprintf(&b, "Hash pinned sources: %d\n", report.HashPinnedSources)
	fmt.Fprintf(&b, "Hash matched sources: %d\n", report.HashMatchedSources)
	fmt.Fprintf(&b, "Hash mismatched sources: %d\n", report.HashMismatchedSources)
	fmt.Fprintf(&b, "Remote fetch allowed pins: %d\n", report.RemoteFetchAllowedSpecs)
	fmt.Fprintf(&b, "Sources requiring approval: %d\n", report.SourcesRequiringApproval)
	fmt.Fprintf(&b, "Sources with risk findings: %d\n", report.SourcesWithRiskFindings)
	if len(matches) == 0 {
		b.WriteString("\nMatched source: none\n")
	} else {
		for _, card := range matches {
			fmt.Fprintf(&b, "\nMatched source: %s\n", card.Name)
			fmt.Fprintf(&b, "Source kind: %s\n", card.SourceKind)
			fmt.Fprintf(&b, "Source ref present: %t\n", card.SourceRefPresent)
			fmt.Fprintf(&b, "Source ref hash: %s\n", card.SourceRefSHA)
			fmt.Fprintf(&b, "Skill matched: %t\n", card.SkillMatched)
			fmt.Fprintf(&b, "Trust level: %s\n", card.TrustLevel)
			fmt.Fprintf(&b, "Install mode: %s\n", card.InstallMode)
			fmt.Fprintf(&b, "Requires approval: %t\n", card.RequiresApproval)
			fmt.Fprintf(&b, "Remote fetch allowed: %t\n", card.RemoteFetchAllowed)
			fmt.Fprintf(&b, "Hash pinned: %t\n", card.HashPinned)
			fmt.Fprintf(&b, "Hash matched: %t\n", card.HashMatched)
			fmt.Fprintf(&b, "Hash mismatched: %t\n", card.HashMismatched)
			fmt.Fprintf(&b, "Risk findings: %d\n", len(card.RiskFindings))
		}
	}
	b.WriteString("\nSource sequence:\n")
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. `%s` - %s\n", i+1, step.Command, step.Reason)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	fmt.Fprintf(&b, "Source map hash: %s\n", snapshot.SnapshotHash)
	fmt.Fprintf(&b, "Source step hash: %s\n", snapshot.StepHash)
	b.WriteString("\nMap source: current GitHub Actions checkout skill-source metadata.\n")
	b.WriteString("Raw source refs: not included.\n")
	b.WriteString("Full source bodies: not included.\n")
	b.WriteString("Full skill bodies: not included.\n")
	b.WriteString("Registry contact: not performed by this action.\n")
	b.WriteString("Remote fetch: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Skill update: not performed by this action.\n")
	b.WriteString("Source pin write: not performed by this action.\n")
	b.WriteString("Installer scripts: not run by this action.\n")
	b.WriteString("Dependency install: not performed by this action.\n")
	b.WriteString("Source proposal issue creation: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelSourceMapSnapshot(cfg Config, repoContext RepoContext, opts ChannelSourceMapOptions) channelSourceMapSnapshot {
	report := BuildSkillSourceReport(cfg, repoContext)
	matches := matchingSkillSourceCards(report.Cards, opts.RequestedSource)
	snapshot := channelSourceMapSnapshot{
		SkillSourceStatus:        report.Status,
		AvailableSourcePins:      report.Specs,
		ParsedSourcePins:         report.ParsedSpecs,
		MatchedSourcePins:        report.MatchedSources,
		MissingSkillMatches:      report.MissingSkillMatches,
		HashPinnedSources:        report.HashPinnedSources,
		HashMatchedSources:       report.HashMatchedSources,
		HashMismatchedSources:    report.HashMismatchedSources,
		RepoLocalSourceRefs:      report.RepoLocalSourceRefs,
		RemoteSourceRefs:         report.RemoteSourceRefs,
		SourcesRequiringApproval: report.SourcesRequiringApproval,
		RemoteFetchAllowedSpecs:  report.RemoteFetchAllowedSpecs,
		SourcesWithRiskFindings:  report.SourcesWithRiskFindings,
		HighRiskFindings:         report.HighRiskFindings,
		WarningRiskFindings:      report.WarningRiskFindings,
		InfoRiskFindings:         report.InfoRiskFindings,
		SelectedSourcePins:       len(matches),
	}
	for _, card := range matches {
		snapshot.SelectedSourceNames = append(snapshot.SelectedSourceNames, card.Name)
		snapshot.SelectedSourcePaths = append(snapshot.SelectedSourcePaths, card.Path)
		snapshot.SelectedSkillPaths = append(snapshot.SelectedSkillPaths, card.SkillPath)
		snapshot.SelectedSourceKinds = append(snapshot.SelectedSourceKinds, card.SourceKind)
		snapshot.SelectedTrustLevels = append(snapshot.SelectedTrustLevels, card.TrustLevel)
		snapshot.SelectedInstallModes = append(snapshot.SelectedInstallModes, card.InstallMode)
		snapshot.SelectedSourceRefHashes = append(snapshot.SelectedSourceRefHashes, card.SourceRefSHA)
		snapshot.SelectedExpectedHashes = append(snapshot.SelectedExpectedHashes, card.ExpectedSHA)
		snapshot.SelectedCurrentSkillHashes = append(snapshot.SelectedCurrentSkillHashes, card.SkillSHA)
		if card.SkillMatched {
			snapshot.SelectedSkillMatched++
		}
		if card.HashPinned {
			snapshot.SelectedHashPinned++
		}
		if card.HashMatched {
			snapshot.SelectedHashMatched++
		}
		if card.HashMismatched {
			snapshot.SelectedHashMismatched++
		}
		if card.RequiresApproval {
			snapshot.SelectedRequiresApproval++
		}
		if card.RemoteFetchAllowed {
			snapshot.SelectedRemoteFetchAllowed++
		}
		snapshot.SelectedRiskFindings += len(card.RiskFindings)
	}
	snapshot.SelectedSourceNames = uniqueSortedStrings(snapshot.SelectedSourceNames)
	snapshot.SelectedSourcePaths = uniqueSortedStrings(snapshot.SelectedSourcePaths)
	snapshot.SelectedSkillPaths = uniqueSortedStrings(snapshot.SelectedSkillPaths)
	snapshot.SelectedSourceKinds = uniqueSortedStrings(snapshot.SelectedSourceKinds)
	snapshot.SelectedTrustLevels = uniqueSortedStrings(snapshot.SelectedTrustLevels)
	snapshot.SelectedInstallModes = uniqueSortedStrings(snapshot.SelectedInstallModes)
	snapshot.SelectedSourceRefHashes = uniqueSortedStrings(snapshot.SelectedSourceRefHashes)
	snapshot.SelectedExpectedHashes = uniqueSortedStrings(snapshot.SelectedExpectedHashes)
	snapshot.SelectedCurrentSkillHashes = uniqueSortedStrings(snapshot.SelectedCurrentSkillHashes)
	steps := channelSourceMapStepsForSource(opts.RequestedSource, snapshot)
	snapshot.StepCount = len(steps)
	snapshot.StepHash = shortDocumentHash(channelSourceMapStepManifest(steps))
	snapshot.SnapshotHash = shortDocumentHash(channelSourceMapSnapshotManifest(opts, snapshot))
	return snapshot
}

func channelSourceMapStepsForSource(source string, snapshot channelSourceMapSnapshot) []channelSourceMapStep {
	source = cleanChannelSourceMapName(source)
	if source == "" {
		source = "<source>"
	}
	return []channelSourceMapStep{
		{Command: "/skills sources", Reason: fmt.Sprintf("confirm the reviewed source-pin inventory first (%d available source pins)", snapshot.AvailableSourcePins)},
		{Command: fmt.Sprintf("/skills sources info %s", source), Reason: "inspect the focused source pin without printing source refs or bodies"},
		{Command: "/skills sources verify", Reason: "check hash and trust gates without contacting registries or fetching remotes"},
		{Command: "/skills sources lock", Reason: "derive reproducibility status without writing a lockfile"},
		{Command: "/skills sources update-plan", Reason: "review stale or unpinned source candidates without fetching remote state"},
		{Command: fmt.Sprintf("/skills sources propose %s --source <ref> --id <proposal-id>", source), Reason: "open a reviewed source-pin proposal only when a human supplies a source ref"},
	}
}

func channelSourceMapStepManifest(steps []channelSourceMapStep) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, strings.Join([]string{step.Command, step.Reason}, "|"))
	}
	return strings.Join(lines, "\n")
}

func channelSourceMapSnapshotManifest(opts ChannelSourceMapOptions, snapshot channelSourceMapSnapshot) string {
	return fmt.Sprintf(
		"source=%s\nstatus=%s\ncounts=%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d\nhashes=%s/%s/%s/%s/%s/%s/%s/%s/%s\nsteps=%d/%s\nnote=%s",
		shortDocumentHash(cleanChannelSourceMapName(opts.RequestedSource)),
		snapshot.SkillSourceStatus,
		snapshot.AvailableSourcePins,
		snapshot.ParsedSourcePins,
		snapshot.MatchedSourcePins,
		snapshot.MissingSkillMatches,
		snapshot.HashPinnedSources,
		snapshot.HashMatchedSources,
		snapshot.HashMismatchedSources,
		snapshot.RepoLocalSourceRefs,
		snapshot.RemoteSourceRefs,
		snapshot.SourcesRequiringApproval,
		snapshot.RemoteFetchAllowedSpecs,
		snapshot.SourcesWithRiskFindings,
		snapshot.HighRiskFindings,
		snapshot.WarningRiskFindings,
		snapshot.InfoRiskFindings,
		snapshot.SelectedSourcePins,
		snapshot.SelectedSkillMatched,
		snapshot.SelectedHashPinned,
		snapshot.SelectedHashMatched,
		snapshot.SelectedHashMismatched,
		snapshot.SelectedRequiresApproval,
		snapshot.SelectedRemoteFetchAllowed,
		snapshot.SelectedRiskFindings,
		hashStringList(snapshot.SelectedSourceNames),
		hashStringList(snapshot.SelectedSourcePaths),
		hashStringList(snapshot.SelectedSkillPaths),
		hashStringList(snapshot.SelectedSourceKinds),
		hashStringList(snapshot.SelectedTrustLevels),
		hashStringList(snapshot.SelectedInstallModes),
		hashStringList(snapshot.SelectedSourceRefHashes),
		hashStringList(snapshot.SelectedExpectedHashes),
		hashStringList(snapshot.SelectedCurrentSkillHashes),
		snapshot.StepCount,
		snapshot.StepHash,
		shortDocumentHash(opts.Note),
	)
}

func (r *ChannelSourceMapActionRequest) applySnapshot(snapshot channelSourceMapSnapshot) {
	r.SkillSourceStatus = snapshot.SkillSourceStatus
	r.AvailableSourcePins = snapshot.AvailableSourcePins
	r.ParsedSourcePins = snapshot.ParsedSourcePins
	r.MatchedSourcePins = snapshot.MatchedSourcePins
	r.MissingSkillMatches = snapshot.MissingSkillMatches
	r.HashPinnedSources = snapshot.HashPinnedSources
	r.HashMatchedSources = snapshot.HashMatchedSources
	r.HashMismatchedSources = snapshot.HashMismatchedSources
	r.RepoLocalSourceRefs = snapshot.RepoLocalSourceRefs
	r.RemoteSourceRefs = snapshot.RemoteSourceRefs
	r.SourcesRequiringApproval = snapshot.SourcesRequiringApproval
	r.RemoteFetchAllowedSpecs = snapshot.RemoteFetchAllowedSpecs
	r.SourcesWithRiskFindings = snapshot.SourcesWithRiskFindings
	r.HighRiskFindings = snapshot.HighRiskFindings
	r.WarningRiskFindings = snapshot.WarningRiskFindings
	r.InfoRiskFindings = snapshot.InfoRiskFindings
	r.SelectedSourcePins = snapshot.SelectedSourcePins
	r.SelectedSkillMatched = snapshot.SelectedSkillMatched
	r.SelectedHashPinned = snapshot.SelectedHashPinned
	r.SelectedHashMatched = snapshot.SelectedHashMatched
	r.SelectedHashMismatched = snapshot.SelectedHashMismatched
	r.SelectedRequiresApproval = snapshot.SelectedRequiresApproval
	r.SelectedRemoteFetchAllowed = snapshot.SelectedRemoteFetchAllowed
	r.SelectedRiskFindings = snapshot.SelectedRiskFindings
	r.SelectedSourceNamesHash = hashStringList(snapshot.SelectedSourceNames)
	r.SelectedSourcePathsHash = hashStringList(snapshot.SelectedSourcePaths)
	r.SelectedSkillPathsHash = hashStringList(snapshot.SelectedSkillPaths)
	r.SelectedSourceKindsHash = hashStringList(snapshot.SelectedSourceKinds)
	r.SelectedTrustLevelsHash = hashStringList(snapshot.SelectedTrustLevels)
	r.SelectedInstallModesHash = hashStringList(snapshot.SelectedInstallModes)
	r.SelectedSourceRefHashesHash = hashStringList(snapshot.SelectedSourceRefHashes)
	r.SelectedExpectedHashesHash = hashStringList(snapshot.SelectedExpectedHashes)
	r.SelectedCurrentSkillHashHash = hashStringList(snapshot.SelectedCurrentSkillHashes)
	r.StepCount = snapshot.StepCount
	r.StepSHA = snapshot.StepHash
	r.SnapshotSHA = snapshot.SnapshotHash
}

func (r *ChannelSourceMapResult) applySnapshot(snapshot channelSourceMapSnapshot) {
	r.SkillSourceStatus = snapshot.SkillSourceStatus
	r.AvailableSourcePins = snapshot.AvailableSourcePins
	r.ParsedSourcePins = snapshot.ParsedSourcePins
	r.MatchedSourcePins = snapshot.MatchedSourcePins
	r.MissingSkillMatches = snapshot.MissingSkillMatches
	r.HashPinnedSources = snapshot.HashPinnedSources
	r.HashMatchedSources = snapshot.HashMatchedSources
	r.HashMismatchedSources = snapshot.HashMismatchedSources
	r.RepoLocalSourceRefs = snapshot.RepoLocalSourceRefs
	r.RemoteSourceRefs = snapshot.RemoteSourceRefs
	r.SourcesRequiringApproval = snapshot.SourcesRequiringApproval
	r.RemoteFetchAllowedSpecs = snapshot.RemoteFetchAllowedSpecs
	r.SourcesWithRiskFindings = snapshot.SourcesWithRiskFindings
	r.HighRiskFindings = snapshot.HighRiskFindings
	r.WarningRiskFindings = snapshot.WarningRiskFindings
	r.InfoRiskFindings = snapshot.InfoRiskFindings
	r.SelectedSourcePins = snapshot.SelectedSourcePins
	r.SelectedSkillMatched = snapshot.SelectedSkillMatched
	r.SelectedHashPinned = snapshot.SelectedHashPinned
	r.SelectedHashMatched = snapshot.SelectedHashMatched
	r.SelectedHashMismatched = snapshot.SelectedHashMismatched
	r.SelectedRequiresApproval = snapshot.SelectedRequiresApproval
	r.SelectedRemoteFetchAllowed = snapshot.SelectedRemoteFetchAllowed
	r.SelectedRiskFindings = snapshot.SelectedRiskFindings
	r.SelectedSourceNamesHash = hashStringList(snapshot.SelectedSourceNames)
	r.SelectedSourcePathsHash = hashStringList(snapshot.SelectedSourcePaths)
	r.SelectedSkillPathsHash = hashStringList(snapshot.SelectedSkillPaths)
	r.SelectedSourceKindsHash = hashStringList(snapshot.SelectedSourceKinds)
	r.SelectedTrustLevelsHash = hashStringList(snapshot.SelectedTrustLevels)
	r.SelectedInstallModesHash = hashStringList(snapshot.SelectedInstallModes)
	r.SelectedSourceRefHashesHash = hashStringList(snapshot.SelectedSourceRefHashes)
	r.SelectedExpectedHashesHash = hashStringList(snapshot.SelectedExpectedHashes)
	r.SelectedCurrentSkillHashHash = hashStringList(snapshot.SelectedCurrentSkillHashes)
	r.StepCount = snapshot.StepCount
}
