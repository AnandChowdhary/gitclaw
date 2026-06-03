package gitclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var markerPattern = regexp.MustCompile(`<!--\s*gitclaw:assistant-turn\s+([^>]*)-->`)
var heartbeatMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:heartbeat\s+([^>]*)-->`)
var errorMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:error\s+([^>]*)-->`)
var backupRestoreRequestIssueMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:backup-restore-request-issue\s+([^>]*)-->`)
var checkpointRehearsalIssueMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:checkpoint-rehearsal-issue\s+([^>]*)-->`)
var channelMessageMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-message\s+([^>]*)-->`)
var channelOutboundMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-outbound\s+([^>]*)-->`)
var channelDeliverableMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-deliverable\s+([^>]*)-->`)
var channelReactionMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-reaction\s+([^>]*)-->`)
var channelStatusMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-status\s+([^>]*)-->`)
var channelEditMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-edit\s+([^>]*)-->`)
var channelTopicMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-topic\s+([^>]*)-->`)
var channelActivityMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-activity\s+([^>]*)-->`)
var channelThreadMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-thread\s+([^>]*)-->`)
var channelRoomMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-room\s+([^>]*)-->`)
var channelHuddleMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-huddle\s+([^>]*)-->`)
var channelPollMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-poll\s+([^>]*)-->`)
var channelPollVoteMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-poll-vote\s+([^>]*)-->`)
var channelRollcallMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-rollcall\s+([^>]*)-->`)
var channelRsvpMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-rsvp\s+([^>]*)-->`)
var channelRsvpResponseMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-rsvp-response\s+([^>]*)-->`)
var channelTaskMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-task\s+([^>]*)-->`)
var channelWatchMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-watch\s+([^>]*)-->`)
var channelStandingOrderProposalMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-standing-order-proposal\s+([^>]*)-->`)
var channelClipMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-clip\s+([^>]*)-->`)
var channelOpenLoopMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-open-loop\s+([^>]*)-->`)
var channelAttachmentMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-attachment\s+([^>]*)-->`)
var channelSnippetMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-snippet\s+([^>]*)-->`)
var channelDecisionMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-decision\s+([^>]*)-->`)
var channelDigestMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-digest\s+([^>]*)-->`)
var channelIdeaMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-idea\s+([^>]*)-->`)
var channelJamMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-jam\s+([^>]*)-->`)
var channelKudosMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-kudos\s+([^>]*)-->`)
var channelRetroMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-retro\s+([^>]*)-->`)
var channelPlaybookMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-playbook\s+([^>]*)-->`)
var channelInsightMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-insight\s+([^>]*)-->`)
var channelWorkspaceProposalMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-workspace-proposal\s+([^>]*)-->`)
var channelBoardCardMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-board-card\s+([^>]*)-->`)
var channelChecklistMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-checklist\s+([^>]*)-->`)
var channelToolsetProposalMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-toolset-proposal\s+([^>]*)-->`)
var channelPromptProposalMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-prompt-proposal\s+([^>]*)-->`)
var channelBundleProposalMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-bundle-proposal\s+([^>]*)-->`)
var channelIncidentMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-incident\s+([^>]*)-->`)
var channelVoiceMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-voice\s+([^>]*)-->`)
var channelImageMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-image\s+([^>]*)-->`)
var channelLinkMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-link\s+([^>]*)-->`)
var channelBookmarkMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-bookmark\s+([^>]*)-->`)
var channelForkMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-fork\s+([^>]*)-->`)
var channelMergeMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-merge\s+([^>]*)-->`)
var channelAccessRequestMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-access-request\s+([^>]*)-->`)
var channelContactMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-contact\s+([^>]*)-->`)
var channelReminderMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-reminder\s+([^>]*)-->`)
var channelStateMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-state\s+([^>]*)-->`)
var channelStateUpdateMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-state-update\s+([^>]*)-->`)
var channelDeliveryMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-delivery\s+([^>]*)-->`)
var proactiveRunMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:proactive-run\s+([^>]*)-->`)

func IdempotencyKey(ev Event) string {
	trigger := fmt.Sprintf("issue:%d", ev.Issue.Number)
	if ev.Comment != nil {
		trigger = fmt.Sprintf("comment:%d", ev.Comment.ID)
	}
	if ev.Kind == EventWorkflowDispatch {
		if ev.DispatchID != "" {
			trigger = fmt.Sprintf("dispatch:%s", ev.DispatchID)
		} else {
			trigger = fmt.Sprintf("dispatch:issue:%d", ev.Issue.Number)
		}
	}
	sha := ev.SHA
	if ev.Kind == EventWorkflowDispatch && ev.DispatchID != "" {
		sha = ""
	}
	seed := fmt.Sprintf("%s|%s|%s|%s", ev.Repo, ev.EventName, trigger, sha)
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:16])
}

func RenderAssistantComment(marker Marker, text string) string {
	parts := []string{
		fmt.Sprintf(`run_id="%s"`, escapeMarkerValue(marker.RunID)),
		fmt.Sprintf(`event_id="%s"`, escapeMarkerValue(marker.EventID)),
		fmt.Sprintf(`model="%s"`, escapeMarkerValue(marker.Model)),
		fmt.Sprintf(`idempotency_key="%s"`, escapeMarkerValue(marker.IdempotencyKey)),
	}
	if marker.RunURL != "" {
		parts = append(parts, fmt.Sprintf(`run_url="%s"`, escapeMarkerValue(marker.RunURL)))
	}
	if marker.PromptContextSHA != "" {
		parts = append(parts,
			fmt.Sprintf(`prompt_context_sha256_12="%s"`, escapeMarkerValue(marker.PromptContextSHA)),
			fmt.Sprintf(`context_documents="%d"`, marker.ContextDocuments),
			fmt.Sprintf(`selected_skills="%d"`, marker.SelectedSkills),
			fmt.Sprintf(`tool_outputs="%d"`, marker.ToolOutputs),
		)
		if len(marker.PromptVisibleSkills) > 0 {
			parts = append(parts, fmt.Sprintf(`skills="%s"`, escapeMarkerValue(strings.Join(marker.PromptVisibleSkills, ","))))
		}
		if len(marker.PromptVisibleTools) > 0 {
			parts = append(parts, fmt.Sprintf(`tools="%s"`, escapeMarkerValue(strings.Join(marker.PromptVisibleTools, ","))))
		}
	}
	if marker.Usage.Present {
		parts = append(parts,
			fmt.Sprintf(`usage_prompt_tokens="%d"`, marker.Usage.PromptTokens),
			fmt.Sprintf(`usage_completion_tokens="%d"`, marker.Usage.CompletionTokens),
			fmt.Sprintf(`usage_total_tokens="%d"`, marker.Usage.TotalTokens),
			fmt.Sprintf(`usage_cache_read_tokens="%d"`, marker.Usage.CacheReadTokens),
			fmt.Sprintf(`usage_cache_write_tokens="%d"`, marker.Usage.CacheWriteTokens),
		)
	}
	return fmt.Sprintf("<!-- gitclaw:assistant-turn %s -->\n%s", strings.Join(parts, " "), strings.TrimSpace(text))
}

func RenderHeartbeatComment(marker HeartbeatMarker, text string) string {
	parts := []string{
		fmt.Sprintf(`run_id="%s"`, escapeMarkerValue(marker.RunID)),
		fmt.Sprintf(`slot="%s"`, escapeMarkerValue(marker.Slot)),
	}
	if marker.RunURL != "" {
		parts = append(parts, fmt.Sprintf(`run_url="%s"`, escapeMarkerValue(marker.RunURL)))
	}
	if marker.Model != "" {
		parts = append(parts, fmt.Sprintf(`model="%s"`, escapeMarkerValue(marker.Model)))
	}
	if marker.PromptContextSHA != "" {
		parts = append(parts,
			fmt.Sprintf(`prompt_context_sha256_12="%s"`, escapeMarkerValue(marker.PromptContextSHA)),
			fmt.Sprintf(`context_documents="%d"`, marker.ContextDocuments),
			fmt.Sprintf(`selected_skills="%d"`, marker.SelectedSkills),
			fmt.Sprintf(`tool_outputs="%d"`, marker.ToolOutputs),
		)
		if len(marker.PromptVisibleSkills) > 0 {
			parts = append(parts, fmt.Sprintf(`skills="%s"`, escapeMarkerValue(strings.Join(marker.PromptVisibleSkills, ","))))
		}
		if len(marker.PromptVisibleTools) > 0 {
			parts = append(parts, fmt.Sprintf(`tools="%s"`, escapeMarkerValue(strings.Join(marker.PromptVisibleTools, ","))))
		}
	}
	if marker.Usage.Present {
		parts = append(parts,
			fmt.Sprintf(`usage_prompt_tokens="%d"`, marker.Usage.PromptTokens),
			fmt.Sprintf(`usage_completion_tokens="%d"`, marker.Usage.CompletionTokens),
			fmt.Sprintf(`usage_total_tokens="%d"`, marker.Usage.TotalTokens),
			fmt.Sprintf(`usage_cache_read_tokens="%d"`, marker.Usage.CacheReadTokens),
			fmt.Sprintf(`usage_cache_write_tokens="%d"`, marker.Usage.CacheWriteTokens),
		)
	}
	return fmt.Sprintf("<!-- gitclaw:heartbeat %s -->\n%s", strings.Join(parts, " "), strings.TrimSpace(text))
}

func RenderErrorComment(marker ErrorMarker, diagnostic string) string {
	parts := []string{
		fmt.Sprintf(`run_id="%s"`, escapeMarkerValue(marker.RunID)),
		fmt.Sprintf(`event_id="%s"`, escapeMarkerValue(marker.EventID)),
		fmt.Sprintf(`phase="%s"`, escapeMarkerValue(marker.Phase)),
	}
	if marker.RunURL != "" {
		parts = append(parts, fmt.Sprintf(`run_url="%s"`, escapeMarkerValue(marker.RunURL)))
	}
	return fmt.Sprintf(`<!-- gitclaw:error %s -->
