package gitclaw

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
)

type ChannelChooseOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ChooseID          string
	Choices           []string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelChooseResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	ChooseIDHash string
	BodyHash     string
	Outcome      ChannelChooseOutcome
}

type ChannelChooseActionRequest struct {
	Options             ChannelChooseOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoChooseID        bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	ChooseIDHash        string
	ChoicesSHA          string
	ChoicesBytes        int
	NotificationBodySHA string
	Outcome             ChannelChooseOutcome
}

type ChannelChooseOutcome struct {
	Choices       []string
	ChoiceIndex   int
	Choice        string
	ChoiceSHA     string
	ChoicesSHA    string
	ChoicesBytes  int
	SeedSHA       string
	SelectionSHA  string
	SelectionMode string
}

func IsChannelChooseActionRequest(ev Event, cfg Config) bool {
	return isChannelChooseActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelChooseActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "choose", "choice", "pick", "select", "decide", "picker", "random-choice":
		return true
	default:
		return false
	}
}

func BuildChannelChooseActionRequest(ev Event, cfg Config) (ChannelChooseActionRequest, error) {
	fields, trailingBody, ok := channelChooseActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelChooseActionRequest{}, fmt.Errorf("missing channel choose command")
	}
	req := ChannelChooseActionRequest{
		Options: ChannelChooseOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--choose-id", "--choice-id", "--pick-id", "--id":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ChooseID = cleanChannelChooseID(fields[i+1])
			i++
		case "--option", "--choice":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Choices = append(req.Options.Choices, fields[i+1])
			i++
		case "--options", "--choices":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Choices = append(req.Options.Choices, splitChannelChooseInlineChoices(fields[i+1])...)
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelChooseActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelChooseActionRequest{}, fmt.Errorf("unknown channel choose argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelChooseActionRequest{}, fmt.Errorf("unexpected channel choose argument %q", field)
		}
	}
	req.Options.Choices = append(req.Options.Choices, parseChannelChooseTrailingChoices(trailingBody)...)
	if err := applyChannelChooseIssueTarget(ev, &req); err != nil {
		return ChannelChooseActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelChooseSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.ChooseID) == "" {
		req.Options.ChooseID = autoChannelChooseID(ev, req.Options)
		req.AutoChooseID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelChooseNotifyMessageID(ev, req.Options.ChooseID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelChooseOptions(req.Options)
	if err := validateChannelChooseActionRequestOptions(req.Options); err != nil {
		return ChannelChooseActionRequest{}, err
	}
	outcome, err := BuildChannelChooseOutcome(req.Options)
	if err != nil {
		return ChannelChooseActionRequest{}, err
	}
	body := renderChannelChooseNotificationBody(outcome)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ChooseIDHash = shortDocumentHash(req.Options.ChooseID)
	req.ChoicesSHA = outcome.ChoicesSHA
	req.ChoicesBytes = outcome.ChoicesBytes
	req.NotificationBodySHA = shortDocumentHash(body)
	req.Outcome = outcome
	return req, nil
}

func RunChannelChoose(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelChooseOptions) (ChannelChooseResult, error) {
	opts = normalizeChannelChooseOptions(opts)
	var err error
	opts, err = applyChannelChooseRoute(cfg, opts)
	if err != nil {
		return ChannelChooseResult{}, err
	}
	if err := validateChannelChooseOptions(opts); err != nil {
		return ChannelChooseResult{}, err
	}
	outcome, err := BuildChannelChooseOutcome(opts)
	if err != nil {
		return ChannelChooseResult{}, err
	}
	body := renderChannelChooseNotificationBody(outcome)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelChooseResult{}, fmt.Errorf("queue channel choice notification: %w", err)
	}
	return ChannelChooseResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		ChooseIDHash: shortDocumentHash(opts.ChooseID),
		BodyHash:     shortDocumentHash(body),
		Outcome:      outcome,
	}, nil
}

