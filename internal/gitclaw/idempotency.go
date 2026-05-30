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
var channelThreadMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:channel-thread\s+([^>]*)-->`)
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

func HasChannelThreadMarker(body string) bool {
	return channelThreadMarkerPattern.MatchString(body)
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
	start := strings.Index(attrs, needle)
	if start < 0 {
		return ""
	}
	start += len(needle)
	end := strings.Index(attrs[start:], `"`)
	if end < 0 {
		return ""
	}
	return attrs[start : start+end]
}

func escapeMarkerValue(value string) string {
	value = strings.ReplaceAll(value, `"`, "")
	value = strings.ReplaceAll(value, "-->", "")
	return value
}
