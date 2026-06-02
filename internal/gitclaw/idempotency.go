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
var channelMessageMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-message\s+([^>]*)-->`)
var channelOutboundMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-outbound\s+([^>]*)-->`)
var channelDeliverableMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-deliverable\s+([^>]*)-->`)
var channelReactionMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-reaction\s+([^>]*)-->`)
var channelStatusMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-status\s+([^>]*)-->`)
var channelEditMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-edit\s+([^>]*)-->`)
var channelThreadMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-thread\s+([^>]*)-->`)
var channelRoomMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-room\s+([^>]*)-->`)
var channelHuddleMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-huddle\s+([^>]*)-->`)
var channelPollMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-poll\s+([^>]*)-->`)
var channelRollcallMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-rollcall\s+([^>]*)-->`)
var channelTaskMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-task\s+([^>]*)-->`)
var channelWatchMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-watch\s+([^>]*)-->`)
var channelStandingOrderProposalMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-standing-order-proposal\s+([^>]*)-->`)
var channelClipMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-clip\s+([^>]*)-->`)
var channelAttachmentMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-attachment\s+([^>]*)-->`)
var channelDecisionMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-decision\s+([^>]*)-->`)
var channelDigestMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-digest\s+([^>]*)-->`)
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

func HasChannelRollcallMarker(body string) bool {
	return channelRollcallMarkerPattern.MatchString(body)
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

func HasChannelAttachmentMarker(body string) bool {
	return channelAttachmentMarkerPattern.MatchString(body)
}

func HasChannelDecisionMarker(body string) bool {
	return channelDecisionMarkerPattern.MatchString(body)
}

func HasChannelDigestMarker(body string) bool {
	return channelDigestMarkerPattern.MatchString(body)
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
