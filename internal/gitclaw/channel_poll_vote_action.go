package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelPollVoteOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	PollID            string
	VoteID            string
	SourceMessageID   string
	NotifyMessageID   string
	Choice            string
	ChoiceIndex       int
	Voter             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelPollVoteResult struct {
	PollIssueNumber  int
	PollIssueURL     string
	VoteCommentID    int64
	VoteDuplicate    bool
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	VoteIDHash       string
	ChoiceHash       string
	VoterHash        string
	NoteHash         string
	SourceMsgHash    string
	NotifyHash       string
	NotificationSHA  string
	ResolvedChoice   bool
	ResolvedChoiceIx int
}

type ChannelPollVoteActionRequest struct {
	Options               ChannelPollVoteOptions
	Command               string
	Subcommand            string
	AutoVoteID            bool
	AutoNotifyMessageID   bool
	TargetFromIssue       bool
	ChoiceSHA             string
	ChoiceBytes           int
	ChoiceLines           int
	VoterSHA              string
	VoterBytes            int
	VoterLines            int
	NoteSHA               string
	NoteBytes             int
	NoteLines             int
	RequestedRouteHash    string
	RequestedThreadHash   string
	RequestedMsgHash      string
	NotifyMessageHash     string
	NotificationBodySHA   string
	PollVoteBodySHA       string
	PollVoteBodyBytes     int
	PollVoteBodyLineCount int
}

func IsChannelPollVoteActionRequest(ev Event, cfg Config) bool {
	return isChannelPollVoteActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelPollVoteActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "poll-vote", "vote-response", "cast-vote", "poll-answer", "poll-response":
		return true
	default:
		return false
	}
}

