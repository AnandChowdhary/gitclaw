package gitclaw

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

type ChannelRollOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RollID            string
	Expression        string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelRollResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	RollIDHash   string
	BodyHash     string
	Outcome      ChannelRollOutcome
}

type ChannelRollActionRequest struct {
	Options             ChannelRollOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoRollID          bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	RollIDHash          string
	ExpressionSHA       string
	ExpressionBytes     int
	NotificationBodySHA string
	Outcome             ChannelRollOutcome
}

type ChannelRollSpec struct {
	Kind                 string
	NormalizedExpression string
	DiceCount            int
	DiceSides            int
	Modifier             int
}

type ChannelRollOutcome struct {
	Kind                 string
	NormalizedExpression string
	DiceCount            int
	DiceSides            int
	Modifier             int
	Values               []int
	Total                int
	Label                string
	SeedSHA              string
	ValuesSHA            string
	ResultSHA            string
}

func IsChannelRollActionRequest(ev Event, cfg Config) bool {
	return isChannelRollActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelRollActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "roll", "dice", "roll-dice", "random", "rng", "coin", "flip", "flip-coin":
		return true
	default:
		return false
	}
}

func BuildChannelRollActionRequest(ev Event, cfg Config) (ChannelRollActionRequest, error) {
	fields, _, ok := channelRollActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRollActionRequest{}, fmt.Errorf("missing channel roll command")
	}
	req := ChannelRollActionRequest{
		Options: ChannelRollOptions{
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
				return ChannelRollActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelRollActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelRollActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelRollActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelRollActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--roll-id", "--dice-id", "--coin-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRollActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RollID = cleanChannelRollID(fields[i+1])
			i++
		case "--dice", "--roll", "--expression", "--expr":
			if i+1 >= len(fields) {
				return ChannelRollActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Expression = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRollActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRollActionRequest{}, fmt.Errorf("unknown channel roll argument %q", field)
			}
			if req.Options.Expression == "" && isChannelRollExpressionCandidate(field) {
				req.Options.Expression = field
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelRollActionRequest{}, fmt.Errorf("unexpected channel roll argument %q", field)
		}
	}
	if err := applyChannelRollIssueTarget(ev, &req); err != nil {
		return ChannelRollActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.Expression) == "" {
		req.Options.Expression = defaultChannelRollExpression(req.Subcommand)
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelRollSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.RollID) == "" {
		req.Options.RollID = autoChannelRollID(ev, req.Options)
		req.AutoRollID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelRollNotifyMessageID(ev, req.Options.RollID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelRollOptions(req.Options)
	if err := validateChannelRollActionRequestOptions(req.Options); err != nil {
		return ChannelRollActionRequest{}, err
	}
	outcome, err := BuildChannelRollOutcome(req.Options)
	if err != nil {
		return ChannelRollActionRequest{}, err
	}
	body := renderChannelRollNotificationBody(outcome)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.RollIDHash = shortDocumentHash(req.Options.RollID)
	req.ExpressionSHA = shortDocumentHash(outcome.NormalizedExpression)
	req.ExpressionBytes = len(outcome.NormalizedExpression)
	req.NotificationBodySHA = shortDocumentHash(body)
	req.Outcome = outcome
	return req, nil
}

func RunChannelRoll(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRollOptions) (ChannelRollResult, error) {
	opts = normalizeChannelRollOptions(opts)
	var err error
	opts, err = applyChannelRollRoute(cfg, opts)
	if err != nil {
		return ChannelRollResult{}, err
	}
	if err := validateChannelRollOptions(opts); err != nil {
		return ChannelRollResult{}, err
	}
	outcome, err := BuildChannelRollOutcome(opts)
	if err != nil {
		return ChannelRollResult{}, err
	}
	body := renderChannelRollNotificationBody(outcome)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelRollResult{}, fmt.Errorf("queue channel roll notification: %w", err)
	}
	return ChannelRollResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		RollIDHash:   shortDocumentHash(opts.RollID),
		BodyHash:     shortDocumentHash(body),
		Outcome:      outcome,
	}, nil
}

