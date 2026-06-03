package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelBundleMapOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MapID             string
	RequestedBundle   string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBundleMapResult struct {
	Notification                  ChannelSendResult
	RouteName                     string
	RouteHash                     string
	Channel                       string
	ThreadHash                    string
	MessageHash                   string
	NotifyHash                    string
	MapIDHash                     string
	RequestedBundleHash           string
	NoteHash                      string
	BodyHash                      string
	StepHash                      string
	SnapshotHash                  string
	StepCount                     int
	AvailableBundles              int
	MatchedBundles                int
	SelectedBundles               int
	BundleSkillRefs               int
	ResolvedBundleSkills          int
	MissingBundleSkills           int
	BundlesWithInstruction        int
	BundlesWithParseErrors        int
	BundlesWithRiskFindings       int
	SelectedBundleSkillRefs       int
	SelectedBundleResolvedSkills  int
	SelectedBundleMissingSkills   int
	SelectedBundleNamesHash       string
	SelectedBundlePathsHash       string
	SelectedBundleSkillsHash      string
	SelectedResolvedSkillsHash    string
	SelectedMissingSkillsHash     string
	SelectedInstructionHashesHash string
}

type ChannelBundleMapActionRequest struct {
	Options                       ChannelBundleMapOptions
	Command                       string
	Subcommand                    string
	AutoSourceMessageID           bool
	AutoNotifyMessageID           bool
	AutoMapID                     bool
	TargetFromIssue               bool
	NoteSource                    string
	BundleSource                  string
	RequestedRouteHash            string
	RequestedThreadHash           string
	RequestedMsgHash              string
	NotifyMessageHash             string
	MapIDHash                     string
	RequestedBundleHash           string
	NormalizedBundleHash          string
	RequestedBundleBytes          int
	RequestedBundleTerms          int
	NoteSHA                       string
	NoteBytes                     int
	NoteLines                     int
	StepSHA                       string
	SnapshotSHA                   string
	StepCount                     int
	NotificationBodySHA           string
	AvailableBundles              int
	MatchedBundles                int
	SelectedBundles               int
	BundleSkillRefs               int
	ResolvedBundleSkills          int
	MissingBundleSkills           int
	BundlesWithInstruction        int
	BundlesWithParseErrors        int
	BundlesWithRiskFindings       int
	SelectedBundleSkillRefs       int
	SelectedBundleResolvedSkills  int
	SelectedBundleMissingSkills   int
	SelectedBundleNamesHash       string
	SelectedBundlePathsHash       string
	SelectedBundleSkillsHash      string
	SelectedResolvedSkillsHash    string
	SelectedMissingSkillsHash     string
	SelectedInstructionHashesHash string
}

type channelBundleMapSnapshot struct {
	AvailableBundles             int
	MatchedBundles               int
	SelectedBundles              int
	BundleSkillRefs              int
	ResolvedBundleSkills         int
	MissingBundleSkills          int
	BundlesWithInstruction       int
	BundlesWithParseErrors       int
	BundlesWithRiskFindings      int
	SelectedBundleNames          []string
	SelectedBundlePaths          []string
	SelectedBundleSkills         []string
	SelectedResolvedSkills       []string
	SelectedMissingSkills        []string
	SelectedInstructionHashes    []string
	SelectedBundleSkillRefs      int
	SelectedBundleResolvedSkills int
	SelectedBundleMissingSkills  int
	StepCount                    int
	StepHash                     string
	SnapshotHash                 string
}

type channelBundleMapStep struct {
	Command string
	Reason  string
}

func IsChannelBundleMapActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelBundleMapActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelBundleMapActionFields(fields)
}

func isChannelBundleMapActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelBundleMapSubcommand(fields[1]) {
	case "bundle-map", "bundles-map", "skill-bundle-map", "skill-bundles-map", "bundle-path", "bundle-flow", "bundle-runbook", "bundle-safety", "safe-bundle":
		return true
	default:
		return false
	}
}

func BuildChannelBundleMapActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelBundleMapActionRequest, error) {
	fields, trailing, ok := channelBundleMapActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBundleMapActionRequest{}, fmt.Errorf("missing channel bundle map command")
	}
	req := ChannelBundleMapActionRequest{
		Options: ChannelBundleMapOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelBundleMapSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var bundleParts []string
	var positional []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--map-id", "--bundle-map-id", "--runbook-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MapID = cleanChannelBundleMapID(fields[i+1])
			i++
		case "--bundle", "--name", "-b":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			bundleParts = append(bundleParts, fields[i+1])
			req.BundleSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBundleMapActionRequest{}, fmt.Errorf("unknown channel bundle map argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelBundleMapIssueTargetIfPresent(ev, &req)
	if req.BundleSource == "" {
		req.BundleSource = "positional"
	}
	if err := applyChannelBundleMapPositionals(&req, positional, &bundleParts); err != nil {
		return ChannelBundleMapActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RequestedBundle) == "" {
		req.Options.RequestedBundle = cleanChannelBundleMapName(strings.Join(bundleParts, " "))
	}
	if strings.TrimSpace(req.Options.RequestedBundle) == "" {
		req.Options.RequestedBundle = parseChannelBundleMapTrailingBundle(trailing)
		if req.Options.RequestedBundle != "" {
			req.BundleSource = "trailing-bundle"
		}
	}
	if err := applyChannelBundleMapIssueTarget(ev, &req); err != nil {
		return ChannelBundleMapActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelBundleMapTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBundleMapSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MapID) == "" {
		req.Options.MapID = autoChannelBundleMapID(ev, req.Options)
		req.AutoMapID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBundleMapNotifyMessageID(ev, req.Options.MapID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBundleMapOptions(req.Options)
	if err := validateChannelBundleMapActionRequestOptions(req.Options); err != nil {
		return ChannelBundleMapActionRequest{}, err
	}
	snapshot := buildChannelBundleMapSnapshot(repoContext, req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MapIDHash = shortDocumentHash(req.Options.MapID)
	req.RequestedBundleHash = shortDocumentHash(req.Options.RequestedBundle)
	req.NormalizedBundleHash = shortDocumentHash(normalizeSkillBundleName(req.Options.RequestedBundle))
	req.RequestedBundleBytes = len(req.Options.RequestedBundle)
	req.RequestedBundleTerms = len(memorySearchTerms(req.Options.RequestedBundle))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelBundleMapNotificationBody(req.Options, repoContext))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelBundleMap(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBundleMapOptions, repoContext RepoContext) (ChannelBundleMapResult, error) {
	opts = normalizeChannelBundleMapOptions(opts)
	var err error
	opts, err = applyChannelBundleMapRoute(cfg, opts)
	if err != nil {
		return ChannelBundleMapResult{}, err
	}
	if err := validateChannelBundleMapOptions(opts); err != nil {
		return ChannelBundleMapResult{}, err
	}
	body := renderChannelBundleMapNotificationBody(opts, repoContext)
	snapshot := buildChannelBundleMapSnapshot(repoContext, opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelBundleMapResult{}, fmt.Errorf("queue channel bundle map notification: %w", err)
	}
	result := ChannelBundleMapResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		MapIDHash:           shortDocumentHash(opts.MapID),
		RequestedBundleHash: shortDocumentHash(opts.RequestedBundle),
		NoteHash:            shortDocumentHash(opts.Note),
		BodyHash:            shortDocumentHash(body),
		StepHash:            snapshot.StepHash,
		SnapshotHash:        snapshot.SnapshotHash,
		StepCount:           snapshot.StepCount,
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelBundleMapActionReport(ev Event, req ChannelBundleMapActionRequest, result ChannelBundleMapResult) string {
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
	requestedBundleHash := firstNonEmpty(result.RequestedBundleHash, req.RequestedBundleHash)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	stepHash := firstNonEmpty(result.StepHash, req.StepSHA)
	snapshotHash := firstNonEmpty(result.SnapshotHash, req.SnapshotSHA)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Bundle Map Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_bundle_map_status: `%s`\n", status)
	fmt.Fprintf(&b, "- bundle_map_mode: `%s`\n", "provider-facing-bundle-sequence")
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
	fmt.Fprintf(&b, "- bundle_map_id_sha256_12: `%s`\n", noneIfEmpty(mapIDHash))
	fmt.Fprintf(&b, "- bundle_map_id_auto: `%t`\n", req.AutoMapID)
	fmt.Fprintf(&b, "- requested_bundle_sha256_12: `%s`\n", noneIfEmpty(requestedBundleHash))
	fmt.Fprintf(&b, "- normalized_bundle_sha256_12: `%s`\n", noneIfEmpty(req.NormalizedBundleHash))
	fmt.Fprintf(&b, "- requested_bundle_bytes: `%d`\n", req.RequestedBundleBytes)
	fmt.Fprintf(&b, "- requested_bundle_terms: `%d`\n", req.RequestedBundleTerms)
	fmt.Fprintf(&b, "- bundle_source: `%s`\n", noneIfEmpty(req.BundleSource))
	fmt.Fprintf(&b, "- bundle_map_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- bundle_map_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- bundle_map_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- bundle_map_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- bundle_map_step_count: `%d`\n", nonzeroOrReq(result.StepCount, req.StepCount))
	fmt.Fprintf(&b, "- bundle_map_step_sha256_12: `%s`\n", noneIfEmpty(stepHash))
	fmt.Fprintf(&b, "- bundle_map_snapshot_sha256_12: `%s`\n", noneIfEmpty(snapshotHash))
	fmt.Fprintf(&b, "- available_bundles: `%d`\n", nonzeroOrReq(result.AvailableBundles, req.AvailableBundles))
	fmt.Fprintf(&b, "- matched_bundles: `%d`\n", result.MatchedBundles)
	fmt.Fprintf(&b, "- selected_bundles: `%d`\n", result.SelectedBundles)
	fmt.Fprintf(&b, "- bundle_skill_refs: `%d`\n", result.BundleSkillRefs)
	fmt.Fprintf(&b, "- resolved_bundle_skills: `%d`\n", result.ResolvedBundleSkills)
	fmt.Fprintf(&b, "- missing_bundle_skills: `%d`\n", result.MissingBundleSkills)
	fmt.Fprintf(&b, "- bundles_with_instruction: `%d`\n", result.BundlesWithInstruction)
	fmt.Fprintf(&b, "- bundles_with_parse_errors: `%d`\n", result.BundlesWithParseErrors)
	fmt.Fprintf(&b, "- bundles_with_risk_findings: `%d`\n", result.BundlesWithRiskFindings)
	fmt.Fprintf(&b, "- selected_bundle_skill_refs: `%d`\n", result.SelectedBundleSkillRefs)
	fmt.Fprintf(&b, "- selected_bundle_resolved_skills: `%d`\n", result.SelectedBundleResolvedSkills)
	fmt.Fprintf(&b, "- selected_bundle_missing_skills: `%d`\n", result.SelectedBundleMissingSkills)
	fmt.Fprintf(&b, "- selected_bundle_names_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedBundleNamesHash, req.SelectedBundleNamesHash)))
	fmt.Fprintf(&b, "- selected_bundle_paths_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedBundlePathsHash, req.SelectedBundlePathsHash)))
	fmt.Fprintf(&b, "- selected_bundle_skills_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedBundleSkillsHash, req.SelectedBundleSkillsHash)))
	fmt.Fprintf(&b, "- selected_resolved_skills_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedResolvedSkillsHash, req.SelectedResolvedSkillsHash)))
	fmt.Fprintf(&b, "- selected_missing_skills_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedMissingSkillsHash, req.SelectedMissingSkillsHash)))
	fmt.Fprintf(&b, "- selected_instruction_hashes_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedInstructionHashesHash, req.SelectedInstructionHashesHash)))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- bundle_enable_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- bundle_yaml_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- bundle_proposal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- bundle_rehearsal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_proposal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_map_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_bundle_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_map_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_map_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_descriptions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_instructions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_bundle_map_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing bundle map on the canonical channel issue. This is a safe bundle sequence card for Slack or Telegram users who want to move from a repo-local skill-bundle profile to reviewed skill and rehearsal workflows: it reports compact bundle metadata and the next reviewed commands, but it does not install skills, update skills, enable bundles, write bundle YAML, run installers, create bundle proposal issues, create bundle rehearsal issues, create skill proposal issues, call a model, mutate workflows, mutate the repository, or call provider APIs. The source receipt keeps thread ids, message ids, map ids, requested bundle names, notes, step text, raw bundle names, paths, descriptions, instructions, bundle bodies, skill names, skill paths, skill bodies, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read bundle-map cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent bundle-map cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate bundle-map cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelBundleMapActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBundleMapActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBundleMapIssueTarget(ev Event, req *ChannelBundleMapActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel bundle map requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelBundleMapIssueTargetIfPresent(ev Event, req *ChannelBundleMapActionRequest) {
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

func applyChannelBundleMapPositionals(req *ChannelBundleMapActionRequest, positional []string, bundleParts *[]string) error {
	if req == nil {
		return nil
	}
	for i, value := range positional {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !req.TargetFromIssue && req.Options.Route == "" && req.Options.Channel == "" && len(*bundleParts) == 0 && len(positional)-i > 1 {
			req.Options.Route = value
			continue
		}
		if len(*bundleParts) == 0 {
			*bundleParts = append(*bundleParts, value)
			continue
		}
		if req.Options.MapID == "" {
			req.Options.MapID = cleanChannelBundleMapID(value)
			continue
		}
		return fmt.Errorf("unexpected channel bundle map argument %q", value)
	}
	return nil
}

func normalizeChannelBundleMapOptions(opts ChannelBundleMapOptions) ChannelBundleMapOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MapID = cleanChannelBundleMapID(opts.MapID)
	opts.RequestedBundle = cleanChannelBundleMapName(opts.RequestedBundle)
	opts.Note = cleanChannelBundleMapNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelBundleMapRoute(cfg Config, opts ChannelBundleMapOptions) (ChannelBundleMapOptions, error) {
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
		Body:      "GitClaw channel bundle map.",
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

func validateChannelBundleMapOptions(opts ChannelBundleMapOptions) error {
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
		return fmt.Errorf("missing bundle map id")
	}
	if !skillNamePattern.MatchString(opts.MapID) {
		return fmt.Errorf("invalid bundle map id %q", opts.MapID)
	}
	if opts.RequestedBundle == "" {
		return fmt.Errorf("missing requested bundle")
	}
	return nil
}

func validateChannelBundleMapActionRequestOptions(opts ChannelBundleMapOptions) error {
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
		return fmt.Errorf("missing bundle map id")
	}
	if !skillNamePattern.MatchString(opts.MapID) {
		return fmt.Errorf("invalid bundle map id %q", opts.MapID)
	}
	if opts.RequestedBundle == "" {
		return fmt.Errorf("missing requested bundle")
	}
	return nil
}

func cleanChannelBundleMapSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelBundleMapID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelBundleMapName(value string) string {
	value = normalizeSkillBundleName(value)
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func cleanChannelBundleMapNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelBundleMapTrailingBundle(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"bundle:", "skill-bundle:", "name:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelBundleMapName(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func parseChannelBundleMapTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelBundleMapTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelBundleMapNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelBundleMapTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "bundle:") ||
		strings.HasPrefix(lower, "skill-bundle:") ||
		strings.HasPrefix(lower, "name:") ||
		strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func autoChannelBundleMapSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-bundle-map-source-%s", eventID(ev))
}

func autoChannelBundleMapID(ev Event, opts ChannelBundleMapOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.RequestedBundle, opts.Note}, "|")
	return cleanChannelBundleMapID(fmt.Sprintf("bundle-map-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelBundleMapNotifyMessageID(ev Event, mapID string) string {
	seed := strings.Join([]string{eventID(ev), mapID}, "|")
	return fmt.Sprintf("gitclaw-channel-bundle-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelBundleMapNotificationBody(opts ChannelBundleMapOptions, repoContext RepoContext) string {
	opts = normalizeChannelBundleMapOptions(opts)
	snapshot := buildChannelBundleMapSnapshot(repoContext, opts)
	steps := channelBundleMapStepsForBundle(opts.RequestedBundle, snapshot)
	matches := matchingSkillBundleSummaries(repoContext.SkillBundles, opts.RequestedBundle)
	var b strings.Builder
	b.WriteString("GitClaw channel bundle map.\n\n")
	fmt.Fprintf(&b, "Requested bundle: %s\n", opts.RequestedBundle)
	fmt.Fprintf(&b, "Available bundles: %d\n", snapshot.AvailableBundles)
	fmt.Fprintf(&b, "Matched bundles: %d\n", snapshot.MatchedBundles)
	fmt.Fprintf(&b, "Selected bundles for this turn: %d\n", snapshot.SelectedBundles)
	fmt.Fprintf(&b, "Bundle skill refs: %d\n", snapshot.BundleSkillRefs)
	fmt.Fprintf(&b, "Resolved bundle skills: %d\n", snapshot.ResolvedBundleSkills)
	fmt.Fprintf(&b, "Missing bundle skills: %d\n", snapshot.MissingBundleSkills)
	fmt.Fprintf(&b, "Bundles with instruction: %d\n", snapshot.BundlesWithInstruction)
	fmt.Fprintf(&b, "Bundle parse errors: %d\n", snapshot.BundlesWithParseErrors)
	fmt.Fprintf(&b, "Bundle risk findings: %d\n", snapshot.BundlesWithRiskFindings)
	if len(matches) == 0 {
		b.WriteString("\nMatched bundle: none\n")
	} else {
		for _, bundle := range matches {
			fmt.Fprintf(&b, "\nMatched bundle: %s\n", bundle.Name)
			fmt.Fprintf(&b, "Bundle skills: %s\n", inlineList(bundle.Skills))
			fmt.Fprintf(&b, "Resolved skills: %s\n", inlineList(bundle.ResolvedSkills))
			fmt.Fprintf(&b, "Missing skills: %s\n", inlineList(bundle.MissingSkills))
			fmt.Fprintf(&b, "Instruction present: %t\n", bundle.InstructionPresent)
			fmt.Fprintf(&b, "Parse error present: %t\n", bundle.ParseError != "")
			fmt.Fprintf(&b, "Risk findings: %d\n", len(bundle.RiskFindings))
		}
	}
	b.WriteString("\nBundle sequence:\n")
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. `%s` - %s\n", i+1, step.Command, step.Reason)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	fmt.Fprintf(&b, "Bundle map hash: %s\n", snapshot.SnapshotHash)
	fmt.Fprintf(&b, "Bundle step hash: %s\n", snapshot.StepHash)
	b.WriteString("\nMap source: current GitHub Actions checkout skill-bundle metadata.\n")
	b.WriteString("Full bundle bodies: not included.\n")
	b.WriteString("Bundle instructions: not included.\n")
	b.WriteString("Full skill bodies: not included.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Skill update: not performed by this action.\n")
	b.WriteString("Bundle enablement: not performed by this action.\n")
	b.WriteString("Bundle YAML write: not performed by this action.\n")
	b.WriteString("Registry contact: not performed by this action.\n")
	b.WriteString("Installer scripts: not run by this action.\n")
	b.WriteString("Bundle proposal issue creation: not performed by this action.\n")
	b.WriteString("Bundle rehearsal issue creation: not performed by this action.\n")
	b.WriteString("Skill proposal issue creation: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelBundleMapSnapshot(repoContext RepoContext, opts ChannelBundleMapOptions) channelBundleMapSnapshot {
	matches := matchingSkillBundleSummaries(repoContext.SkillBundles, opts.RequestedBundle)
	snapshot := channelBundleMapSnapshot{
		AvailableBundles:        len(repoContext.SkillBundles),
		MatchedBundles:          len(matches),
		SelectedBundles:         selectedSkillBundleCount(repoContext.SkillBundles),
		BundleSkillRefs:         bundleSkillRefCount(repoContext.SkillBundles),
		ResolvedBundleSkills:    resolvedBundleSkillCount(repoContext.SkillBundles),
		MissingBundleSkills:     missingBundleSkillCount(repoContext.SkillBundles),
		BundlesWithInstruction:  bundlesWithInstructionCount(repoContext.SkillBundles),
		BundlesWithParseErrors:  bundlesWithParseErrorCount(repoContext.SkillBundles),
		BundlesWithRiskFindings: bundlesWithRiskFindingCount(repoContext.SkillBundles),
	}
	for _, bundle := range matches {
		snapshot.SelectedBundleNames = append(snapshot.SelectedBundleNames, bundle.Name)
		snapshot.SelectedBundlePaths = append(snapshot.SelectedBundlePaths, bundle.Path)
		snapshot.SelectedBundleSkills = append(snapshot.SelectedBundleSkills, bundle.Skills...)
		snapshot.SelectedResolvedSkills = append(snapshot.SelectedResolvedSkills, bundle.ResolvedSkills...)
		snapshot.SelectedMissingSkills = append(snapshot.SelectedMissingSkills, bundle.MissingSkills...)
		snapshot.SelectedInstructionHashes = append(snapshot.SelectedInstructionHashes, bundle.InstructionSHA)
		snapshot.SelectedBundleSkillRefs += len(bundle.Skills)
		snapshot.SelectedBundleResolvedSkills += len(bundle.ResolvedSkills)
		snapshot.SelectedBundleMissingSkills += len(bundle.MissingSkills)
	}
	snapshot.SelectedBundleNames = uniqueSortedStrings(snapshot.SelectedBundleNames)
	snapshot.SelectedBundlePaths = uniqueSortedStrings(snapshot.SelectedBundlePaths)
	snapshot.SelectedBundleSkills = uniqueSortedStrings(snapshot.SelectedBundleSkills)
	snapshot.SelectedResolvedSkills = uniqueSortedStrings(snapshot.SelectedResolvedSkills)
	snapshot.SelectedMissingSkills = uniqueSortedStrings(snapshot.SelectedMissingSkills)
	snapshot.SelectedInstructionHashes = uniqueSortedStrings(snapshot.SelectedInstructionHashes)
	steps := channelBundleMapStepsForBundle(opts.RequestedBundle, snapshot)
	snapshot.StepCount = len(steps)
	snapshot.StepHash = shortDocumentHash(channelBundleMapStepManifest(steps))
	snapshot.SnapshotHash = shortDocumentHash(channelBundleMapSnapshotManifest(opts, snapshot))
	return snapshot
}

func channelBundleMapStepsForBundle(bundle string, snapshot channelBundleMapSnapshot) []channelBundleMapStep {
	bundle = cleanChannelBundleMapName(bundle)
	if bundle == "" {
		bundle = "<bundle>"
	}
	skill := "<skill>"
	if len(snapshot.SelectedResolvedSkills) > 0 {
		skill = snapshot.SelectedResolvedSkills[0]
	} else if len(snapshot.SelectedBundleSkills) > 0 {
		skill = snapshot.SelectedBundleSkills[0]
	}
	return []channelBundleMapStep{
		{Command: "/channels profile --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("confirm the current channel profile and bundle count (%d available bundles)", snapshot.AvailableBundles)},
		{Command: fmt.Sprintf("/bundles info %s", bundle), Reason: "inspect the focused repo-local bundle metadata in GitHub without loading bundle bodies into the channel"},
		{Command: fmt.Sprintf("/bundles risk %s", bundle), Reason: "review parse errors, risky instructions, and missing skill references before use"},
		{Command: fmt.Sprintf("/channels skill-map %s --message-id <id> --notify-message-id <id>", skill), Reason: "map a referenced skill into a provider-facing safe sequence"},
		{Command: fmt.Sprintf("/bundles rehearse %s --id <rehearsal-id>", bundle), Reason: "open a model-backed rehearsal issue when a human wants to test the bundle path"},
		{Command: "/channels propose-bundle --bundle-id <id> --message-id <id> --notify-message-id <id>", Reason: "create a reviewed proposal issue only when a channel-origin bundle should become repo-local YAML"},
	}
}

func channelBundleMapStepManifest(steps []channelBundleMapStep) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, strings.Join([]string{step.Command, step.Reason}, "|"))
	}
	return strings.Join(lines, "\n")
}

func channelBundleMapSnapshotManifest(opts ChannelBundleMapOptions, snapshot channelBundleMapSnapshot) string {
	return fmt.Sprintf(
		"bundle=%s\ncounts=%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d\nhashes=%s/%s/%s/%s/%s/%s\nsteps=%d/%s\nnote=%s",
		shortDocumentHash(cleanChannelBundleMapName(opts.RequestedBundle)),
		snapshot.AvailableBundles,
		snapshot.MatchedBundles,
		snapshot.SelectedBundles,
		snapshot.BundleSkillRefs,
		snapshot.ResolvedBundleSkills,
		snapshot.MissingBundleSkills,
		snapshot.BundlesWithInstruction,
		snapshot.BundlesWithParseErrors,
		snapshot.BundlesWithRiskFindings,
		snapshot.SelectedBundleSkillRefs,
		snapshot.SelectedBundleResolvedSkills,
		snapshot.SelectedBundleMissingSkills,
		hashStringList(snapshot.SelectedBundleNames),
		hashStringList(snapshot.SelectedBundlePaths),
		hashStringList(snapshot.SelectedBundleSkills),
		hashStringList(snapshot.SelectedResolvedSkills),
		hashStringList(snapshot.SelectedMissingSkills),
		hashStringList(snapshot.SelectedInstructionHashes),
		snapshot.StepCount,
		snapshot.StepHash,
		shortDocumentHash(opts.Note),
	)
}

func bundlesWithParseErrorCount(bundles []SkillBundleSummary) int {
	count := 0
	for _, bundle := range bundles {
		if bundle.ParseError != "" {
			count++
		}
	}
	return count
}

func bundlesWithRiskFindingCount(bundles []SkillBundleSummary) int {
	count := 0
	for _, bundle := range bundles {
		if len(bundle.RiskFindings) > 0 {
			count++
		}
	}
	return count
}

func (r *ChannelBundleMapActionRequest) applySnapshot(snapshot channelBundleMapSnapshot) {
	r.AvailableBundles = snapshot.AvailableBundles
	r.MatchedBundles = snapshot.MatchedBundles
	r.SelectedBundles = snapshot.SelectedBundles
	r.BundleSkillRefs = snapshot.BundleSkillRefs
	r.ResolvedBundleSkills = snapshot.ResolvedBundleSkills
	r.MissingBundleSkills = snapshot.MissingBundleSkills
	r.BundlesWithInstruction = snapshot.BundlesWithInstruction
	r.BundlesWithParseErrors = snapshot.BundlesWithParseErrors
	r.BundlesWithRiskFindings = snapshot.BundlesWithRiskFindings
	r.SelectedBundleSkillRefs = snapshot.SelectedBundleSkillRefs
	r.SelectedBundleResolvedSkills = snapshot.SelectedBundleResolvedSkills
	r.SelectedBundleMissingSkills = snapshot.SelectedBundleMissingSkills
	r.SelectedBundleNamesHash = hashStringList(snapshot.SelectedBundleNames)
	r.SelectedBundlePathsHash = hashStringList(snapshot.SelectedBundlePaths)
	r.SelectedBundleSkillsHash = hashStringList(snapshot.SelectedBundleSkills)
	r.SelectedResolvedSkillsHash = hashStringList(snapshot.SelectedResolvedSkills)
	r.SelectedMissingSkillsHash = hashStringList(snapshot.SelectedMissingSkills)
	r.SelectedInstructionHashesHash = hashStringList(snapshot.SelectedInstructionHashes)
	r.StepCount = snapshot.StepCount
	r.StepSHA = snapshot.StepHash
	r.SnapshotSHA = snapshot.SnapshotHash
}

func (r *ChannelBundleMapResult) applySnapshot(snapshot channelBundleMapSnapshot) {
	r.AvailableBundles = snapshot.AvailableBundles
	r.MatchedBundles = snapshot.MatchedBundles
	r.SelectedBundles = snapshot.SelectedBundles
	r.BundleSkillRefs = snapshot.BundleSkillRefs
	r.ResolvedBundleSkills = snapshot.ResolvedBundleSkills
	r.MissingBundleSkills = snapshot.MissingBundleSkills
	r.BundlesWithInstruction = snapshot.BundlesWithInstruction
	r.BundlesWithParseErrors = snapshot.BundlesWithParseErrors
	r.BundlesWithRiskFindings = snapshot.BundlesWithRiskFindings
	r.SelectedBundleSkillRefs = snapshot.SelectedBundleSkillRefs
	r.SelectedBundleResolvedSkills = snapshot.SelectedBundleResolvedSkills
	r.SelectedBundleMissingSkills = snapshot.SelectedBundleMissingSkills
	r.SelectedBundleNamesHash = hashStringList(snapshot.SelectedBundleNames)
	r.SelectedBundlePathsHash = hashStringList(snapshot.SelectedBundlePaths)
	r.SelectedBundleSkillsHash = hashStringList(snapshot.SelectedBundleSkills)
	r.SelectedResolvedSkillsHash = hashStringList(snapshot.SelectedResolvedSkills)
	r.SelectedMissingSkillsHash = hashStringList(snapshot.SelectedMissingSkills)
	r.SelectedInstructionHashesHash = hashStringList(snapshot.SelectedInstructionHashes)
	r.StepCount = snapshot.StepCount
}
