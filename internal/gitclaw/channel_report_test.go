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
		"`/channels browser --message-id <message>`",
		"`/channels model --message-id <message>`",
		"`/channels skills --message-id <message>`",
		"`/channels skill-search <query> --message-id <message>`",
		"`/channels skill-info <skill> --message-id <message>`",
		"`/channels skill-map <skill> --map-id <id> --message-id <message>`",
		"`/channels bundle-map <bundle> --map-id <id> --message-id <message>`",
		"`/channels source-map <source> --map-id <id> --message-id <message>`",
		"`/channels tools --message-id <message>`",
		"`/channels tool-search <query> --message-id <message>`",
		"`/channels tool-info <tool> --message-id <message>`",
		"`/channels tool-map <tool> --map-id <id> --message-id <message>`",
		"`/channels dock <target-route> --dock-id <id> --message-id <message>`",
		"`/channels warmup <theme> --warmup-id <id> --message-id <message>`",
		"`/channels spark --spark-id <id> --message-id <message>`",
		"`/channels vibe-check --vibe-id <id> --message-id <message>`",
		"`/channels backup --message-id <message>`",
		"`/channels recovery-map <scope> --map-id <id> --message-id <message>`",
		"`/channels backup-search <query> --message-id <message>`",
		"`/channels backup-timeline --timeline-id <id> --message-id <message>`",
		"`/channels backup-freshness --freshness-id <id> --message-id <message>`",
		"`/channels backup-info <issue> --message-id <message>`",
		"`/channels checkpoint-status --message-id <message>`",
		"`/channels availability --message-id <message>`",
		"`/channels topic --topic-id <id>`",
		"`/channels activity <activity> --activity-id <id>`",
		"`/channels profile-status --message-id <message>`",
		"`/channels soul-status --message-id <message>`",
		"`/channels soul-info <path> --message-id <message>`",
		"`/channels soul-risk --message-id <message>`",
		"`/channels soul-search <query> --message-id <message>`",
		"`/channels memory-status --message-id <message>`",
		"`/channels memory-search <query> --message-id <message>`",
		"`/channels roll --dice <expr> --message-id <message>`",
		"`/channels choose --message-id <message>`",
		"`/channels oracle --choose-id <id> --message-id <message>`",
		"`/channels mood <mood> --message-id <message>`",
		"`/channels room-pulse <focus> --pulse-id <id> --message-id <message>`",
		"`/channels quick-replies <lane> --reply-id <id> --message-id <message>`",
		"`/channels status-wheel <lane> --wheel-id <id> --message-id <message>`",
		"`/channels toast <title> --toast-id <id> --message-id <message>`",
		"`/channels postcard <title> --postcard-id <id> --message-id <message>`",
		"`/channels timer <duration> --timer-id <id> --message-id <message>`",
		"`/channels bingo <theme> --bingo-id <id> --message-id <message>`",
		"`/channels haiku <theme> --haiku-id <id> --message-id <message>`",
		"`/channels soundtrack <theme> --soundtrack-id <id> --message-id <message>`",
		"`/channels story-dice <theme> --story-dice-id <id> --message-id <message>`",
		"`/channels coach <lane> --coach-id <id> --message-id <message>`",
		"`/channels session-search <query> --message-id <message>`",
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
