package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderChannelListReportCarriesDedicatedE2EMarker(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workdir = t.TempDir()
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 12,
			Title:  "@gitclaw /channels list e2e",
			Body:   "Hidden channel list token: CHANNEL_LIST_SECRET.",
		},
	}

	report := RenderChannelReport(ev, cfg, nil)
	for _, want := range []string{
		"GitClaw Channel Report",
		"llm_e2e_required_after_channel_report_change: `true`",
		"llm_e2e_required_after_channel_list_change: `true`",
		"`/channels propose-prompt --prompt-id <id> --message-id <message>`",
		"`/channels propose-bundle --bundle-id <id> --message-id <message>`",
		"`/channels model --message-id <message>`",
		"`/channels skills --message-id <message>`",
		"`/channels tools --message-id <message>`",
		"`/channels backup --message-id <message>`",
		"`/channels profile-status --message-id <message>`",
		"`/channels soul-status --message-id <message>`",
		"`/channels memory-status --message-id <message>`",
		"`/channels roll --dice <expr> --message-id <message>`",
		"`/channels choose --message-id <message>`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("channel list report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"CHANNEL_LIST_SECRET", "@gitclaw /channels list e2e"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("channel list report leaked %q:\n%s", notWant, report)
		}
	}
}