func BuildChannelPollVoteActionRequest(ev Event, cfg Config) (ChannelPollVoteActionRequest, error) {
	fields, trailing, ok := channelPollVoteActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelPollVoteActionRequest{}, fmt.Errorf("missing channel poll vote command")
	}
	req := ChannelPollVoteActionRequest{
		Options: ChannelPollVoteOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
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
				return ChannelPollVoteActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--poll-id", "--id":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.PollID = cleanChannelPollID(fields[i+1])
			i++
		case "--vote-id", "--response-id", "--ballot-id":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.VoteID = cleanChannelPollVoteID(fields[i+1])
			i++
		case "--message-id", "--source-message-id":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			if req.Options.VoteID == "" {
				req.Options.VoteID = cleanChannelPollVoteID(fields[i+1])
			}
			i++
		case "--notify-message-id", "--ack-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--choice", "--option", "--answer", "--vote":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Choice = cleanChannelPollVoteChoice(fields[i+1])
			i++
		case "--voter", "--name":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Voter = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelPollVoteActionRequest{}, fmt.Errorf("unknown channel poll vote argument %q", field)
			}
			if req.Options.PollID == "" {
				req.Options.PollID = cleanChannelPollID(field)
				continue
			}
			if req.Options.Choice == "" {
				req.Options.Choice = cleanChannelPollVoteChoice(field)
				continue
			}
			return ChannelPollVoteActionRequest{}, fmt.Errorf("unexpected channel poll vote argument %q", field)
		}
	}
	choice, voter, note := parseChannelPollVoteText(trailing)
	if req.Options.Choice == "" {
		req.Options.Choice = choice
	}
	if req.Options.Voter == "" {
		req.Options.Voter = voter
	}
	req.Options.Note = note
	if err := applyChannelPollVoteIssueTarget(ev, &req); err != nil {
		return ChannelPollVoteActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.VoteID) == "" {
		req.Options.VoteID = autoChannelPollVoteID(ev, req.Options.PollID, req.Options.Channel, req.Options.ThreadID, req.Options.Choice, req.Options.Voter, req.Options.Note)
		req.AutoVoteID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelPollVoteNotifyMessageID(req.Options.PollID, req.Options.VoteID, req.Options.Channel, req.Options.ThreadID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelPollVoteOptions(req.Options)
	if err := validateChannelPollVoteActionRequestOptions(req.Options); err != nil {
		return ChannelPollVoteActionRequest{}, err
	}
	voteBody := RenderChannelPollVoteComment(req.Options)
	notificationBody := renderChannelPollVoteNotificationBody(req.Options, 0, issueURL(ev.Repo, 0))
	req.ChoiceSHA = shortDocumentHash(req.Options.Choice)
	req.ChoiceBytes = len(req.Options.Choice)
	req.ChoiceLines = lineCount(req.Options.Choice)
	req.VoterSHA = shortDocumentHash(req.Options.Voter)
	req.VoterBytes = len(req.Options.Voter)
	req.VoterLines = lineCount(req.Options.Voter)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.PollVoteBodySHA = shortDocumentHash(voteBody)
	req.PollVoteBodyBytes = len(voteBody)
	req.PollVoteBodyLineCount = lineCount(voteBody)
	return req, nil
}

func RunChannelPollVote(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPollVoteOptions) (ChannelPollVoteResult, error) {
	opts = normalizeChannelPollVoteOptions(opts)
	var err error
	opts, err = applyChannelPollVoteRoute(cfg, opts)
	if err != nil {
		return ChannelPollVoteResult{}, err
	}
	if err := validateChannelPollVoteOptions(opts); err != nil {
		return ChannelPollVoteResult{}, err
	}
	poll, err := findChannelPollIssue(ctx, cfg, github, opts.Repo, opts.PollID)
	if err != nil {
		return ChannelPollVoteResult{}, err
	}
	choices := channelPollChoicesFromIssueBody(poll.Body)
	resolvedChoice, resolvedIndex, resolved, err := resolveChannelPollVoteChoice(opts.Choice, choices)
	if err != nil {
		return ChannelPollVoteResult{}, err
	}
	opts.Choice = resolvedChoice
	opts.ChoiceIndex = resolvedIndex
	voteBody := RenderChannelPollVoteComment(opts)
	voteDuplicate := false
	var voteCommentID int64
	comments, err := github.ListIssueComments(ctx, opts.Repo, poll.Number)
	if err != nil {
		return ChannelPollVoteResult{}, fmt.Errorf("list channel poll vote comments: %w", err)
	}
	for _, comment := range comments {
		if channelPollVoteMatches(comment.Body, opts.PollID, opts.VoteID) {
			voteDuplicate = true
			voteCommentID = comment.ID
			break
		}
	}
	if !voteDuplicate {
		posted, err := github.PostIssueComment(ctx, opts.Repo, poll.Number, voteBody)
		if err != nil {
			return ChannelPollVoteResult{}, fmt.Errorf("post channel poll vote: %w", err)
		}
		voteCommentID = posted.ID
	}
	notificationBody := renderChannelPollVoteNotificationBody(opts, poll.Number, issueURL(opts.Repo, poll.Number))
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelPollVoteResult{}, fmt.Errorf("queue channel poll vote notification: %w", err)
	}
	return ChannelPollVoteResult{
		PollIssueNumber:  poll.Number,
		PollIssueURL:     issueURL(opts.Repo, poll.Number),
		VoteCommentID:    voteCommentID,
		VoteDuplicate:    voteDuplicate,
		Notification:     notification,
		RouteName:        notification.RouteName,
		RouteHash:        notification.RouteHash,
		Channel:          notification.Channel,
		ThreadHash:       notification.ThreadHash,
		VoteIDHash:       shortDocumentHash(opts.VoteID),
		ChoiceHash:       shortDocumentHash(opts.Choice),
		VoterHash:        shortDocumentHash(opts.Voter),
		NoteHash:         shortDocumentHash(opts.Note),
		SourceMsgHash:    shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
		NotificationSHA:  shortDocumentHash(notificationBody),
		ResolvedChoice:   resolved,
		ResolvedChoiceIx: resolvedIndex,
	}, nil
}