func RenderChannelChooseActionReport(ev Event, req ChannelChooseActionRequest, result ChannelChooseResult) string {
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
	chooseIDHash := result.ChooseIDHash
	if chooseIDHash == "" {
		chooseIDHash = req.ChooseIDHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	outcome := result.Outcome
	if outcome.SelectionMode == "" {
		outcome = req.Outcome
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Choose Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_choose_status: `%s`\n", status)
	fmt.Fprintf(&b, "- choose_mode: `%s`\n", "deterministic-channel-option-picker")
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
	fmt.Fprintf(&b, "- choose_id_sha256_12: `%s`\n", noneIfEmpty(chooseIDHash))
	fmt.Fprintf(&b, "- choose_id_auto: `%t`\n", req.AutoChooseID)
	fmt.Fprintf(&b, "- choices_count: `%d`\n", len(outcome.Choices))
	fmt.Fprintf(&b, "- choices_sha256_12: `%s`\n", noneIfEmpty(outcome.ChoicesSHA))
	fmt.Fprintf(&b, "- choices_bytes: `%d`\n", outcome.ChoicesBytes)
	fmt.Fprintf(&b, "- selected_choice_index: `%d`\n", outcome.ChoiceIndex)
	fmt.Fprintf(&b, "- selected_choice_sha256_12: `%s`\n", noneIfEmpty(outcome.ChoiceSHA))
	fmt.Fprintf(&b, "- selection_sha256_12: `%s`\n", noneIfEmpty(outcome.SelectionSHA))
	fmt.Fprintf(&b, "- choice_seed_sha256_12: `%s`\n", noneIfEmpty(outcome.SeedSHA))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- deterministic_picker_used: `%t`\n", true)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- cryptographic_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_choose_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_choices_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_selected_choice_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_choose_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing deterministic option pick on the canonical channel issue. This is a small channel-native interaction: provider users get one selected choice, while the source receipt keeps thread ids, message ids, choose ids, option text, selected choice text, and channel bodies out of band. The action does not call a model, use external randomness, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read choice notifications with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent choice notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate choice notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelChooseActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelChooseActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelChooseIssueTarget(ev Event, req *ChannelChooseActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel choose requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelChooseOptions(opts ChannelChooseOptions) ChannelChooseOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ChooseID = cleanChannelChooseID(opts.ChooseID)
	opts.Choices = normalizeChannelChooseChoices(opts.Choices)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelChooseRoute(cfg Config, opts ChannelChooseOptions) (ChannelChooseOptions, error) {
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
		Body:      "GitClaw channel choice.",
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

func validateChannelChooseOptions(opts ChannelChooseOptions) error {
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
	if opts.ChooseID == "" {
		return fmt.Errorf("missing choose id")
	}
	choices := normalizeChannelChooseChoices(opts.Choices)
	if len(choices) < 2 {
		return fmt.Errorf("channel choose requires at least 2 choices")
	}
	if len(choices) > 20 {
		return fmt.Errorf("channel choose supports at most 20 choices")
	}
	return nil
}

func validateChannelChooseActionRequestOptions(opts ChannelChooseOptions) error {
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
	if opts.ChooseID == "" {
		return fmt.Errorf("missing choose id")
	}
	choices := normalizeChannelChooseChoices(opts.Choices)
	if len(choices) < 2 {
		return fmt.Errorf("channel choose requires at least 2 choices")
	}
	if len(choices) > 20 {
		return fmt.Errorf("channel choose supports at most 20 choices")
	}
	return nil
}

func cleanChannelChooseID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelChooseSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-choose-source-%s", eventID(ev))
}

func autoChannelChooseID(ev Event, opts ChannelChooseOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, strings.Join(normalizeChannelChooseChoices(opts.Choices), "\n")}, "|")
	return fmt.Sprintf("choose-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelChooseNotifyMessageID(ev Event, chooseID string) string {
	seed := strings.Join([]string{eventID(ev), chooseID}, "|")
	return fmt.Sprintf("gitclaw-channel-choose-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func splitChannelChooseInlineChoices(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	replacer := strings.NewReplacer("|", "\n", ";", "\n", ",", "\n")
	parts := strings.Split(replacer.Replace(value), "\n")
	choices := make([]string, 0, len(parts))
	for _, part := range parts {
		choices = append(choices, part)
	}
	return normalizeChannelChooseChoices(choices)
}

func parseChannelChooseTrailingChoices(trailing string) []string {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	choices := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, " \t\r"))
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "options:") || strings.HasPrefix(lower, "choices:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				choices = append(choices, splitChannelChooseInlineChoices(trimmed[idx+1:])...)
			}
			continue
		}
		if !isChannelChooseChoiceLine(trimmed) {
			continue
		}
		choices = append(choices, trimPollOptionBullet(trimmed))
	}
	return normalizeChannelChooseChoices(choices)
}

