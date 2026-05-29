package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderCommandReportListsCatalogWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 109,
			"title": "@gitclaw /help",
			"body": "Hidden command body token: COMMAND_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderCommandReport(ev, DefaultConfig())
	for _, want := range []string{
		"GitClaw Commands Report",
		"Generated without a model call",
		"commands: `15`",
		"aliases: `7`",
		"local_cli_helpers: `31`",
		"run_mode: `read-only`",
		"### Slash Commands",
		"### Local CLI Helpers",
		"`/help` model=`gitclaw/commands`",
		"aliases=`/commands`",
		"`/backup` model=`gitclaw/backup`",
		"`/tools` model=`gitclaw/tools`",
		"`gitclaw channels list` command=`/channels`",
		"`gitclaw config list` command=`/config`",
		"`gitclaw context list` command=`/context`",
		"`gitclaw prompt list` command=`/prompt`",
		"`gitclaw models list` command=`/models`",
		"`gitclaw policy list` command=`/policy`",
		"`gitclaw backup list` command=`/backup`",
		"`gitclaw backup stats` command=`/backup`",
		"`gitclaw backup search <query>` command=`/backup`",
		"`gitclaw backup retention-plan` command=`/backup`",
		"`gitclaw memory validate` command=`/memory`",
		"`gitclaw memory list` command=`/memory`",
		"`gitclaw memory search <query>` command=`/memory`",
		"`gitclaw soul list` command=`/soul`",
		"`gitclaw soul search <query>` command=`/soul`",
		"`gitclaw skills list` command=`/skills`",
		"`gitclaw skills info <name>` command=`/skills`",
		"`gitclaw skills search <query>` command=`/skills`",
		"`gitclaw tools validate` command=`/tools`",
		"`gitclaw tools list` command=`/tools`",
		"`gitclaw tools search <query>` command=`/tools`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("command report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "COMMAND_BODY_SECRET") {
		t.Fatalf("command report leaked issue body token:\n%s", report)
	}
}

func TestIsCommandReportRequestAcceptsCommandsAlias(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 110,
			"title": "@gitclaw /commands",
			"body": "",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	if !IsCommandReportRequest(ev, DefaultConfig()) {
		t.Fatalf("/commands should be accepted as command report alias")
	}
}
