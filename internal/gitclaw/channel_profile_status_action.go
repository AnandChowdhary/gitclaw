package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelProfileStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelProfileStatusResult struct {
	Notification                    ChannelSendResult
	RouteName                       string
	RouteHash                       string
	Channel                         string
	ThreadHash                      string
	MessageHash                     string
	NotifyHash                      string
	StatusIDHash                    string
	BodyHash                        string
	ProfileStatus                   string
	ProfileStrategy                 string
	ProfileStore                    string
	ProfileScope                    string
	SnapshotVersion                 string
	SnapshotScope                   string
	SnapshotSHA                     string
	SnapshotEntries                 int
	ProfileDocumentsLoaded          int
	RequiredProfileDocuments        int
	RequiredProfileDocumentsPresent int
	RequiredProfileDocumentsMissing int
	AvailableSkills                 int
	SelectedSkills                  int
	SkillBundles                    int
	AvailableTools                  int
	ActiveToolOutputs               int
	ManifestEntries                 int
	ProfileManifestSHA              string
	SoulSnapshotSHA                 string
	MemorySnapshotSHA               string
	SkillSnapshotSHA                string
	ToolSnapshotSHA                 string
	SoulStatus                      string
	MemoryStatus                    string
	SkillStatus                     string
	ToolStatus                      string
	ProfileExportSupported          bool
	ProfileImportSupported          bool
	ProfileSwitchingSupported       bool
	ProfileMutationAllowed          bool
	CredentialsIncluded             bool
	SessionsIncluded                bool
	BackupPayloadsIncluded          bool
}

type ChannelProfileStatusActionRequest struct {
	Options                         ChannelProfileStatusOptions
	Command                         string
	Subcommand                      string
	AutoSourceMessageID             bool
	AutoNotifyMessageID             bool
	AutoStatusID                    bool
	TargetFromIssue                 bool
	RequestedRouteHash              string
	RequestedThreadHash             string
	RequestedMsgHash                string
	NotifyMessageHash               string
	StatusIDHash                    string
	NotificationBodySHA             string
	ProfileStatus                   string
	ProfileStrategy                 string
	ProfileStore                    string
	ProfileScope                    string
	SnapshotVersion                 string
	SnapshotScope                   string
	SnapshotSHA                     string
	SnapshotEntries                 int
	ProfileDocumentsLoaded          int
	RequiredProfileDocuments        int
	RequiredProfileDocumentsPresent int
	RequiredProfileDocumentsMissing int
	AvailableSkills                 int
	SelectedSkills                  int
	SkillBundles                    int
	AvailableTools                  int
	ActiveToolOutputs               int
	ManifestEntries                 int
	ProfileManifestSHA              string
	SoulSnapshotSHA                 string
	MemorySnapshotSHA               string
	SkillSnapshotSHA                string
	ToolSnapshotSHA                 string
	SoulStatus                      string
	MemoryStatus                    string
	SkillStatus                     string
	ToolStatus                      string
	ProfileExportSupported          bool
	ProfileImportSupported          bool
	ProfileSwitchingSupported       bool
	ProfileMutationAllowed          bool
	CredentialsIncluded             bool
	SessionsIncluded                bool
	BackupPayloadsIncluded          bool
}

type channelProfileStatusSnapshot struct {
	ProfileStatus                   string
	ProfileStrategy                 string
	ProfileStore                    string
	ProfileScope                    string
	SnapshotVersion                 string
	SnapshotScope                   string
	SnapshotSHA                     string
	SnapshotEntries                 int
	ProfileDocumentsLoaded          int
	RequiredProfileDocuments        int
	RequiredProfileDocumentsPresent int
	RequiredProfileDocumentsMissing int
	AvailableSkills                 int
	SelectedSkills                  int
	SkillBundles                    int
	AvailableTools                  int
	ActiveToolOutputs               int
	ManifestEntries                 int
	ProfileManifestSHA              string
	SoulSnapshotSHA                 string
	MemorySnapshotSHA               string
	SkillSnapshotSHA                string
	ToolSnapshotSHA                 string
	SoulStatus                      string
	MemoryStatus                    string
	SkillStatus                     string
	ToolStatus                      string
	ProfileExportSupported          bool
	ProfileImportSupported          bool
	ProfileSwitchingSupported       bool
	ProfileMutationAllowed          bool
	CredentialsIncluded             bool
	SessionsIncluded                bool
	BackupPayloadsIncluded          bool
}

func IsChannelProfileStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelProfileStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelProfileStatusActionFields(fields)
}

func isChannelProfileStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "profile-status", "profile-snapshot", "profile-health", "agent-profile", "repo-profile", "context-profile", "context-status":
		return true
	default:
		return false
	}
}

func BuildChannelProfileStatusActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelProfileStatusActionRequest, error) {
	fields, _, ok := channelProfileStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelProfileStatusActionRequest{}, fmt.Errorf("missing channel profile status command")
	}
	req := ChannelProfileStatusActionRequest{
		Options:    ChannelProfileStatusOptions{Repo: ev.Repo},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelProfileStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelProfileStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelProfileStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelProfileStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelProfileStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--profile-status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelProfileStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelProfileStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelProfileStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelProfileStatusActionRequest{}, fmt.Errorf("unknown channel profile status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelProfileStatusActionRequest{}, fmt.Errorf("unexpected channel profile status argument %q", field)
		}
	}
	if err := applyChannelProfileStatusIssueTarget(ev, &req); err != nil {
		return ChannelProfileStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelProfileStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelProfileStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelProfileStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelProfileStatusOptions(req.Options)
	if err := validateChannelProfileStatusActionRequestOptions(req.Options); err != nil {
		return ChannelProfileStatusActionRequest{}, err
	}
	snapshot := buildChannelProfileStatusSnapshot(cfg, repoContext)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelProfileStatusNotificationBody(snapshot))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelProfileStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelProfileStatusOptions, repoContext RepoContext) (ChannelProfileStatusResult, error) {
	opts = normalizeChannelProfileStatusOptions(opts)
	var err error
	opts, err = applyChannelProfileStatusRoute(cfg, opts)
	if err != nil {
		return ChannelProfileStatusResult{}, err
	}
	if err := validateChannelProfileStatusOptions(opts); err != nil {
		return ChannelProfileStatusResult{}, err
	}
	snapshot := buildChannelProfileStatusSnapshot(cfg, repoContext)
	body := renderChannelProfileStatusNotificationBody(snapshot)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelProfileStatusResult{}, fmt.Errorf("queue channel profile status notification: %w", err)
	}
	result := ChannelProfileStatusResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		StatusIDHash: shortDocumentHash(opts.StatusID),
		BodyHash:     shortDocumentHash(body),
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelProfileStatusActionReport(ev Event, req ChannelProfileStatusActionRequest, result ChannelProfileStatusResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	threadHash := result.ThreadHash
	if threadHash == "" {
		threadHash = req.RequestedThreadHash
	}
	messageHash := result.MessageHash
	if messageHash == "" {
		messageHash = req.RequestedMsgHash
	}
	notifyHash := result.NotifyHash
	if notifyHash == "" {
		notifyHash = req.NotifyMessageHash
	}
	statusIDHash := result.StatusIDHash
	if statusIDHash == "" {
		statusIDHash = req.StatusIDHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Profile Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_profile_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- profile_snapshot_mode: `%s`\n", "provider-facing-profile-status")
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
	fmt.Fprintf(&b, "- profile_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- profile_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- profile_status: `%s`\n", firstNonEmpty(result.ProfileStatus, req.ProfileStatus, "unknown"))
	fmt.Fprintf(&b, "- profile_strategy: `%s`\n", firstNonEmpty(result.ProfileStrategy, req.ProfileStrategy, "repo-local-git-profile"))
	fmt.Fprintf(&b, "- profile_store: `%s`\n", firstNonEmpty(result.ProfileStore, req.ProfileStore, ".gitclaw/"))
	fmt.Fprintf(&b, "- profile_scope: `%s`\n", firstNonEmpty(result.ProfileScope, req.ProfileScope, "repository"))
	fmt.Fprintf(&b, "- snapshot_version: `%s`\n", firstNonEmpty(result.SnapshotVersion, req.SnapshotVersion, profileSnapshotVersion))
	fmt.Fprintf(&b, "- snapshot_scope: `%s`\n", firstNonEmpty(result.SnapshotScope, req.SnapshotScope, "repo-local-profile-soul-memory-skills-tools"))
	fmt.Fprintf(&b, "- snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SnapshotSHA, req.SnapshotSHA)))
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", nonzeroOrReq(result.SnapshotEntries, req.SnapshotEntries))
	fmt.Fprintf(&b, "- profile_documents_loaded: `%d`\n", nonzeroOrReq(result.ProfileDocumentsLoaded, req.ProfileDocumentsLoaded))
	fmt.Fprintf(&b, "- required_profile_documents: `%d`\n", nonzeroOrReq(result.RequiredProfileDocuments, req.RequiredProfileDocuments))
	fmt.Fprintf(&b, "- required_profile_documents_present: `%d`\n", nonzeroOrReq(result.RequiredProfileDocumentsPresent, req.RequiredProfileDocumentsPresent))
	fmt.Fprintf(&b, "- required_profile_documents_missing: `%d`\n", result.RequiredProfileDocumentsMissing)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", result.AvailableSkills)
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", result.SelectedSkills)
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", result.SkillBundles)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", result.AvailableTools)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", result.ActiveToolOutputs)
	fmt.Fprintf(&b, "- manifest_entries: `%d`\n", result.ManifestEntries)
	fmt.Fprintf(&b, "- profile_manifest_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.ProfileManifestSHA, req.ProfileManifestSHA)))
	fmt.Fprintf(&b, "- soul_snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SoulSnapshotSHA, req.SoulSnapshotSHA)))
	fmt.Fprintf(&b, "- memory_snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.MemorySnapshotSHA, req.MemorySnapshotSHA)))
	fmt.Fprintf(&b, "- skill_snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SkillSnapshotSHA, req.SkillSnapshotSHA)))
	fmt.Fprintf(&b, "- tool_snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.ToolSnapshotSHA, req.ToolSnapshotSHA)))
	fmt.Fprintf(&b, "- soul_status: `%s`\n", firstNonEmpty(result.SoulStatus, req.SoulStatus, "unknown"))
	fmt.Fprintf(&b, "- memory_status: `%s`\n", firstNonEmpty(result.MemoryStatus, req.MemoryStatus, "unknown"))
	fmt.Fprintf(&b, "- skill_status: `%s`\n", firstNonEmpty(result.SkillStatus, req.SkillStatus, "unknown"))
	fmt.Fprintf(&b, "- tool_status: `%s`\n", firstNonEmpty(result.ToolStatus, req.ToolStatus, "unknown"))
	fmt.Fprintf(&b, "- profile_export_supported: `%t`\n", result.ProfileExportSupported || req.ProfileExportSupported)
	fmt.Fprintf(&b, "- profile_import_supported: `%t`\n", result.ProfileImportSupported || req.ProfileImportSupported)
	fmt.Fprintf(&b, "- profile_switching_supported: `%t`\n", result.ProfileSwitchingSupported || req.ProfileSwitchingSupported)
	fmt.Fprintf(&b, "- profile_mutation_allowed: `%t`\n", result.ProfileMutationAllowed || req.ProfileMutationAllowed)
	fmt.Fprintf(&b, "- credentials_included: `%t`\n", result.CredentialsIncluded || req.CredentialsIncluded)
	fmt.Fprintf(&b, "- sessions_included: `%t`\n", result.SessionsIncluded || req.SessionsIncluded)
	fmt.Fprintf(&b, "- backup_payloads_included: `%t`\n", result.BackupPayloadsIncluded || req.BackupPayloadsIncluded)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- profile_export_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_import_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_switch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_profile_home_accessed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_profile_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_profile_file_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_profile_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_session_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_profile_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing profile status snapshot on the canonical channel issue. This is the GitHub-native channel version of a profile health card: it reports the repo-local profile snapshot, manifest, soul, memory, skills, and tools as counts and hashes, but it does not export profiles, import profiles, switch profiles, read external agent homes, call a model, mutate repository files, or call provider APIs. The source receipt keeps thread ids, message ids, status ids, profile file paths, profile bodies, skill bodies, memory bodies, tool outputs, sessions, backup payloads, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the profile-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent profile-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate profile-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/profile snapshot`, `/profile manifest`, `/soul snapshot`, `/memory snapshot`, `/skills snapshot`, or `/tools snapshot` on GitHub for deeper body-free reports\n")
	return strings.TrimSpace(b.String())
}

func channelProfileStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelProfileStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelProfileStatusIssueTarget(ev Event, req *ChannelProfileStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel profile status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelProfileStatusOptions(opts ChannelProfileStatusOptions) ChannelProfileStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelProfileStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelProfileStatusRoute(cfg Config, opts ChannelProfileStatusOptions) (ChannelProfileStatusOptions, error) {
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
		Body:      "GitClaw channel profile status.",
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

func validateChannelProfileStatusOptions(opts ChannelProfileStatusOptions) error {
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
	if opts.StatusID == "" {
		return fmt.Errorf("missing profile status id")
	}
	return nil
}

func validateChannelProfileStatusActionRequestOptions(opts ChannelProfileStatusOptions) error {
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
	if opts.StatusID == "" {
		return fmt.Errorf("missing profile status id")
	}
	return nil
}

func cleanChannelProfileStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelProfileStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-profile-source-%s", eventID(ev))
}

func autoChannelProfileStatusID(ev Event, opts ChannelProfileStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("profile-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelProfileStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-profile-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelProfileStatusNotificationBody(snapshot channelProfileStatusSnapshot) string {
	var b strings.Builder
	b.WriteString("GitClaw channel profile status.\n\n")
	fmt.Fprintf(&b, "Profile status: %s\n", snapshot.ProfileStatus)
	fmt.Fprintf(&b, "Profile store: %s\n", snapshot.ProfileStore)
	fmt.Fprintf(&b, "Profile scope: %s\n", snapshot.ProfileScope)
	fmt.Fprintf(&b, "Snapshot version: %s\n", snapshot.SnapshotVersion)
	fmt.Fprintf(&b, "Snapshot hash: %s\n", snapshot.SnapshotSHA)
	fmt.Fprintf(&b, "Snapshot entries: %d\n", snapshot.SnapshotEntries)
	fmt.Fprintf(&b, "Profile documents loaded: %d\n", snapshot.ProfileDocumentsLoaded)
	fmt.Fprintf(&b, "Required profile documents: %d/%d present\n", snapshot.RequiredProfileDocumentsPresent, snapshot.RequiredProfileDocuments)
	fmt.Fprintf(&b, "Available skills: %d\n", snapshot.AvailableSkills)
	fmt.Fprintf(&b, "Selected skills: %d\n", snapshot.SelectedSkills)
	fmt.Fprintf(&b, "Skill bundles: %d\n", snapshot.SkillBundles)
	fmt.Fprintf(&b, "Available tools: %d\n", snapshot.AvailableTools)
	fmt.Fprintf(&b, "Active tool outputs: %d\n", snapshot.ActiveToolOutputs)
	fmt.Fprintf(&b, "Components: manifest=%s soul=%s memory=%s skills=%s tools=%s\n", snapshot.ProfileStatus, snapshot.SoulStatus, snapshot.MemoryStatus, snapshot.SkillStatus, snapshot.ToolStatus)
	b.WriteString("\nProfile export: disabled.\n")
	b.WriteString("Profile import: disabled.\n")
	b.WriteString("Profile switching: disabled.\n")
	b.WriteString("Profile mutation: disabled.\n")
	b.WriteString("External profile home: not accessed.\n")
	b.WriteString("Credentials: not included.\n")
	b.WriteString("Sessions: not included.\n")
	b.WriteString("Backup payloads: not included.\n")
	b.WriteString("Raw profile bodies: not included.\n")
	b.WriteString("Raw skill bodies: not included.\n")
	b.WriteString("Raw memory bodies: not included.\n")
	b.WriteString("Raw tool outputs: not included.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelProfileStatusSnapshot(cfg Config, repoContext RepoContext) channelProfileStatusSnapshot {
	report := BuildProfileSnapshotReport(cfg, repoContext)
	return channelProfileStatusSnapshot{
		ProfileStatus:                   report.Status,
		ProfileStrategy:                 report.Manifest.ProfileStrategy,
		ProfileStore:                    report.Manifest.ProfileStore,
		ProfileScope:                    report.Manifest.ProfileScope,
		SnapshotVersion:                 report.SnapshotVersion,
		SnapshotScope:                   report.SnapshotScope,
		SnapshotSHA:                     report.SnapshotSHA,
		SnapshotEntries:                 report.SnapshotEntries,
		ProfileDocumentsLoaded:          report.ProfileDocumentsLoaded,
		RequiredProfileDocuments:        report.Manifest.RequiredProfileDocuments,
		RequiredProfileDocumentsPresent: report.Manifest.RequiredProfileDocumentsPresent,
		RequiredProfileDocumentsMissing: report.Manifest.RequiredProfileDocumentsMissing,
		AvailableSkills:                 report.AvailableSkills,
		SelectedSkills:                  report.SelectedSkills,
		SkillBundles:                    report.SkillBundles,
		AvailableTools:                  report.AvailableTools,
		ActiveToolOutputs:               report.ActiveToolOutputs,
		ManifestEntries:                 report.ManifestEntries,
		ProfileManifestSHA:              report.Manifest.ManifestSHA,
		SoulSnapshotSHA:                 report.Soul.SnapshotSHA,
		MemorySnapshotSHA:               report.Memory.SnapshotSHA,
		SkillSnapshotSHA:                report.Skills.SnapshotSHA,
		ToolSnapshotSHA:                 report.Tools.SnapshotSHA,
		SoulStatus:                      report.Soul.Status,
		MemoryStatus:                    report.Memory.Status,
		SkillStatus:                     report.Skills.Status,
		ToolStatus:                      report.Tools.Status,
		ProfileExportSupported:          report.ProfileExportSupported,
		ProfileImportSupported:          report.ProfileImportSupported,
		ProfileSwitchingSupported:       report.ProfileSwitchingSupported,
		ProfileMutationAllowed:          report.ProfileMutationAllowed,
		CredentialsIncluded:             report.CredentialsIncluded,
		SessionsIncluded:                report.SessionsIncluded,
		BackupPayloadsIncluded:          report.BackupPayloadsIncluded,
	}
}

func (r *ChannelProfileStatusActionRequest) applySnapshot(snapshot channelProfileStatusSnapshot) {
	r.ProfileStatus = snapshot.ProfileStatus
	r.ProfileStrategy = snapshot.ProfileStrategy
	r.ProfileStore = snapshot.ProfileStore
	r.ProfileScope = snapshot.ProfileScope
	r.SnapshotVersion = snapshot.SnapshotVersion
	r.SnapshotScope = snapshot.SnapshotScope
	r.SnapshotSHA = snapshot.SnapshotSHA
	r.SnapshotEntries = snapshot.SnapshotEntries
	r.ProfileDocumentsLoaded = snapshot.ProfileDocumentsLoaded
	r.RequiredProfileDocuments = snapshot.RequiredProfileDocuments
	r.RequiredProfileDocumentsPresent = snapshot.RequiredProfileDocumentsPresent
	r.RequiredProfileDocumentsMissing = snapshot.RequiredProfileDocumentsMissing
	r.AvailableSkills = snapshot.AvailableSkills
	r.SelectedSkills = snapshot.SelectedSkills
	r.SkillBundles = snapshot.SkillBundles
	r.AvailableTools = snapshot.AvailableTools
	r.ActiveToolOutputs = snapshot.ActiveToolOutputs
	r.ManifestEntries = snapshot.ManifestEntries
	r.ProfileManifestSHA = snapshot.ProfileManifestSHA
	r.SoulSnapshotSHA = snapshot.SoulSnapshotSHA
	r.MemorySnapshotSHA = snapshot.MemorySnapshotSHA
	r.SkillSnapshotSHA = snapshot.SkillSnapshotSHA
	r.ToolSnapshotSHA = snapshot.ToolSnapshotSHA
	r.SoulStatus = snapshot.SoulStatus
	r.MemoryStatus = snapshot.MemoryStatus
	r.SkillStatus = snapshot.SkillStatus
	r.ToolStatus = snapshot.ToolStatus
	r.ProfileExportSupported = snapshot.ProfileExportSupported
	r.ProfileImportSupported = snapshot.ProfileImportSupported
	r.ProfileSwitchingSupported = snapshot.ProfileSwitchingSupported
	r.ProfileMutationAllowed = snapshot.ProfileMutationAllowed
	r.CredentialsIncluded = snapshot.CredentialsIncluded
	r.SessionsIncluded = snapshot.SessionsIncluded
	r.BackupPayloadsIncluded = snapshot.BackupPayloadsIncluded
}

func (r *ChannelProfileStatusResult) applySnapshot(snapshot channelProfileStatusSnapshot) {
	r.ProfileStatus = snapshot.ProfileStatus
	r.ProfileStrategy = snapshot.ProfileStrategy
	r.ProfileStore = snapshot.ProfileStore
	r.ProfileScope = snapshot.ProfileScope
	r.SnapshotVersion = snapshot.SnapshotVersion
	r.SnapshotScope = snapshot.SnapshotScope
	r.SnapshotSHA = snapshot.SnapshotSHA
	r.SnapshotEntries = snapshot.SnapshotEntries
	r.ProfileDocumentsLoaded = snapshot.ProfileDocumentsLoaded
	r.RequiredProfileDocuments = snapshot.RequiredProfileDocuments
	r.RequiredProfileDocumentsPresent = snapshot.RequiredProfileDocumentsPresent
	r.RequiredProfileDocumentsMissing = snapshot.RequiredProfileDocumentsMissing
	r.AvailableSkills = snapshot.AvailableSkills
	r.SelectedSkills = snapshot.SelectedSkills
	r.SkillBundles = snapshot.SkillBundles
	r.AvailableTools = snapshot.AvailableTools
	r.ActiveToolOutputs = snapshot.ActiveToolOutputs
	r.ManifestEntries = snapshot.ManifestEntries
	r.ProfileManifestSHA = snapshot.ProfileManifestSHA
	r.SoulSnapshotSHA = snapshot.SoulSnapshotSHA
	r.MemorySnapshotSHA = snapshot.MemorySnapshotSHA
	r.SkillSnapshotSHA = snapshot.SkillSnapshotSHA
	r.ToolSnapshotSHA = snapshot.ToolSnapshotSHA
	r.SoulStatus = snapshot.SoulStatus
	r.MemoryStatus = snapshot.MemoryStatus
	r.SkillStatus = snapshot.SkillStatus
	r.ToolStatus = snapshot.ToolStatus
	r.ProfileExportSupported = snapshot.ProfileExportSupported
	r.ProfileImportSupported = snapshot.ProfileImportSupported
	r.ProfileSwitchingSupported = snapshot.ProfileSwitchingSupported
	r.ProfileMutationAllowed = snapshot.ProfileMutationAllowed
	r.CredentialsIncluded = snapshot.CredentialsIncluded
	r.SessionsIncluded = snapshot.SessionsIncluded
	r.BackupPayloadsIncluded = snapshot.BackupPayloadsIncluded
}