func isChannelChooseChoiceLine(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "- ") || strings.HasPrefix(value, "* ") {
		return true
	}
	for i, r := range value {
		if r < '0' || r > '9' {
			return i > 0 && (r == '.' || r == ')')
		}
	}
	return false
}

func normalizeChannelChooseChoices(choices []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(choices))
	for _, choice := range choices {
		choice = cleanChannelPollChoice(choice)
		if choice == "" {
			continue
		}
		key := strings.ToLower(choice)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, choice)
	}
	return normalized
}

func BuildChannelChooseOutcome(opts ChannelChooseOptions) (ChannelChooseOutcome, error) {
	choices := normalizeChannelChooseChoices(opts.Choices)
	if len(choices) < 2 {
		return ChannelChooseOutcome{}, fmt.Errorf("channel choose requires at least 2 choices")
	}
	if len(choices) > 20 {
		return ChannelChooseOutcome{}, fmt.Errorf("channel choose supports at most 20 choices")
	}
	choicesText := strings.Join(choices, "\n")
	choicesSHA := shortDocumentHash(choicesText)
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.ChooseID, choicesSHA}, "|")
	choiceIndex := deterministicChannelChooseIndex(seed, len(choices))
	choice := choices[choiceIndex]
	selectionText := fmt.Sprintf("%d|%s", choiceIndex+1, choice)
	return ChannelChooseOutcome{
		Choices:       choices,
		ChoiceIndex:   choiceIndex + 1,
		Choice:        choice,
		ChoiceSHA:     shortDocumentHash(choice),
		ChoicesSHA:    choicesSHA,
		ChoicesBytes:  len(choicesText),
		SeedSHA:       shortDocumentHash(seed),
		SelectionSHA:  shortDocumentHash(selectionText),
		SelectionMode: "deterministic-channel-option-picker",
	}, nil
}

func deterministicChannelChooseIndex(seed string, choices int) int {
	if choices <= 0 {
		return 0
	}
	sum := sha256.Sum256([]byte(seed))
	value := binary.BigEndian.Uint64(sum[:8])
	return int(value % uint64(choices))
}

func renderChannelChooseNotificationBody(outcome ChannelChooseOutcome) string {
	var b strings.Builder
	b.WriteString("GitClaw channel choice.\n\n")
	fmt.Fprintf(&b, "Choices: %d\n", len(outcome.Choices))
	fmt.Fprintf(&b, "Picked: #%d\n", outcome.ChoiceIndex)
	fmt.Fprintf(&b, "Choice: %s\n", outcome.Choice)
	fmt.Fprintf(&b, "Choice hash: %s\n", outcome.ChoiceSHA)
	fmt.Fprintf(&b, "Seed hash: %s\n", outcome.SeedSHA)
	b.WriteString("\nSelection source: deterministic GitHub channel action seed.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("External randomness: not used.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}