func RenderChannelRollActionReport(ev Event, req ChannelRollActionRequest, result ChannelRollResult) string {
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
	rollIDHash := result.RollIDHash
	if rollIDHash == "" {
		rollIDHash = req.RollIDHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	outcome := result.Outcome
	if outcome.NormalizedExpression == "" {
		outcome = req.Outcome
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Roll Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_roll_status: `%s`\n", status)
	fmt.Fprintf(&b, "- roll_mode: `%s`\n", "deterministic-channel-randomizer")
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
	fmt.Fprintf(&b, "- roll_id_sha256_12: `%s`\n", noneIfEmpty(rollIDHash))
	fmt.Fprintf(&b, "- roll_id_auto: `%t`\n", req.AutoRollID)
	fmt.Fprintf(&b, "- roll_expression_sha256_12: `%s`\n", noneIfEmpty(req.ExpressionSHA))
	fmt.Fprintf(&b, "- roll_expression_bytes: `%d`\n", req.ExpressionBytes)
	fmt.Fprintf(&b, "- roll_kind: `%s`\n", outcome.Kind)
	fmt.Fprintf(&b, "- dice_count: `%d`\n", outcome.DiceCount)
	fmt.Fprintf(&b, "- dice_sides: `%d`\n", outcome.DiceSides)
	fmt.Fprintf(&b, "- dice_modifier: `%d`\n", outcome.Modifier)
	fmt.Fprintf(&b, "- roll_total: `%d`\n", outcome.Total)
	fmt.Fprintf(&b, "- roll_label_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(outcome.Label)))
	fmt.Fprintf(&b, "- roll_values_sha256_12: `%s`\n", noneIfEmpty(outcome.ValuesSHA))
	fmt.Fprintf(&b, "- roll_result_sha256_12: `%s`\n", noneIfEmpty(outcome.ResultSHA))
	fmt.Fprintf(&b, "- roll_seed_sha256_12: `%s`\n", noneIfEmpty(outcome.SeedSHA))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- deterministic_rng_used: `%t`\n", true)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- cryptographic_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_roll_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_roll_expression_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_roll_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing deterministic dice/coin result on the canonical channel issue. This is a small channel-native interaction: provider users get a useful roll result, while the source receipt keeps thread ids, message ids, roll ids, expressions, and channel bodies out of band. The action does not call a model, use external randomness, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read roll notifications with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent roll notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate roll notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelRollActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRollActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelRollIssueTarget(ev Event, req *ChannelRollActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel roll requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelRollOptions(opts ChannelRollOptions) ChannelRollOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RollID = cleanChannelRollID(opts.RollID)
	opts.Expression = strings.TrimSpace(opts.Expression)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelRollRoute(cfg Config, opts ChannelRollOptions) (ChannelRollOptions, error) {
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
		Body:      "GitClaw channel roll.",
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

func validateChannelRollOptions(opts ChannelRollOptions) error {
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
	if opts.RollID == "" {
		return fmt.Errorf("missing roll id")
	}
	_, err := ParseChannelRollSpec(opts.Expression)
	return err
}

func validateChannelRollActionRequestOptions(opts ChannelRollOptions) error {
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
	if opts.RollID == "" {
		return fmt.Errorf("missing roll id")
	}
	_, err := ParseChannelRollSpec(opts.Expression)
	return err
}

func cleanChannelRollID(value string) string {
	return cleanChannelHuddleID(value)
}

func defaultChannelRollExpression(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "coin", "flip", "flip-coin":
		return "coin"
	default:
		return "1d6"
	}
}

func autoChannelRollSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-roll-source-%s", eventID(ev))
}

func autoChannelRollID(ev Event, opts ChannelRollOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Expression}, "|")
	return fmt.Sprintf("roll-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelRollNotifyMessageID(ev Event, rollID string) string {
	seed := strings.Join([]string{eventID(ev), rollID}, "|")
	return fmt.Sprintf("gitclaw-channel-roll-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func isChannelRollExpressionCandidate(value string) bool {
	_, err := ParseChannelRollSpec(value)
	return err == nil
}

func ParseChannelRollSpec(expression string) (ChannelRollSpec, error) {
	cleaned := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(expression), " ", ""))
	switch cleaned {
	case "", "default":
		cleaned = "1d6"
	case "coin", "flip", "heads-tails", "heads_or_tails":
		return ChannelRollSpec{Kind: "coin", NormalizedExpression: "coin", DiceCount: 1, DiceSides: 2}, nil
	}
	dIndex := strings.Index(cleaned, "d")
	if dIndex < 0 {
		return ChannelRollSpec{}, fmt.Errorf("roll expression must be `coin` or dice like `2d6+1`")
	}
	countText := cleaned[:dIndex]
	if countText == "" {
		countText = "1"
	}
	diceCount, err := strconv.Atoi(countText)
	if err != nil || diceCount < 1 || diceCount > 20 {
		return ChannelRollSpec{}, fmt.Errorf("dice count must be between 1 and 20")
	}
	rest := cleaned[dIndex+1:]
	if rest == "" {
		return ChannelRollSpec{}, fmt.Errorf("dice sides are required")
	}
	modifier := 0
	modIndex := -1
	for i := 1; i < len(rest); i++ {
		if rest[i] == '+' || rest[i] == '-' {
			modIndex = i
			break
		}
	}
	sidesText := rest
	if modIndex >= 0 {
		sidesText = rest[:modIndex]
		modifierText := rest[modIndex:]
		modifier, err = strconv.Atoi(modifierText)
		if err != nil || modifier < -10000 || modifier > 10000 {
			return ChannelRollSpec{}, fmt.Errorf("dice modifier must be between -10000 and 10000")
		}
	}
	diceSides, err := strconv.Atoi(sidesText)
	if err != nil || diceSides < 2 || diceSides > 1000 {
		return ChannelRollSpec{}, fmt.Errorf("dice sides must be between 2 and 1000")
	}
	normalized := fmt.Sprintf("%dd%d", diceCount, diceSides)
	if modifier != 0 {
		normalized = fmt.Sprintf("%s%+d", normalized, modifier)
	}
	return ChannelRollSpec{
		Kind:                 "dice",
		NormalizedExpression: normalized,
		DiceCount:            diceCount,
		DiceSides:            diceSides,
		Modifier:             modifier,
	}, nil
}

func BuildChannelRollOutcome(opts ChannelRollOptions) (ChannelRollOutcome, error) {
	spec, err := ParseChannelRollSpec(opts.Expression)
	if err != nil {
		return ChannelRollOutcome{}, err
	}
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.RollID, spec.NormalizedExpression}, "|")
	values := make([]int, 0, spec.DiceCount)
	sum := 0
	for i := 0; i < spec.DiceCount; i++ {
		value := deterministicChannelRollValue(seed, i, spec.DiceSides)
		values = append(values, value)
		sum += value
	}
	total := sum + spec.Modifier
	label := ""
	resultText := strconv.Itoa(total)
	if spec.Kind == "coin" {
		if values[0] == 1 {
			label = "heads"
		} else {
			label = "tails"
		}
		total = values[0]
		resultText = label
	}
	valuesText := channelRollValuesString(values)
	return ChannelRollOutcome{
		Kind:                 spec.Kind,
		NormalizedExpression: spec.NormalizedExpression,
		DiceCount:            spec.DiceCount,
		DiceSides:            spec.DiceSides,
		Modifier:             spec.Modifier,
		Values:               values,
		Total:                total,
		Label:                label,
		SeedSHA:              shortDocumentHash(seed),
		ValuesSHA:            shortDocumentHash(valuesText),
		ResultSHA:            shortDocumentHash(resultText),
	}, nil
}

func deterministicChannelRollValue(seed string, index, sides int) int {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", seed, index)))
	value := binary.BigEndian.Uint64(sum[:8])
	return int(value%uint64(sides)) + 1
}

func renderChannelRollNotificationBody(outcome ChannelRollOutcome) string {
	var b strings.Builder
	b.WriteString("GitClaw channel roll.\n\n")
	fmt.Fprintf(&b, "Roll: %s\n", outcome.NormalizedExpression)
	if outcome.Kind == "coin" {
		fmt.Fprintf(&b, "Result: %s\n", outcome.Label)
	} else {
		fmt.Fprintf(&b, "Result: %d\n", outcome.Total)
		fmt.Fprintf(&b, "Dice: %s\n", channelRollValuesString(outcome.Values))
		if outcome.Modifier != 0 {
			fmt.Fprintf(&b, "Modifier: %+d\n", outcome.Modifier)
		}
	}
	fmt.Fprintf(&b, "Roll hash: %s\n", outcome.ResultSHA)
	fmt.Fprintf(&b, "Seed hash: %s\n", outcome.SeedSHA)
	b.WriteString("\nRandom source: deterministic GitHub channel action seed.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("External randomness: not used.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelRollValuesString(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}