GitClaw could not complete this turn.

Diagnostic: %s

See the linked Actions run for details.`, strings.Join(parts, " "), strings.TrimSpace(diagnostic))
}

func HasGitClawMarker(body string) bool {
	return markerPattern.MatchString(body)
}

func HasGitClawErrorMarker(body string) bool {
	return errorMarkerPattern.MatchString(body)
}

func HasBackupRestoreRequestIssueMarker(body string) bool {
	return backupRestoreRequestIssueMarkerPattern.MatchString(body)
}

func HasCheckpointRehearsalIssueMarker(body string) bool {
	return checkpointRehearsalIssueMarkerPattern.MatchString(body)
}

func HasChannelMessageMarker(body string) bool {
	return channelMessageMarkerPattern.MatchString(body)
}

func HasChannelOutboundMarker(body string) bool {
	return channelOutboundMarkerPattern.MatchString(body)
}

func HasChannelDeliverableMarker(body string) bool {
	return channelDeliverableMarkerPattern.MatchString(body)
}

func HasChannelReactionMarker(body string) bool {
	return channelReactionMarkerPattern.MatchString(body)
}

func HasChannelStatusMarker(body string) bool {
	return channelStatusMarkerPattern.MatchString(body)
}

func HasChannelEditMarker(body string) bool {
	return channelEditMarkerPattern.MatchString(body)
}

func HasChannelTopicMarker(body string) bool {
	return channelTopicMarkerPattern.MatchString(body)
}

func HasChannelActivityMarker(body string) bool {
	return channelActivityMarkerPattern.MatchString(body)
}

func HasChannelThreadMarker(body string) bool {
	return channelThreadMarkerPattern.MatchString(body)
}

func HasChannelRoomMarker(body string) bool {
	return channelRoomMarkerPattern.MatchString(body)
}

func HasChannelHuddleMarker(body string) bool {
	return channelHuddleMarkerPattern.MatchString(body)
}

func HasChannelPollMarker(body string) bool {
	return channelPollMarkerPattern.MatchString(body)
}

func HasChannelPollVoteMarker(body string) bool {
	return channelPollVoteMarkerPattern.MatchString(body)
}

func HasChannelRollcallMarker(body string) bool {
	return channelRollcallMarkerPattern.MatchString(body)
}

func HasChannelRsvpMarker(body string) bool {
	return channelRsvpMarkerPattern.MatchString(body)
}

func HasChannelRsvpResponseMarker(body string) bool {
	return channelRsvpResponseMarkerPattern.MatchString(body)
}

func HasChannelTaskMarker(body string) bool {
	return channelTaskMarkerPattern.MatchString(body)
}

func HasChannelWatchMarker(body string) bool {
	return channelWatchMarkerPattern.MatchString(body)
}

func HasChannelStandingOrderProposalMarker(body string) bool {
	return channelStandingOrderProposalMarkerPattern.MatchString(body)
}

func HasChannelClipMarker(body string) bool {
	return channelClipMarkerPattern.MatchString(body)
}

func HasChannelOpenLoopMarker(body string) bool {
	return channelOpenLoopMarkerPattern.MatchString(body)
}

func HasChannelAttachmentMarker(body string) bool {
	return channelAttachmentMarkerPattern.MatchString(body)
}

func HasChannelSnippetMarker(body string) bool {
	return channelSnippetMarkerPattern.MatchString(body)
}

func HasChannelDecisionMarker(body string) bool {
	return channelDecisionMarkerPattern.MatchString(body)
}

func HasChannelDigestMarker(body string) bool {
	return channelDigestMarkerPattern.MatchString(body)
}

func HasChannelIdeaMarker(body string) bool {
	return channelIdeaMarkerPattern.MatchString(body)
}

func HasChannelJamMarker(body string) bool {
	return channelJamMarkerPattern.MatchString(body)
}

func HasChannelKudosMarker(body string) bool {
	return channelKudosMarkerPattern.MatchString(body)
}

func HasChannelRetroMarker(body string) bool {
	return channelRetroMarkerPattern.MatchString(body)
}

func HasChannelPlaybookMarker(body string) bool {
	return channelPlaybookMarkerPattern.MatchString(body)
}

func HasChannelInsightMarker(body string) bool {
	return channelInsightMarkerPattern.MatchString(body)
}

func HasChannelWorkspaceProposalMarker(body string) bool {
	return channelWorkspaceProposalMarkerPattern.MatchString(body)
}

func HasChannelBoardCardMarker(body string) bool {
	return channelBoardCardMarkerPattern.MatchString(body)
}

func HasChannelChecklistMarker(body string) bool {
	return channelChecklistMarkerPattern.MatchString(body)
}

func HasChannelToolsetProposalMarker(body string) bool {
	return channelToolsetProposalMarkerPattern.MatchString(body)
}

func HasChannelPromptProposalMarker(body string) bool {
	return channelPromptProposalMarkerPattern.MatchString(body)
}

func HasChannelBundleProposalMarker(body string) bool {
	return channelBundleProposalMarkerPattern.MatchString(body)
}

func HasChannelIncidentMarker(body string) bool {
	return channelIncidentMarkerPattern.MatchString(body)
}

func HasChannelVoiceMarker(body string) bool {
	return channelVoiceMarkerPattern.MatchString(body)
}

func HasChannelImageMarker(body string) bool {
	return channelImageMarkerPattern.MatchString(body)
}

func HasChannelLinkMarker(body string) bool {
	return channelLinkMarkerPattern.MatchString(body)
}

func HasChannelBookmarkMarker(body string) bool {
	return channelBookmarkMarkerPattern.MatchString(body)
}

func HasChannelForkMarker(body string) bool {
	return channelForkMarkerPattern.MatchString(body)
}

func HasChannelMergeMarker(body string) bool {
	return channelMergeMarkerPattern.MatchString(body)
}

func HasChannelAccessRequestMarker(body string) bool {
	return channelAccessRequestMarkerPattern.MatchString(body)
}

func HasChannelContactMarker(body string) bool {
	return channelContactMarkerPattern.MatchString(body)
}

func HasChannelReminderMarker(body string) bool {
	return channelReminderMarkerPattern.MatchString(body)
}

func HasChannelStateMarker(body string) bool {
	return channelStateMarkerPattern.MatchString(body)
}

func HasChannelStateUpdateMarker(body string) bool {
	return channelStateUpdateMarkerPattern.MatchString(body)
}

func HasChannelDeliveryMarker(body string) bool {
	return channelDeliveryMarkerPattern.MatchString(body)
}

func HasProactiveRunMarker(body string) bool {
	return proactiveRunMarkerPattern.MatchString(body)
}

func HasHeartbeatMarker(body string) bool {
	return heartbeatMarkerPattern.MatchString(body)
}

func ContainsHeartbeatSlot(body, slot string) bool {
	return strings.Contains(body, fmt.Sprintf(`slot="%s"`, slot)) ||
		strings.Contains(body, fmt.Sprintf("slot=%s", slot))
}

func ContainsIdempotencyKey(body, key string) bool {
	return strings.Contains(body, fmt.Sprintf(`idempotency_key="%s"`, key)) ||
		strings.Contains(body, fmt.Sprintf("idempotency_key=%s", key))
}

func StripMarker(body string) string {
	return strings.TrimSpace(markerPattern.ReplaceAllString(body, ""))
}

func StripChannelMessageMarker(body string) string {
	return strings.TrimSpace(channelMessageMarkerPattern.ReplaceAllString(body, ""))
}

func StripChannelOutboundMarker(body string) string {
	return strings.TrimSpace(channelOutboundMarkerPattern.ReplaceAllString(body, ""))
}

func StripChannelDeliverableMarker(body string) string {
	return strings.TrimSpace(channelDeliverableMarkerPattern.ReplaceAllString(body, ""))
}

func StripChannelReactionMarker(body string) string {
	return strings.TrimSpace(channelReactionMarkerPattern.ReplaceAllString(body, ""))
}

func StripChannelStatusMarker(body string) string {
	return strings.TrimSpace(channelStatusMarkerPattern.ReplaceAllString(body, ""))
}

func StripChannelEditMarker(body string) string {
	return strings.TrimSpace(channelEditMarkerPattern.ReplaceAllString(body, ""))
}

func StripChannelTopicMarker(body string) string {
	return strings.TrimSpace(channelTopicMarkerPattern.ReplaceAllString(body, ""))
}

func StripChannelActivityMarker(body string) string {
	return strings.TrimSpace(channelActivityMarkerPattern.ReplaceAllString(body, ""))
}

func withWorkflowDispatchActiveText(ev Event, comments []Comment) Event {
	if ev.Kind != EventWorkflowDispatch || strings.TrimSpace(ev.DispatchID) == "" {
		return ev
	}
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		if channelMessageDispatchID(comment.Body) != ev.DispatchID {
			continue
		}
		ev.ActiveText = StripChannelMessageMarker(comment.Body)
		return ev
	}
	return ev
}

func channelMessageDispatchID(body string) string {
	match := channelMessageMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	channel := markerAttribute(match[1], "channel")
	messageID := markerAttribute(match[1], "message_id")
	if channel == "" || messageID == "" {
		return ""
	}
	return channel + "-" + messageID
}

func markerAttribute(attrs, key string) string {
	needle := key + `="`
	for _, field := range strings.Fields(attrs) {
		if !strings.HasPrefix(field, needle) {
			continue
		}
		value := strings.TrimPrefix(field, needle)
		end := strings.Index(value, `"`)
		if end < 0 {
			return ""
		}
		return value[:end]
	}
	return ""
}

func escapeMarkerValue(value string) string {
	value = strings.ReplaceAll(value, `"`, "")
	value = strings.ReplaceAll(value, "-->", "")
	return value
}
