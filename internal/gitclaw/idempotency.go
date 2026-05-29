package gitclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var markerPattern = regexp.MustCompile(`<!--\s*gitclaw:assistant-turn\s+([^>]*)-->`)

func IdempotencyKey(ev Event) string {
	trigger := fmt.Sprintf("issue:%d", ev.Issue.Number)
	if ev.Comment != nil {
		trigger = fmt.Sprintf("comment:%d", ev.Comment.ID)
	}
	seed := fmt.Sprintf("%s|%s|%s|%s", ev.Repo, ev.EventName, trigger, ev.SHA)
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

func HasGitClawMarker(body string) bool {
	return markerPattern.MatchString(body)
}

func ContainsIdempotencyKey(body, key string) bool {
	return strings.Contains(body, fmt.Sprintf(`idempotency_key="%s"`, key)) ||
		strings.Contains(body, fmt.Sprintf("idempotency_key=%s", key))
}

func StripMarker(body string) string {
	return strings.TrimSpace(markerPattern.ReplaceAllString(body, ""))
}

func escapeMarkerValue(value string) string {
	value = strings.ReplaceAll(value, `"`, "")
	value = strings.ReplaceAll(value, "-->", "")
	return value
}