func RenderChannelPollVoteActionReport(ev Event, req ChannelPollVoteActionRequest, result ChannelPollVoteResult) string {
	status := "recorded"
	switch {
	case result.VoteDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.VoteDuplicate:
		status = "existing"
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
	sourceMessageHash := result.SourceMsgHash
	if sourceMessageHash == "" {
		sourceMessageHash = req.RequestedMsgHash
	}
	notifyHash := result.NotifyHash
	if notifyHash == "" {
		notifyHash = req.NotifyMessageHash
	}
	choiceHash := result.ChoiceHash
	if choiceHash == "" {
		choiceHash = req.ChoiceSHA
	}
	voterHash := result.VoterHash
	if voterHash == "" {
		voterHash = req.VoterSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	notificationBodyHash := result.NotificationSHA
	if notificationBodyHash == "" {
		notificationBodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Poll Vote Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_poll_vote_status: `%s`\n", status)
	fmt.Fprintf(&b, "- poll_issue: `#%d`\n", result.PollIssueNumber)
	fmt.Fprintf(&b, "- poll_issue_url: `%s`\n", result.PollIssueURL)
	fmt.Fprintf(&b, "- vote_comment_id: `%d`\n", result.VoteCommentID)
	fmt.Fprintf(&b, "- vote_recorded: `%t`\n", !result.VoteDuplicate)
	fmt.Fprintf(&b, "- vote_duplicate_suppressed: `%t`\n", result.VoteDuplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- poll_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.PollID))
	fmt.Fprintf(&b, "- vote_id_sha256_12: `%s`\n", noneIfEmpty(result.VoteIDHash))
	fmt.Fprintf(&b, "- vote_id_auto: `%t`\n", req.AutoVoteID)
	fmt.Fprintf(&b, "- choice_sha256_12: `%s`\n", noneIfEmpty(choiceHash))
	fmt.Fprintf(&b, "- choice_bytes: `%d`\n", req.ChoiceBytes)
	fmt.Fprintf(&b, "- choice_lines: `%d`\n", req.ChoiceLines)
	fmt.Fprintf(&b, "- choice_resolved_from_poll_options: `%t`\n", result.ResolvedChoice)
	fmt.Fprintf(&b, "- choice_index: `%d`\n", result.ResolvedChoiceIx)
	fmt.Fprintf(&b, "- voter_sha256_12: `%s`\n", noneIfEmpty(voterHash))
	fmt.Fprintf(&b, "- voter_bytes: `%d`\n", req.VoterBytes)
	fmt.Fprintf(&b, "- voter_lines: `%d`\n", req.VoterLines)
	fmt.Fprintf(&b, "- note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(sourceMessageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- poll_vote_body_sha256_12: `%s`\n", req.PollVoteBodySHA)
	fmt.Fprintf(&b, "- poll_vote_body_bytes: `%d`\n", req.PollVoteBodyBytes)
	fmt.Fprintf(&b, "- poll_vote_body_lines: `%d`\n", req.PollVoteBodyLineCount)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_poll_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_vote_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_choice_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_voter_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_poll_vote_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a channel-origin poll vote on the GitHub poll issue, then queued a provider-facing acknowledgement back to the mirrored channel thread. The poll issue contains the human-readable vote; this source receipt keeps poll IDs, vote IDs, choice text, participant text, notes, thread IDs, message IDs, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the poll vote acknowledgement with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent acknowledgements with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate poll vote records are suppressed by `poll_id + vote_id`; duplicate acknowledgements are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the poll issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelPollVoteComment(opts ChannelPollVoteOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-poll-vote poll_id=\"%s\" vote_id=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" channel=\"%s\" choice_sha256_12=\"%s\" choice_index=\"%d\" voter_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" note_sha256_12=\"%s\" -->\n", escapeMarkerValue(opts.PollID), escapeMarkerValue(opts.VoteID), opts.SourceIssueNumber, opts.SourceCommentID, escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Choice), opts.ChoiceIndex, shortDocumentHash(opts.Voter), shortDocumentHash(opts.SourceMessageID), shortDocumentHash(opts.Note))
	b.WriteString("GitClaw channel poll vote.\n\n")
	fmt.Fprintf(&b, "- choice: %s\n", opts.Choice)
	if opts.ChoiceIndex > 0 {
		fmt.Fprintf(&b, "- choice_index: %d\n", opts.ChoiceIndex)
	}
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- vote_id_sha256_12: %s\n", shortDocumentHash(opts.VoteID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n\n")
	if strings.TrimSpace(opts.Voter) != "" {
		b.WriteString("## Participant\n\n")
		b.WriteString(strings.TrimSpace(opts.Voter))
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(opts.Note) != "" {
		b.WriteString("## Note\n\n")
		b.WriteString(strings.TrimSpace(opts.Note))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func channelPollVoteActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelPollVoteActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelPollVoteIssueTarget(ev Event, req *ChannelPollVoteActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel poll vote requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelPollVoteText(trailing string) (string, string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	var choice string
	var voter string
	var note []string
	inNote := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inNote && len(note) > 0 {
				note = append(note, "")
			}
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "choice:"):
			inNote = false
			choice = cleanChannelPollVoteChoice(trimmed[len("choice:"):])
		case strings.HasPrefix(lower, "option:"):
			inNote = false
			choice = cleanChannelPollVoteChoice(trimmed[len("option:"):])
		case strings.HasPrefix(lower, "answer:"):
			inNote = false
			choice = cleanChannelPollVoteChoice(trimmed[len("answer:"):])
		case strings.HasPrefix(lower, "vote:"):
			inNote = false
			choice = cleanChannelPollVoteChoice(trimmed[len("vote:"):])
		case strings.HasPrefix(lower, "voter:"):
			inNote = false
			voter = strings.TrimSpace(trimmed[len("voter:"):])
		case strings.HasPrefix(lower, "participant:"):
			inNote = false
			voter = strings.TrimSpace(trimmed[len("participant:"):])
		case strings.HasPrefix(lower, "name:"):
			inNote = false
			voter = strings.TrimSpace(trimmed[len("name:"):])
		case strings.HasPrefix(lower, "note:"):
			inNote = true
			value := strings.TrimSpace(trimmed[len("note:"):])
			if value != "" {
				note = append(note, value)
			}
		case strings.HasPrefix(lower, "notes:"):
			inNote = true
			value := strings.TrimSpace(trimmed[len("notes:"):])
			if value != "" {
				note = append(note, value)
			}
		case inNote:
			note = append(note, line)
		case choice == "":
			choice = cleanChannelPollVoteChoice(trimmed)
		default:
			note = append(note, line)
		}
	}
	return strings.TrimSpace(choice), strings.TrimSpace(voter), strings.TrimSpace(strings.Join(note, "\n"))
}

func normalizeChannelPollVoteOptions(opts ChannelPollVoteOptions) ChannelPollVoteOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.PollID = cleanChannelPollID(opts.PollID)
	opts.VoteID = cleanChannelPollVoteID(opts.VoteID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.Choice = cleanChannelPollVoteChoice(opts.Choice)
	opts.Voter = strings.TrimSpace(opts.Voter)
	opts.Note = strings.TrimSpace(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelPollVoteRoute(cfg Config, opts ChannelPollVoteOptions) (ChannelPollVoteOptions, error) {
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
		Body:      opts.Choice,
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

func validateChannelPollVoteActionRequestOptions(opts ChannelPollVoteOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Route == "" && (opts.Channel == "" || opts.ThreadID == "") {
		return fmt.Errorf("missing channel route or channel thread target")
	}
	if opts.PollID == "" {
		return fmt.Errorf("missing poll id")
	}
	if opts.VoteID == "" {
		return fmt.Errorf("missing poll vote id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing poll vote source issue")
	}
	if opts.Choice == "" {
		return fmt.Errorf("missing poll vote choice")
	}
	return nil
}

func validateChannelPollVoteOptions(opts ChannelPollVoteOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.PollID == "" {
		return fmt.Errorf("missing poll id")
	}
	if opts.VoteID == "" {
		return fmt.Errorf("missing poll vote id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing poll vote source issue")
	}
	if opts.Choice == "" {
		return fmt.Errorf("missing poll vote choice")
	}
	return nil
}

func findChannelPollIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, repo, pollID string) (Issue, error) {
	issues, err := github.ListOpenIssues(ctx, repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, fmt.Errorf("list channel poll issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelPollMatches(issue.Body, pollID) {
			return issue, nil
		}
	}
	return Issue{}, fmt.Errorf("channel poll %q not found", cleanChannelPollID(pollID))
}

func channelPollVoteMatches(body, pollID, voteID string) bool {
	return HasChannelPollVoteMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`poll_id="%s"`, escapeMarkerValue(cleanChannelPollID(pollID)))) &&
		strings.Contains(body, fmt.Sprintf(`vote_id="%s"`, escapeMarkerValue(cleanChannelPollVoteID(voteID))))
}

func cleanChannelPollVoteID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelPollVoteChoice(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 160 {
		value = strings.TrimSpace(value[:160])
	}
	return value
}

func channelPollChoicesFromIssueBody(body string) []string {
	lines := strings.Split(body, "\n")
	inOptions := false
	var choices []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, "## Options") {
			inOptions = true
			continue
		}
		if inOptions && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if !inOptions || trimmed == "" {
			continue
		}
		dot := strings.Index(trimmed, ".")
		if dot <= 0 {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(trimmed[:dot])); err != nil {
			continue
		}
		choice := cleanChannelPollChoice(trimmed[dot+1:])
		if choice != "" {
			choices = append(choices, choice)
		}
	}
	return normalizeChannelPollChoices(choices)
}

