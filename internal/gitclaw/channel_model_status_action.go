package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelModelStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelModelStatusResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	StatusIDHash string
	BodyHash     string
	Provider     string
	Model        string
	EndpointHost string
}

type ChannelModelStatusActionRequest struct {
	Options             ChannelModelStatusOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoStatusID        bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	StatusIDHash        string
	ProviderHash        string
	ModelHash           string
	EndpointHostHash    string
	NotificationBodySHA string
}

func IsChannelModelStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelModelStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelModelStatusActionFields(fields)
}

func isChannelModelStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "model", "models", "model-status", "runtime-model", "which-model", "llm", "llm-status":
		return true
	default:
		return false
	}
}

func BuildChannelModelStatusActionRequest(ev Event, cfg Config) (ChannelModelStatusActionRequest, error) {
	fields, _, ok := channelModelStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelModelStatusActionRequest{}, fmt.Errorf("missing channel model status command")
	}
	req := ChannelModelStatusActionRequest{
		Options: ChannelModelStatusOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelModelStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelModelStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelModelStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelModelStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelModelStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--model-status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelModelStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelModelStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelModelStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelModelStatusActionRequest{}, fmt.Errorf("unknown channel model status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelModelStatusActionRequest{}, fmt.Errorf("unexpected channel model status argument %q", field)
		}
	}
	if err := applyChannelModelStatusIssueTarget(ev, &req); err != nil {
		return ChannelModelStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelModelStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelModelStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelModelStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelModelStatusOptions(req.Options)
	if err := validateChannelModelStatusActionRequestOptions(req.Options); err != nil {
		return ChannelModelStatusActionRequest{}, err
	}
	provider := channelModelStatusProvider(cfg)
	model := strings.TrimSpace(cfg.Model)
	endpointHost := llmEndpointHost(llmBaseURL(cfg))
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.ProviderHash = shortDocumentHash(provider)
	req.ModelHash = shortDocumentHash(model)
	req.EndpointHostHash = shortDocumentHash(endpointHost)
	req.NotificationBodySHA = shortDocumentHash(renderChannelModelStatusNotificationBody(req.Options, cfg))
	return req, nil
}

func RunChannelModelStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelModelStatusOptions) (ChannelModelStatusResult, error) {
	opts = normalizeChannelModelStatusOptions(opts)
	var err error
	opts, err = applyChannelModelStatusRoute(cfg, opts)
	if err != nil {
		return ChannelModelStatusResult{}, err
	}
	if err := validateChannelModelStatusOptions(opts); err != nil {
		return ChannelModelStatusResult{}, err
	}
	body := renderChannelModelStatusNotificationBody(opts, cfg)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelModelStatusResult{}, fmt.Errorf("queue channel model status notification: %w", err)
	}
	return ChannelModelStatusResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		StatusIDHash: shortDocumentHash(opts.StatusID),
		BodyHash:     shortDocumentHash(body),
		Provider:     channelModelStatusProvider(cfg),
		Model:        strings.TrimSpace(cfg.Model),
		EndpointHost: llmEndpointHost(llmBaseURL(cfg)),
	}, nil
}

func RenderChannelModelStatusActionReport(ev Event, cfg Config, req ChannelModelStatusActionRequest, result ChannelModelStatusResult) string {
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
	provider := result.Provider
	if provider == "" {
		provider = channelModelStatusProvider(cfg)
	}
	model := result.Model
	if model == "" {
		model = strings.TrimSpace(cfg.Model)
	}
	endpointHost := result.EndpointHost
	if endpointHost == "" {
		endpointHost = llmEndpointHost(llmBaseURL(cfg))
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Model Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_model_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- model_snapshot_mode: `%s`\n", "provider-facing-runtime-status")
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
	fmt.Fprintf(&b, "- model_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- model_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- model_provider: `%s`\n", provider)
	fmt.Fprintf(&b, "- model_provider_sha256_12: `%s`\n", req.ProviderHash)
	fmt.Fprintf(&b, "- model: `%s`\n", model)
	fmt.Fprintf(&b, "- model_sha256_12: `%s`\n", req.ModelHash)
	fmt.Fprintf(&b, "- fallback_models: `%s`\n", inlineListOrNone(cfg.ModelFallbacks))
	fmt.Fprintf(&b, "- fallback_model_count: `%d`\n", len(cfg.ModelFallbacks))
	fmt.Fprintf(&b, "- endpoint_host: `%s`\n", endpointHost)
	fmt.Fprintf(&b, "- endpoint_host_sha256_12: `%s`\n", req.EndpointHostHash)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- default_model_policy: `%s`\n", "smallest-openai-github-models-catalog-model")
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_switch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_config_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- fallback_config_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_model_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_model_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing model runtime snapshot on the canonical channel issue. This is the GitHub-native channel version of a runtime model-status command: it reports the reviewed provider, model, fallback count, endpoint host, and read-only action mode, but it does not call a model, switch models, edit configuration, mutate the repository, or call provider APIs. The source receipt keeps thread ids, message ids, status ids, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the model-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent model-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate model-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal follow-up comments on the same GitHub issue still take the GitHub Models path and must expose model, tool, skill, prompt-context, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func channelModelStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelModelStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelModelStatusIssueTarget(ev Event, req *ChannelModelStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel model status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelModelStatusOptions(opts ChannelModelStatusOptions) ChannelModelStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelModelStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelModelStatusRoute(cfg Config, opts ChannelModelStatusOptions) (ChannelModelStatusOptions, error) {
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
		Body:      strings.TrimSpace(cfg.Model),
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

func validateChannelModelStatusOptions(opts ChannelModelStatusOptions) error {
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
		return fmt.Errorf("missing model status id")
	}
	return nil
}

func validateChannelModelStatusActionRequestOptions(opts ChannelModelStatusOptions) error {
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
		return fmt.Errorf("missing model status id")
	}
	return nil
}

func cleanChannelModelStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelModelStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-model-source-%s", eventID(ev))
}

func autoChannelModelStatusID(ev Event, opts ChannelModelStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("model-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelModelStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-model-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelModelStatusProvider(cfg Config) string {
	return llmProviderForReport(cfg, llmBaseURL(cfg))
}

func renderChannelModelStatusNotificationBody(opts ChannelModelStatusOptions, cfg Config) string {
	baseURL := llmBaseURL(cfg)
	provider := llmProviderForReport(cfg, baseURL)
	var b strings.Builder
	b.WriteString("GitClaw channel model status.\n\n")
	fmt.Fprintf(&b, "Provider: %s\n", provider)
	fmt.Fprintf(&b, "Model: %s\n", strings.TrimSpace(cfg.Model))
	fmt.Fprintf(&b, "Fallback models: %s\n", inlineListOrNone(cfg.ModelFallbacks))
	fmt.Fprintf(&b, "Fallbacks configured: %d\n", len(cfg.ModelFallbacks))
	fmt.Fprintf(&b, "Endpoint host: %s\n", llmEndpointHost(baseURL))
	b.WriteString("Run mode: read-only\n")
	b.WriteString("Default model policy: smallest-openai-github-models-catalog-model\n")
	b.WriteString("\nModel call: not performed by this action.\n")
	b.WriteString("Model switch: not performed by this action.\n")
	b.WriteString("Configuration write: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}