func resolveChannelPollVoteChoice(value string, choices []string) (string, int, bool, error) {
	value = cleanChannelPollVoteChoice(value)
	if value == "" {
		return "", 0, false, fmt.Errorf("missing poll vote choice")
	}
	if len(choices) == 0 {
		return value, 0, false, nil
	}
	if index, err := strconv.Atoi(value); err == nil {
		if index < 1 || index > len(choices) {
			return "", 0, false, fmt.Errorf("poll vote option %d is outside 1-%d", index, len(choices))
		}
		return choices[index-1], index, true, nil
	}
	cleanValue := strings.ToLower(cleanChannelPollChoice(value))
	for i, choice := range choices {
		if strings.ToLower(cleanChannelPollChoice(choice)) == cleanValue {
			return choice, i + 1, true, nil
		}
	}
	return "", 0, false, fmt.Errorf("poll vote choice does not match a poll option")
}

func autoChannelPollVoteID(ev Event, pollID, channel, threadID, choice, voter, note string) string {
	seed := strings.Join([]string{eventID(ev), pollID, channel, threadID, choice, voter, note}, "|")
	return fmt.Sprintf("poll-vote-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelPollVoteNotifyMessageID(pollID, voteID, channel, threadID string) string {
	seed := strings.Join([]string{pollID, voteID, channel, threadID}, "|")
	return fmt.Sprintf("gitclaw-poll-vote-%s", shortDocumentHash(seed))
}

func renderChannelPollVoteNotificationBody(opts ChannelPollVoteOptions, pollIssueNumber int, pollIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw poll vote recorded.\n\n")
	if pollIssueNumber > 0 {
		fmt.Fprintf(&b, "Poll: #%d\n", pollIssueNumber)
	}
	if pollIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", pollIssueURL)
	}
	fmt.Fprintf(&b, "Choice: %s\n", opts.Choice)
	if opts.ChoiceIndex > 0 {
		fmt.Fprintf(&b, "Choice index: %d\n", opts.ChoiceIndex)
	}
	if strings.TrimSpace(opts.Voter) != "" {
		fmt.Fprintf(&b, "Participant: %s\n", strings.TrimSpace(opts.Voter))
	}
	b.WriteString("\nRecorded in the linked GitHub poll issue.")
	return strings.TrimSpace(b.String())
}
