package gitclaw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func RunCLI(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gitclaw handle --event <path>")
	}
	switch args[0] {
	case "preflight":
		return runPreflight(ctx, args[1:])
	case "handle":
		return runHandle(ctx, args[1:])
	case "backup":
		return runBackup(ctx, args[1:])
	case "heartbeat":
		return runHeartbeatCommand(ctx, args[1:])
	case "hooks", "hook":
		return runHooksCommand(args[1:])
	case "plugins", "plugin":
		return runPluginsCommand(args[1:])
	case "tasks", "task":
		return runTasksCommand(args[1:])
	case "agents", "agent":
		return runAgentsCommand(args[1:])
	case "nodes", "node":
		return runNodesCommand(args[1:])
	case "artifacts", "artifact":
		return runArtifactsCommand(args[1:])
	case "channel-ingest":
		return runChannelIngestCommand(ctx, args[1:])
	case "channel-send":
		return runChannelSendCommand(ctx, args[1:])
	case "channel-status":
		return runChannelStatusCommand(ctx, args[1:])
	case "channel-edit":
		return runChannelEditCommand(ctx, args[1:])
	case "channel-react", "channel-reaction":
		return runChannelReactionCommand(ctx, args[1:])
	case "channel-state":
		return runChannelStateCommand(ctx, args[1:])
	case "channel-gateway":
		return runChannelGatewayCommand(ctx, args[1:])
	case "channel-delivery":
		return runChannelDeliveryCommand(ctx, args[1:])
	case "channel-outbox":
		return runChannelOutboxCommand(ctx, args[1:])
	case "channels", "channel":
		return runChannelsCommand(args[1:])
	case "approvals", "approval":
		return runApprovalsCommand(args[1:])
	case "checkpoints", "checkpoint":
		return runCheckpointsCommand(args[1:])
	case "rollback":
		return runRollbackCommand(args[1:])
	case "proactive":
		return runProactiveCommand(ctx, args[1:])
	case "migrate", "migration":
		return runMigrateCommand(args[1:])
	case "profile", "profiles":
		return runProfileCommand(args[1:])
	case "runs", "run", "ledger":
		return runRunsCommand(args[1:])
	case "sandbox", "sandboxes", "exec-policy":
		return runSandboxCommand(args[1:])
	case "security", "sec":
		return runSecurityCommand(args[1:])
	case "skills":
		return runSkillsCommand(args[1:])
	case "bundles", "bundle":
		return runBundlesCommand(args[1:])
	case "memory":
		return runMemoryCommand(args[1:])
	case "soul":
		return runSoulCommand(args[1:])
	case "secrets", "secret":
		return runSecretsCommand(args[1:])
	case "tools":
		return runToolsCommand(args[1:])
	case "models", "model":
		return runModelsCommand(args[1:])
	case "orders", "standing-orders":
		return runOrdersCommand(args[1:])
	case "config", "configuration":
		return runConfigCommand(args[1:])
	case "policy":
		return runPolicyCommand(args[1:])
	case "context":
		return runContextCommand(args[1:])
	case "diffs", "diff", "changes":
		return runDiffsCommand(args[1:])
	case "workspace", "workdir", "repo":
		return runWorkspaceCommand(args[1:])
	case "research", "landscape":
		return runResearchCommand(args[1:])
	case "prompt", "budget", "prompt-budget":
		return runPromptCommand(args[1:])
	case "session":
		return runSessionCommand(args[1:])
	case "doctor":
		return runDoctorCommand(args[1:])
	case "commands":
		return runCommandsCommand(args[1:])
	case "version":
		fmt.Println("gitclaw dev")
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runResearchCommand(args []string) error {
	if len(args) == 0 {
		return runResearchCatalogCommand(nil)
	}
	switch args[0] {
	case "catalog", "sources", "source", "coverage", "map", "verify", "list":
		return runResearchCatalogCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw research [catalog|sources|coverage|verify]")
	}
}

func runResearchCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown research catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderResearchCLIReport(cfg, repoContext))
	return nil
}

func runMemoryCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw memory catalog|snapshot|provenance|verify|risk|validate|timeline|list|promote-plan [target]|info <path>|search <query>")
	}
	switch args[0] {
	case "catalog", "index", "memory-catalog", "discovery", "eligible":
		return runMemoryCatalogCommand(args[1:])
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return runMemorySnapshotCommand(args[1:])
	case "provenance", "git-history":
		return runMemoryProvenanceCommand(args[1:])
	case "verify":
		return runMemoryVerifyCommand(args[1:])
	case "risk", "risk-audit":
		return runMemoryRiskCommand(args[1:])
	case "validate":
		return runMemoryValidateCommand(args[1:])
	case "timeline", "history", "ledger":
		return runMemoryTimelineCommand(args[1:])
	case "list":
		return runMemoryListCommand(args[1:])
	case "promote-plan", "promote", "remember-plan":
		return runMemoryPromotePlanCommand(args[1:])
	case "info":
		return runMemoryInfoCommand(args[1:])
	case "search":
		return runMemorySearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown memory command %q", args[0])
	}
}

func runMemorySnapshotCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory snapshot argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemorySnapshotCLIReport(cfg, repoContext))
	return nil
}

func runMemoryCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryCatalogCLIReport(cfg, repoContext))
	return nil
}

func runMemoryProvenanceCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory provenance argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryProvenanceCLIReport(cfg, repoContext))
	return nil
}

func runProfileCommand(args []string) error {
	if len(args) == 0 {
		return runProfileShowCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "capabilities", "index":
		return runProfileCatalogCommand(args[1:])
	case "show", "verify", "list":
		return runProfileShowCommand(args[1:])
	case "provenance", "history", "git", "git-history":
		return runProfileProvenanceCommand(args[1:])
	case "search":
		return runProfileSearchCommand(args[1:])
	case "diff", "changes", "compare":
		return runProfileDiffCommand(args[1:])
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return runProfileSnapshotCommand(args[1:])
	case "manifest", "portability", "portable", "export-plan", "export", "package-plan", "distribution":
		return runProfileManifestCommand(args[1:])
	case "risk", "risk-audit":
		return runProfileRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw profile [catalog|show|verify|list|provenance|search <query>|diff [base-ref]|snapshot|manifest|export-plan|risk]")
	}
}

func runProfileCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown profile catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderProfileCatalogCLIReport(cfg, repoContext))
	return nil
}

func runProfileShowCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown profile argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderProfileCLIReport(cfg, repoContext))
	return nil
}

func runProfileSnapshotCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown profile snapshot argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "profile snapshot"}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderProfileSnapshotCLIReport(cfg, repoContext))
	return nil
}

func runProfileProvenanceCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown profile provenance argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "profile provenance"}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderProfileProvenanceCLIReport(cfg, repoContext))
	return nil
}

func runProfileSearchCommand(args []string) error {
	query := cleanMemorySearchQuery(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("usage: gitclaw profile search <query>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "profile search " + query}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderProfileSearchCLIReport(cfg, repoContext, query))
	return nil
}

func runProfileDiffCommand(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: gitclaw profile diff [base-ref]")
	}
	baseRef := defaultProfileDiffBaseRef
	if len(args) == 1 {
		baseRef = strings.TrimSpace(args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "profile diff"}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderProfileDiffCLIReport(cfg, repoContext, baseRef))
	return nil
}

func runProfileRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown profile risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderProfileRiskCLIReport(cfg, repoContext))
	return nil
}

func runProfileManifestCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown profile manifest argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderProfileManifestCLIReport(cfg, repoContext))
	return nil
}

func runRunsCommand(args []string) error {
	if len(args) == 0 {
		return runRunsCurrentCommand(nil)
	}
	switch args[0] {
	case "current", "verify", "list":
		return runRunsCurrentCommand(args[1:])
	case "history", "timeline":
		return runRunsHistoryCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw runs [current|verify|list|history --backup <issue.json>]")
	}
}

func runRunsCurrentCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown runs argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderRunCLIReport(cfg, repoContext))
	return nil
}

func runRunsHistoryCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown runs history argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw runs history --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderRunHistoryCLIReport(backupPath, backup))
	return nil
}

func runSandboxCommand(args []string) error {
	if len(args) == 0 {
		return runSandboxExplainCommand(nil)
	}
	switch args[0] {
	case "explain", "verify", "list":
		return runSandboxExplainCommand(args[1:])
	case "risk", "risk-audit":
		return runSandboxRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw sandbox [explain|verify|list|risk]")
	}
}

func runSandboxExplainCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown sandbox argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSandboxCLIReport(cfg, repoContext))
	return nil
}

func runSandboxRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown sandbox risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSandboxRiskReport(Event{}, cfg, repoContext, false))
	return nil
}

func runSecurityCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "audit" && args[0] != "risk" && args[0] != "risk-audit" && args[0] != "list") {
		return fmt.Errorf("usage: gitclaw security [audit|risk]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	report, err := RenderSecurityCLIReport(cfg, repoContext)
	if err != nil {
		return err
	}
	fmt.Println(report)
	return nil
}

func runCheckpointsCommand(args []string) error {
	if len(args) == 0 {
		return runCheckpointsStatusCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "index", "map":
		return runCheckpointsCatalogCommand(args[1:])
	case "status", "list", "verify":
		return runCheckpointsStatusCommand(args[1:])
	case "preview", "diff", "plan":
		return runCheckpointsPreviewCommand(args[1:])
	case "risk", "risk-audit":
		return runCheckpointsRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw checkpoints [catalog|status|list|preview|risk|verify]")
	}
}

func runApprovalsCommand(args []string) error {
	if len(args) == 0 {
		return runApprovalsListCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "gates", "index":
		return runApprovalsCatalogCommand(args[1:])
	case "list", "verify":
		return runApprovalsListCommand(args[1:])
	case "provenance", "trace", "evidence":
		return runApprovalsProvenanceCommand(args[1:])
	case "risk", "risk-audit":
		return runApprovalsRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw approvals [catalog|list|verify|provenance|risk]")
	}
}

func runApprovalsCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown approvals catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderApprovalCatalogCLIReport(cfg))
	return nil
}

func runApprovalsListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown approvals argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderApprovalCLIReport(cfg))
	return nil
}

func runApprovalsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown approvals risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderApprovalRiskCLIReport(cfg))
	return nil
}

func runApprovalsProvenanceCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown approvals provenance argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderApprovalProvenanceCLIReport(cfg))
	return nil
}

func runOrdersCommand(args []string) error {
	if len(args) == 0 {
		return runOrdersListCommand(nil)
	}
	switch args[0] {
	case "list", "verify":
		return runOrdersListCommand(args[1:])
	case "risk", "risk-audit":
		return runOrdersRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw orders [list|verify|risk]")
	}
}

func runOrdersListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown orders argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderStandingOrdersCLIReport(cfg))
	return nil
}

func runOrdersRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown orders risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderStandingOrdersRiskCLIReport(cfg))
	return nil
}

func runHooksCommand(args []string) error {
	if len(args) == 0 {
		return runHooksListCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "index", "map":
		return runHooksCatalogCommand(args[1:])
	case "list", "verify":
		return runHooksListCommand(args[1:])
	case "risk", "risk-audit":
		return runHooksRiskCommand(args[1:])
	case "provenance", "history", "timeline":
		return runHooksProvenanceCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw hooks [catalog|list|risk|verify|provenance]")
	}
}

func runHooksCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown hooks catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderHookCatalogCLIReport(cfg))
	return nil
}

func runHooksListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown hooks argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderHookCLIReport(cfg))
	return nil
}

func runHooksRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown hooks risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderHookRiskCLIReport(cfg))
	return nil
}

func runHooksProvenanceCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown hooks provenance argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderHookProvenanceCLIReport(cfg))
	return nil
}

func runPluginsCommand(args []string) error {
	if len(args) == 0 {
		return runPluginsListCommand(nil)
	}
	switch args[0] {
	case "list", "verify":
		return runPluginsListCommand(args[1:])
	case "risk", "risk-audit":
		return runPluginsRiskCommand(args[1:])
	case "mcp", "mcps":
		return runPluginsMCPCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw plugins [list|risk|verify|mcp [list|risk|provenance|info <name>]]")
	}
}

func runPluginsMCPCommand(args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		if len(args) > 1 {
			return fmt.Errorf("unknown plugins mcp list argument %q", args[1])
		}
		fmt.Println(RenderMCPCLIReport(cfg))
		return nil
	}
	if args[0] == "risk" || args[0] == "risk-audit" {
		if len(args) > 1 {
			return fmt.Errorf("unknown plugins mcp risk argument %q", args[1])
		}
		fmt.Println(RenderMCPRiskCLIReport(cfg))
		return nil
	}
	if args[0] == "provenance" || args[0] == "history" || args[0] == "timeline" {
		if len(args) > 1 {
			return fmt.Errorf("unknown plugins mcp provenance argument %q", args[1])
		}
		fmt.Println(RenderMCPProvenanceCLIReport(cfg))
		return nil
	}
	if args[0] == "info" || args[0] == "show" {
		if len(args) != 2 {
			return fmt.Errorf("usage: gitclaw plugins mcp info <name>")
		}
		report := RenderMCPInfoCLIReport(cfg, args[1])
		fmt.Println(report)
		if len(matchingMCPCards(BuildMCPReport(cfg).Cards, args[1])) == 0 {
			return fmt.Errorf("MCP spec %q not found", args[1])
		}
		return nil
	}
	return fmt.Errorf("usage: gitclaw plugins mcp [list|risk|provenance|info <name>]")
}

func runPluginsListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown plugins argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderPluginCLIReport(cfg))
	return nil
}

func runPluginsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown plugins risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderPluginRiskCLIReport(cfg))
	return nil
}

func runTasksCommand(args []string) error {
	if len(args) == 0 {
		return runTasksListCommand(nil)
	}
	switch args[0] {
	case "list", "verify":
		return runTasksListCommand(args[1:])
	case "risk", "risk-audit":
		return runTasksRiskCommand(args[1:])
	case "ledger", "timeline", "history":
		return runTasksLedgerCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw tasks [list|risk|verify|ledger [--backup <issue.json>]]")
	}
}

func runTasksListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tasks argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderTaskCLIReport(cfg))
	return nil
}

func runTasksRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tasks risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderTaskRiskCLIReport(cfg))
	return nil
}

func runTasksLedgerCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown tasks ledger argument %q", args[i])
		}
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	if backupPath == "" {
		fmt.Println(RenderTaskLedgerCLIReport(cfg))
		return nil
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderTaskLedgerBackupCLIReport(cfg, backupPath, backup))
	return nil
}

func runAgentsCommand(args []string) error {
	if len(args) == 0 {
		return runAgentsListCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "capabilities", "index":
		return runAgentsCatalogCommand(args[1:])
	case "list", "verify":
		return runAgentsListCommand(args[1:])
	case "provenance", "history", "git-history":
		return runAgentsProvenanceCommand(args[1:])
	case "risk", "risk-audit":
		return runAgentsRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw agents [catalog|list|provenance|risk|verify]")
	}
}

func runAgentsCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown agents catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderAgentCatalogCLIReport(cfg))
	return nil
}

func runAgentsListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown agents argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderAgentCLIReport(cfg))
	return nil
}

func runAgentsProvenanceCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown agents provenance argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderAgentProvenanceCLIReport(cfg))
	return nil
}

func runAgentsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown agents risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderAgentRiskCLIReport(cfg))
	return nil
}

func runNodesCommand(args []string) error {
	if len(args) == 0 {
		return runNodesListCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "capabilities", "index":
		return runNodesCatalogCommand(args[1:])
	case "list", "verify":
		return runNodesListCommand(args[1:])
	case "risk", "risk-audit":
		return runNodesRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw nodes [catalog|list|risk|verify]")
	}
}

func runNodesCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown nodes catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderNodeCatalogCLIReport(cfg))
	return nil
}

func runNodesListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown nodes argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderNodeCLIReport(cfg))
	return nil
}

func runNodesRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown nodes risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderNodeRiskCLIReport(cfg))
	return nil
}

func runArtifactsCommand(args []string) error {
	if len(args) == 0 {
		return runArtifactsListCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "index", "uploads":
		return runArtifactsCatalogCommand(args[1:])
	case "list", "verify":
		return runArtifactsListCommand(args[1:])
	case "risk", "risk-audit":
		return runArtifactsRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw artifacts [catalog|list|risk|verify]")
	}
}

func runArtifactsCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown artifacts catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderArtifactCatalogCLIReport(cfg))
	return nil
}

func runArtifactsListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown artifacts argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderArtifactCLIReport(cfg))
	return nil
}

func runArtifactsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown artifacts risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderArtifactRiskCLIReport(cfg))
	return nil
}

func runRollbackCommand(args []string) error {
	if len(args) == 0 {
		return runCheckpointsStatusCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "index", "map":
		return runCheckpointsCatalogCommand(args[1:])
	case "diff", "preview", "plan":
		return runCheckpointsPreviewCommand(args[1:])
	case "list", "status":
		return runCheckpointsStatusCommand(args[1:])
	case "risk", "risk-audit":
		return runCheckpointsRiskCommand(args[1:])
	default:
		return fmt.Errorf("gitclaw rollback is inspect-only; use: gitclaw rollback [catalog|diff|list|risk]")
	}
}

func runCheckpointsCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown checkpoints catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	report := BuildCheckpointReport(cfg.Workdir)
	fmt.Println(RenderCheckpointCatalogCLIReport(report))
	return nil
}

func runCheckpointsStatusCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown checkpoints argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	report := BuildCheckpointReport(cfg.Workdir)
	fmt.Println(RenderCheckpointCLIReport(report))
	return nil
}

func runCheckpointsPreviewCommand(args []string) error {
	target, err := parseCheckpointPreviewArgs(args)
	if err != nil {
		return err
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderCheckpointPreviewCLIReport(cfg.Workdir, target))
	return nil
}

func runCheckpointsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown checkpoints risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	report := BuildCheckpointReport(cfg.Workdir)
	fmt.Println(RenderCheckpointRiskCLIReport(report))
	return nil
}

func parseCheckpointPreviewArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) == 1 {
		return args[0], nil
	}
	if len(args) == 2 && (args[0] == "--to" || args[0] == "--ref" || args[0] == "target") {
		return args[1], nil
	}
	return "", fmt.Errorf("usage: gitclaw rollback diff [--to <ref>|<ref>]")
}

func runMemoryVerifyCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory verify argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryVerifyReport(Event{}, cfg, repoContext))
	return nil
}

func runMemoryRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryRiskCLIReport(cfg, repoContext))
	return nil
}

func runMemoryListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory list argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryCLIReport(cfg, repoContext))
	return nil
}

func runMemoryTimelineCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory timeline argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryTimelineCLIReport(cfg, repoContext))
	return nil
}

func runMemoryInfoCommand(args []string) error {
	pathFlag := ""
	var pathParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path":
			if i+1 >= len(args) {
				return fmt.Errorf("--path requires a value")
			}
			pathFlag = args[i+1]
			i++
		default:
			pathParts = append(pathParts, args[i])
		}
	}
	path := strings.TrimSpace(strings.Join(pathParts, " "))
	if strings.TrimSpace(pathFlag) != "" {
		path = strings.TrimSpace(pathFlag)
	}
	if path == "" {
		return fmt.Errorf("usage: gitclaw memory info <path>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "memory info " + path}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryInfoCLIReport(cfg, repoContext, path))
	return nil
}

func runMemoryPromotePlanCommand(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: gitclaw memory promote-plan [target]")
	}
	target := "long-term"
	if len(args) == 1 {
		target = args[0]
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "memory promote-plan " + target}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryPromotePlanCLIReport(cfg, repoContext, target))
	return nil
}

func runMemorySearchCommand(args []string) error {
	maxResults := defaultMemorySearchMaxResults
	queryFlag := ""
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--query":
			if i+1 >= len(args) {
				return fmt.Errorf("--query requires a value")
			}
			queryFlag = args[i+1]
			i++
		case "--max-results":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-results requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-results: %q", args[i+1])
			}
			maxResults = parsed
			i++
		default:
			queryParts = append(queryParts, args[i])
		}
	}
	query := strings.TrimSpace(strings.Join(queryParts, " "))
	if strings.TrimSpace(queryFlag) != "" {
		query = strings.TrimSpace(queryFlag)
	}
	if query == "" {
		return fmt.Errorf("usage: gitclaw memory search <query>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "memory search " + query}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemorySearchReport(Event{}, cfg, repoContext, query, maxResults))
	return nil
}

func runMemoryValidateCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory validate argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryValidationReport(Event{}, cfg, repoContext))
	return nil
}

func runSoulCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw soul catalog|anchors|snapshot|provenance|verify|risk|validate|list|edit-plan <path>|info <path>|search <query>")
	}
	switch args[0] {
	case "catalog", "index", "profile-catalog", "authority-catalog":
		return runSoulCatalogCommand(args[1:])
	case "anchors", "anchor", "authority", "map":
		return runSoulAnchorsCommand(args[1:])
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return runSoulSnapshotCommand(args[1:])
	case "provenance", "history", "timeline":
		return runSoulProvenanceCommand(args[1:])
	case "verify":
		return runSoulVerifyCommand(args[1:])
	case "risk", "risk-audit":
		return runSoulRiskCommand(args[1:])
	case "validate":
		return runSoulValidateCommand(args[1:])
	case "list":
		return runSoulListCommand(args[1:])
	case "edit-plan", "plan":
		return runSoulEditPlanCommand(args[1:])
	case "info":
		return runSoulInfoCommand(args[1:])
	case "search":
		return runSoulSearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown soul command %q", args[0])
	}
}

func runSoulCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulCatalogCLIReport(repoContext))
	return nil
}

func runSoulAnchorsCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul anchors argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulAnchorsReport(repoContext))
	return nil
}

func runSoulSnapshotCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul snapshot argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulSnapshotCLIReport(repoContext))
	return nil
}

func runSoulVerifyCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul verify argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulVerifyReport(repoContext))
	return nil
}

func runSoulProvenanceCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul provenance argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulProvenanceCLIReport(cfg, repoContext))
	return nil
}

func runSoulRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulRiskReport(repoContext))
	return nil
}

func runSoulListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul list argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulCLIReport(repoContext))
	return nil
}

func runSoulEditPlanCommand(args []string) error {
	pathFlag := ""
	var pathParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path":
			if i+1 >= len(args) {
				return fmt.Errorf("--path requires a value")
			}
			pathFlag = args[i+1]
			i++
		default:
			pathParts = append(pathParts, args[i])
		}
	}
	path := strings.TrimSpace(strings.Join(pathParts, " "))
	if strings.TrimSpace(pathFlag) != "" {
		path = strings.TrimSpace(pathFlag)
	}
	if path == "" {
		return fmt.Errorf("usage: gitclaw soul edit-plan <path>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "soul edit-plan " + path}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulEditPlanCLIReport(cfg, repoContext, path))
	return nil
}

func runSoulInfoCommand(args []string) error {
	pathFlag := ""
	var pathParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path":
			if i+1 >= len(args) {
				return fmt.Errorf("--path requires a value")
			}
			pathFlag = args[i+1]
			i++
		default:
			pathParts = append(pathParts, args[i])
		}
	}
	path := strings.TrimSpace(strings.Join(pathParts, " "))
	if strings.TrimSpace(pathFlag) != "" {
		path = strings.TrimSpace(pathFlag)
	}
	if path == "" {
		return fmt.Errorf("usage: gitclaw soul info <path>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "soul info " + path}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulInfoCLIReport(cfg, repoContext, path))
	return nil
}

func runSoulSearchCommand(args []string) error {
	maxResults := defaultSoulSearchMaxResults
	queryFlag := ""
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--query":
			if i+1 >= len(args) {
				return fmt.Errorf("--query requires a value")
			}
			queryFlag = args[i+1]
			i++
		case "--max-results":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-results requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-results: %q", args[i+1])
			}
			maxResults = parsed
			i++
		default:
			queryParts = append(queryParts, args[i])
		}
	}
	query := strings.TrimSpace(strings.Join(queryParts, " "))
	if strings.TrimSpace(queryFlag) != "" {
		query = strings.TrimSpace(queryFlag)
	}
	if query == "" {
		return fmt.Errorf("usage: gitclaw soul search <query>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "soul search " + query}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulSearchReport(Event{}, repoContext, query, maxResults))
	return nil
}

func runSoulValidateCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul validate argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulValidationReport(repoContext))
	return nil
}

func runCommandsCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown commands argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderCommandCLIReport(cfg))
	return nil
}

func runChannelsCommand(args []string) error {
	if len(args) > 2 {
		return fmt.Errorf("usage: gitclaw channels [list|verify|risk|info <provider>]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		fmt.Println(RenderChannelCLIReport(cfg))
		return nil
	}
	if args[0] == "list" {
		if len(args) != 1 {
			return fmt.Errorf("unknown channels list argument %q", args[1])
		}
		fmt.Println(RenderChannelCLIReport(cfg))
		return nil
	}
	if args[0] == "verify" {
		if len(args) != 1 {
			return fmt.Errorf("unknown channels verify argument %q", args[1])
		}
		fmt.Println(RenderChannelVerifyCLIReport(cfg))
		return nil
	}
	if args[0] == "risk" || args[0] == "risk-audit" {
		if len(args) != 1 {
			return fmt.Errorf("unknown channels risk argument %q", args[1])
		}
		fmt.Println(RenderChannelRiskCLIReport(cfg))
		return nil
	}
	if args[0] == "info" {
		if len(args) != 2 {
			return fmt.Errorf("usage: gitclaw channels info <provider>")
		}
		report := RenderChannelInfoCLIReport(cfg, args[1])
		fmt.Println(report)
		if _, ok := lookupChannelProvider(args[1]); !ok {
			return fmt.Errorf("channel provider %q not supported", args[1])
		}
		return nil
	}
	return fmt.Errorf("usage: gitclaw channels [list|verify|risk|info <provider>]")
}

func runModelsCommand(args []string) error {
	if len(args) == 0 || args[0] == "list" {
		if len(args) > 1 {
			return fmt.Errorf("unknown models list argument %q", args[1])
		}
		cfg, err := LoadEffectiveConfig()
		if err != nil {
			return err
		}
		fmt.Println(RenderModelCLIReport(cfg))
		return nil
	}
	if args[0] == "catalog" || args[0] == "default" || args[0] == "defaults" || args[0] == "selection" || args[0] == "select" || args[0] == "available" {
		if len(args) > 1 {
			return fmt.Errorf("unknown models catalog argument %q", args[1])
		}
		cfg, err := LoadEffectiveConfig()
		if err != nil {
			return err
		}
		fmt.Println(RenderModelCatalogCLIReport(cfg))
		return nil
	}
	if args[0] == "risk" || args[0] == "risk-audit" {
		if len(args) > 1 {
			return fmt.Errorf("unknown models risk argument %q", args[1])
		}
		cfg, err := LoadEffectiveConfig()
		if err != nil {
			return err
		}
		fmt.Println(RenderModelRiskCLIReport(cfg))
		return nil
	}
	if args[0] == "usage" || args[0] == "tokens" || args[0] == "token-use" {
		if len(args) > 1 {
			return fmt.Errorf("unknown models usage argument %q", args[1])
		}
		cfg, err := LoadEffectiveConfig()
		if err != nil {
			return err
		}
		repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
		if err != nil {
			return err
		}
		fmt.Println(RenderModelUsageCLIReport(cfg, repoContext))
		return nil
	}
	if args[0] == "cost" || args[0] == "costs" || args[0] == "spend" || args[0] == "billing" || args[0] == "bill" || args[0] == "budget" {
		if len(args) > 1 {
			return fmt.Errorf("unknown models cost argument %q", args[1])
		}
		cfg, err := LoadEffectiveConfig()
		if err != nil {
			return err
		}
		repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
		if err != nil {
			return err
		}
		fmt.Println(RenderModelCostCLIReport(cfg, repoContext))
		return nil
	}
	return fmt.Errorf("usage: gitclaw models [list|catalog|usage|cost|risk]")
}

func runConfigCommand(args []string) error {
	if len(args) == 0 || args[0] == "list" {
		if len(args) > 1 {
			return fmt.Errorf("unknown config list argument %q", args[1])
		}
		cfg, err := LoadEffectiveConfig()
		if err != nil {
			return err
		}
		fmt.Println(RenderConfigCLIReport(cfg))
		return nil
	}
	if args[0] == "risk" || args[0] == "risk-audit" {
		if len(args) > 1 {
			return fmt.Errorf("unknown config risk argument %q", args[1])
		}
		cfg, err := LoadEffectiveConfig()
		if err != nil {
			return err
		}
		fmt.Println(RenderConfigRiskCLIReport(cfg))
		return nil
	}
	return fmt.Errorf("usage: gitclaw config [list|risk]")
}

func runPolicyCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "list" && args[0] != "verify" && args[0] != "risk" && args[0] != "risk-audit") {
		return fmt.Errorf("usage: gitclaw policy [list|verify|risk]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	if len(args) == 1 && args[0] == "verify" {
		fmt.Println(RenderPolicyVerifyCLIReport(cfg, repoContext))
		return nil
	}
	if len(args) == 1 && (args[0] == "risk" || args[0] == "risk-audit") {
		fmt.Println(RenderPolicyRiskCLIReport(cfg, repoContext))
		return nil
	}
	fmt.Println(RenderPolicyCLIReport(cfg, repoContext))
	return nil
}

func runContextCommand(args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	if len(args) == 0 || (len(args) == 1 && args[0] == "list") {
		repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
		if err != nil {
			return err
		}
		fmt.Println(RenderContextCLIReport(cfg, repoContext))
		return nil
	}
	if len(args) == 1 && (args[0] == "risk" || args[0] == "risk-audit") {
		repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
		if err != nil {
			return err
		}
		fmt.Println(RenderContextRiskCLIReport(cfg, repoContext))
		return nil
	}
	if len(args) >= 2 && args[0] == "info" {
		path := cleanContextLookupPath(strings.Join(args[1:], " "))
		if path == "" {
			return fmt.Errorf("usage: gitclaw context info <path>")
		}
		repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "context info " + path}}, cfg)
		if err != nil {
			return err
		}
		report := RenderContextInfoCLIReport(cfg, repoContext, path)
		fmt.Println(report)
		if len(BuildContextInfoMatches(repoContext, path)) == 0 {
			return fmt.Errorf("context %q not found", path)
		}
		return nil
	}
	return fmt.Errorf("usage: gitclaw context [list|risk|info <path>]")
}

func runDiffsCommand(args []string) error {
	if len(args) == 0 {
		return runDiffsSummaryCommand(nil)
	}
	switch args[0] {
	case "summary", "list", "verify":
		return runDiffsSummaryCommand(args[1:])
	case "risk", "risk-audit":
		return runDiffsRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw diffs [summary|risk|verify]")
	}
}

func runDiffsSummaryCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown diffs argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderDiffCLIReport(cfg))
	return nil
}

func runDiffsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown diffs risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderDiffRiskCLIReport(cfg))
	return nil
}

func runWorkspaceCommand(args []string) error {
	if len(args) == 0 {
		return runWorkspaceSummaryCommand(nil)
	}
	switch args[0] {
	case "catalog", "commands", "capabilities", "index":
		return runWorkspaceCatalogCommand(args[1:])
	case "summary", "list", "verify":
		return runWorkspaceSummaryCommand(args[1:])
	case "risk", "risk-audit":
		return runWorkspaceRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw workspace [catalog|summary|risk|verify]")
	}
}

func runWorkspaceCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown workspace catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderWorkspaceCatalogCLIReport(cfg))
	return nil
}

func runWorkspaceSummaryCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown workspace argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderWorkspaceCLIReport(cfg))
	return nil
}

func runWorkspaceRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown workspace risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderWorkspaceRiskCLIReport(cfg))
	return nil
}

func runPromptCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && !isPromptCLISubcommand(args[0])) {
		return fmt.Errorf("usage: gitclaw prompt [list|pack|context|cache|compression|risk]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	if len(args) == 1 && (args[0] == "risk" || args[0] == "risk-audit") {
		fmt.Println(RenderPromptRiskCLIReport(cfg, repoContext))
		return nil
	}
	if len(args) == 1 && (args[0] == "pack" || args[0] == "pack-plan" || args[0] == "packing" || args[0] == "context-pack") {
		fmt.Println(RenderPromptPackCLIReport(cfg, repoContext))
		return nil
	}
	if len(args) == 1 && (args[0] == "cache" || args[0] == "cache-plan" || args[0] == "cache-status" || args[0] == "cache-readiness") {
		fmt.Println(RenderPromptCacheCLIReport(cfg, repoContext))
		return nil
	}
	if len(args) == 1 && (args[0] == "compression" || args[0] == "compress" || args[0] == "compaction" || args[0] == "compact" || args[0] == "summarization" || args[0] == "summary") {
		fmt.Println(RenderPromptCompressionCLIReport(cfg, repoContext))
		return nil
	}
	if len(args) == 1 && (args[0] == "context" || args[0] == "manifest" || args[0] == "snapshot" || args[0] == "inputs") {
		fmt.Println(RenderPromptContextCLIReport(cfg, repoContext))
		return nil
	}
	fmt.Println(RenderPromptCLIReport(cfg, repoContext))
	return nil
}

func isPromptCLISubcommand(arg string) bool {
	switch arg {
	case "list", "risk", "risk-audit", "pack", "pack-plan", "packing", "context-pack", "context", "manifest", "snapshot", "inputs", "cache", "cache-plan", "cache-status", "cache-readiness", "compression", "compress", "compaction", "compact", "summarization", "summary":
		return true
	default:
		return false
	}
}

func runSessionCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw session catalog | gitclaw session list --backup <issue.json> | gitclaw session provenance --backup <issue.json> | gitclaw session tools --backup <issue.json> | gitclaw session skills --backup <issue.json> | gitclaw session usage --backup <issue.json> | gitclaw session trajectory --backup <issue.json> | gitclaw session compaction --backup <issue.json> | gitclaw session resume --backup <issue.json> | gitclaw session status --backup <issue.json> | gitclaw session stats --backup <issue.json> | gitclaw session coverage --backup <issue.json> | gitclaw session risk --backup <issue.json> | gitclaw session search <query> --backup <issue.json>")
	}
	switch args[0] {
	case "catalog", "commands", "capabilities":
		return runSessionCatalogCommand(args[1:])
	case "list":
		return runSessionListCommand(args[1:])
	case "provenance", "prompt-provenance", "evidence", "lineage":
		return runSessionProvenanceCommand(args[1:])
	case "tools", "tool", "tool-ledger", "tool-use", "tool-usage":
		return runSessionToolsCommand(args[1:])
	case "skills", "skill", "skill-ledger", "skill-use", "skill-usage":
		return runSessionSkillsCommand(args[1:])
	case "usage", "tokens", "token-use", "token-usage":
		return runSessionUsageCommand(args[1:])
	case "trajectory", "trace", "manifest", "export-trajectory":
		return runSessionTrajectoryCommand(args[1:])
	case "compaction", "compact", "compression", "compress", "summarization":
		return runSessionCompactionCommand(args[1:])
	case "resume", "handoff", "continuation", "continue", "yield":
		return runSessionResumeCommand(args[1:])
	case "status", "readback":
		return runSessionStatusCommand(args[1:])
	case "stats", "summary":
		return runSessionStatsCommand(args[1:])
	case "coverage", "covered":
		return runSessionCoverageCommand(args[1:])
	case "risk", "risk-audit":
		return runSessionRiskCommand(args[1:])
	case "search":
		return runSessionSearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown session command %q", args[0])
	}
}

func runSessionCatalogCommand(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gitclaw session catalog")
	}
	fmt.Println(RenderSessionCatalogCLIReport())
	return nil
}

func runSessionListCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session list argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session list --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionCLIReport(backupPath, backup))
	return nil
}

func runSessionProvenanceCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session provenance argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session provenance --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionProvenanceCLIReport(backupPath, backup))
	return nil
}

func runSessionToolsCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session tools argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session tools --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionToolsCLIReport(backupPath, backup))
	return nil
}

func runSessionSkillsCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session skills argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session skills --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionSkillsCLIReport(backupPath, backup))
	return nil
}

func runSessionUsageCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session usage argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session usage --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionUsageCLIReport(backupPath, backup))
	return nil
}

func runSessionTrajectoryCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session trajectory argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session trajectory --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionTrajectoryCLIReport(backupPath, backup))
	return nil
}

func runSessionCompactionCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session compaction argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session compaction --backup <issue.json>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionCompactionCLIReport(backupPath, backup, cfg))
	return nil
}

func runSessionResumeCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session resume argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session resume --backup <issue.json>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionResumeCLIReport(backupPath, backup, cfg))
	return nil
}

func runSessionStatusCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session status argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session status --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionStatusCLIReport(backupPath, backup))
	return nil
}

func runSessionStatsCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session stats argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session stats --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionStatsCLIReport(backupPath, backup))
	return nil
}

func runSessionCoverageCommand(args []string) error {
	backupPath := ""
	req := DefaultSessionCoverageRequirements()
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		case "--min-assistant-turns":
			if i+1 >= len(args) {
				return fmt.Errorf("--min-assistant-turns requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --min-assistant-turns: %q", args[i+1])
			}
			req.MinAssistantTurns = parsed
			i++
		case "--min-prompt-provenance":
			if i+1 >= len(args) {
				return fmt.Errorf("--min-prompt-provenance requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --min-prompt-provenance: %q", args[i+1])
			}
			req.MinPromptProvenance = parsed
			i++
		case "--min-model-turns":
			if i+1 >= len(args) {
				return fmt.Errorf("--min-model-turns requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --min-model-turns: %q", args[i+1])
			}
			req.MinModelBackedTurns = parsed
			i++
		case "--require-skill":
			if i+1 >= len(args) {
				return fmt.Errorf("--require-skill requires a value")
			}
			req.RequiredSkills = append(req.RequiredSkills, args[i+1])
			i++
		case "--require-tool":
			if i+1 >= len(args) {
				return fmt.Errorf("--require-tool requires a value")
			}
			req.RequiredTools = append(req.RequiredTools, args[i+1])
			i++
		default:
			return fmt.Errorf("unknown session coverage argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session coverage --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	report := BuildSessionCoverageReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments), backup.Transcript, req)
	fmt.Println(RenderSessionCoverageReport(report))
	if !report.OK() {
		return fmt.Errorf("session coverage reported %s", report.SessionCoverageStatus)
	}
	return nil
}

func runSessionRiskCommand(args []string) error {
	backupPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown session risk argument %q", args[i])
		}
	}
	if backupPath == "" {
		return fmt.Errorf("usage: gitclaw session risk --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionRiskCLIReport(backupPath, backup))
	return nil
}

func runSessionSearchCommand(args []string) error {
	backupPath := ""
	maxResults := defaultSessionSearchMaxResults
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup":
			if i+1 >= len(args) {
				return fmt.Errorf("--backup requires a value")
			}
			backupPath = args[i+1]
			i++
		case "--max-results":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-results requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-results: %q", args[i+1])
			}
			maxResults = parsed
			i++
		default:
			queryParts = append(queryParts, args[i])
		}
	}
	query := strings.TrimSpace(strings.Join(queryParts, " "))
	if backupPath == "" || query == "" {
		return fmt.Errorf("usage: gitclaw session search <query> --backup <issue.json>")
	}
	backup, err := ReadIssueBackupFile(backupPath)
	if err != nil {
		return err
	}
	fmt.Println(RenderSessionSearchCLIReport(backupPath, backup, query, maxResults))
	return nil
}

func runToolsCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw tools catalog|snapshot|verify|risk|validate|list|exposure [risk]|defer-plan|boundary [query]|provenance [query]|toolsets [risk|provenance|info <name>]|approval-plan <name>|run-plan <name>|info <name>|search <query>")
	}
	switch args[0] {
	case "catalog", "index", "tool-catalog", "discovery", "eligible":
		return runToolsCatalogCommand(args[1:])
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return runToolsSnapshotCommand(args[1:])
	case "verify":
		return runToolsVerifyCommand(args[1:])
	case "risk", "risk-audit":
		return runToolsRiskCommand(args[1:])
	case "validate":
		return runToolsValidateCommand(args[1:])
	case "list":
		return runToolsListCommand(args[1:])
	case "exposure", "expose":
		return runToolsExposureCommand(args[1:])
	case "defer-plan", "deferral", "defer", "tool-search-plan", "progressive-disclosure":
		return runToolsDeferPlanCommand(args[1:])
	case "boundary", "prompt-boundary", "promptware":
		return runToolsBoundaryCommand(args[1:])
	case "provenance", "outputs", "trace", "lineage":
		return runToolsProvenanceCommand(args[1:])
	case "toolsets", "toolset":
		return runToolsToolsetsCommand(args[1:])
	case "approval-plan", "approval", "approve-plan", "approval-gate", "gate":
		return runToolsApprovalPlanCommand(args[1:])
	case "run-plan", "plan":
		return runToolsRunPlanCommand(args[1:])
	case "info":
		return runToolsInfoCommand(args[1:])
	case "search":
		return runToolsSearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown tools command %q", args[0])
	}
}

func runToolsCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tools catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolCatalogCLIReport(cfg, repoContext))
	return nil
}

func runToolsSnapshotCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tools snapshot argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolSnapshotCLIReport(cfg, repoContext))
	return nil
}

func runToolsDeferPlanCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tools defer-plan argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolDeferPlanCLIReport(cfg, repoContext))
	return nil
}

func runToolsBoundaryCommand(args []string) error {
	queryFlag := ""
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--query":
			if i+1 >= len(args) {
				return fmt.Errorf("--query requires a value")
			}
			queryFlag = args[i+1]
			i++
		default:
			queryParts = append(queryParts, args[i])
		}
	}
	query := strings.TrimSpace(strings.Join(queryParts, " "))
	if strings.TrimSpace(queryFlag) != "" {
		query = strings.TrimSpace(queryFlag)
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	var transcript []TranscriptMessage
	if query != "" {
		transcript = []TranscriptMessage{{Role: "user", Body: "tools boundary " + query}}
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, transcript, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolBoundaryCLIReport(repoContext))
	return nil
}

func runToolsProvenanceCommand(args []string) error {
	queryFlag := ""
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--query":
			if i+1 >= len(args) {
				return fmt.Errorf("--query requires a value")
			}
			queryFlag = args[i+1]
			i++
		default:
			queryParts = append(queryParts, args[i])
		}
	}
	query := strings.TrimSpace(strings.Join(queryParts, " "))
	if strings.TrimSpace(queryFlag) != "" {
		query = strings.TrimSpace(queryFlag)
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	var transcript []TranscriptMessage
	if query != "" {
		transcript = []TranscriptMessage{{Role: "user", Body: "tools provenance " + query}}
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, transcript, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolProvenanceCLIReport(repoContext))
	return nil
}

func runToolsExposureCommand(args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		if len(args) > 1 {
			return fmt.Errorf("unknown tools exposure list argument %q", args[1])
		}
		fmt.Println(RenderToolExposureCLIReport(repoContext))
		return nil
	}
	if args[0] == "risk" || args[0] == "risk-audit" {
		if len(args) > 1 {
			return fmt.Errorf("unknown tools exposure risk argument %q", args[1])
		}
		fmt.Println(RenderToolExposureRiskCLIReport(repoContext))
		return nil
	}
	return fmt.Errorf("usage: gitclaw tools exposure [list|risk]")
}

func runToolsToolsetsCommand(args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		if len(args) > 1 {
			return fmt.Errorf("unknown tools toolsets list argument %q", args[1])
		}
		fmt.Println(RenderToolsetsCLIReport(cfg))
		return nil
	}
	if args[0] == "risk" || args[0] == "risk-audit" {
		if len(args) > 1 {
			return fmt.Errorf("unknown tools toolsets risk argument %q", args[1])
		}
		fmt.Println(RenderToolsetsRiskCLIReport(cfg))
		return nil
	}
	if args[0] == "provenance" || args[0] == "history" || args[0] == "timeline" {
		if len(args) > 1 {
			return fmt.Errorf("unknown tools toolsets provenance argument %q", args[1])
		}
		fmt.Println(RenderToolsetsProvenanceCLIReport(cfg))
		return nil
	}
	if args[0] == "info" || args[0] == "show" {
		if len(args) != 2 {
			return fmt.Errorf("usage: gitclaw tools toolsets info <name>")
		}
		report := RenderToolsetInfoCLIReport(cfg, args[1])
		fmt.Println(report)
		if len(matchingToolsetSummaries(BuildToolsetStoreReport(cfg).Summaries, args[1])) == 0 {
			return fmt.Errorf("toolset %q not found", args[1])
		}
		return nil
	}
	return fmt.Errorf("usage: gitclaw tools toolsets [list|risk|provenance|info <name>]")
}

func runSecretsCommand(args []string) error {
	if len(args) == 0 {
		return runSecretsAuditCommand(nil)
	}
	switch args[0] {
	case "audit", "scan", "list":
		return runSecretsAuditCommand(args[1:])
	case "risk", "risk-audit":
		return runSecretsRiskCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw secrets [audit|scan|list|risk]")
	}
}

func runSecretsAuditCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown secrets audit argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	report, err := BuildSecretAuditReport(cfg.Workdir)
	if err != nil {
		return err
	}
	fmt.Println(RenderSecretsCLIReport(report))
	return nil
}

func runSecretsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown secrets risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	report, err := BuildSecretAuditReport(cfg.Workdir)
	if err != nil {
		return err
	}
	fmt.Println(RenderSecretsRiskCLIReport(report))
	return nil
}

func runToolsVerifyCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tools verify argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolVerifyReport(repoContext))
	return nil
}

func runToolsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tools risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolRiskReport(repoContext))
	return nil
}

func runToolsListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tools list argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolsCLIReport(repoContext))
	return nil
}

func runToolsRunPlanCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw tools run-plan <name>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "tools run-plan " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolRunPlanCLIReport(repoContext, args[0]))
	return nil
}

func runToolsApprovalPlanCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw tools approval-plan <name>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "tools approval-plan " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolApprovalPlanCLIReport(cfg, repoContext, args[0]))
	return nil
}

func runToolsInfoCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw tools info <name>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "tools info " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	report := RenderToolInfoCLIReport(repoContext, args[0])
	fmt.Println(report)
	if len(matchingToolContracts(toolReportContracts, args[0])) == 0 {
		return fmt.Errorf("tool %q not found", args[0])
	}
	return nil
}

func runToolsSearchCommand(args []string) error {
	maxResults := defaultToolSearchMaxResults
	queryFlag := ""
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--query":
			if i+1 >= len(args) {
				return fmt.Errorf("--query requires a value")
			}
			queryFlag = args[i+1]
			i++
		case "--max-results":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-results requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-results: %q", args[i+1])
			}
			maxResults = parsed
			i++
		default:
			queryParts = append(queryParts, args[i])
		}
	}
	query := strings.TrimSpace(strings.Join(queryParts, " "))
	if strings.TrimSpace(queryFlag) != "" {
		query = strings.TrimSpace(queryFlag)
	}
	if query == "" {
		return fmt.Errorf("usage: gitclaw tools search <query>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "tools search " + query}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolSearchReport(Event{}, repoContext, query, maxResults))
	return nil
}

func runToolsValidateCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tools validate argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolsValidationReport(repoContext))
	return nil
}

func runDoctorCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "list") {
		return fmt.Errorf("usage: gitclaw doctor [list]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderDoctorCLIReport(cfg, repoContext))
	return nil
}

func runMigrateCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw migrate [plan|risk] <source>")
	}
	if args[0] == "risk" || args[0] == "risk-audit" {
		return runMigrateRiskCommand(args[1:])
	}
	if args[0] != "plan" && args[0] != "dry-run" {
		return fmt.Errorf("usage: gitclaw migrate [plan|risk] <source>")
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: gitclaw migrate plan <source>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "migrate plan " + args[1]}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMigrationCLIReport(repoContext, args[1]))
	return nil
}

func runMigrateRiskCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw migrate risk <source>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "migrate risk " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderMigrationRiskCLIReport(repoContext, args[0]))
	return nil
}

func runSkillsCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw skills verify|risk|runtime|catalog|snapshot|eligible|validate|check|list|provenance|select-plan <name>|refresh-plan|sources [verify|lock|update-plan|provenance|risk|info <name>|search <query>]|proposals [risk]|proposal-plan <name>|install-plan <target>|upgrade-plan <target>|bundles [risk|provenance]|bundle <name>|info <name>|search <query>")
	}
	switch args[0] {
	case "verify":
		return runSkillsVerifyCommand(args[1:])
	case "risk", "risk-audit":
		return runSkillsRiskCommand(args[1:])
	case "runtime", "requirements", "metadata":
		return runSkillsRuntimeCommand(args[1:])
	case "catalog", "eligible", "eligibility", "index":
		return runSkillsCatalogCommand(args[1:])
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return runSkillsSnapshotCommand(args[1:])
	case "validate", "check":
		return runSkillsValidateCommand(args[1:])
	case "list":
		return runSkillsListCommand(args[1:])
	case "provenance", "history", "timeline":
		return runSkillsProvenanceCommand(args[1:])
	case "select-plan", "selection-plan":
		return runSkillsSelectPlanCommand(args[1:])
	case "refresh-plan", "refresh", "reload-plan":
		return runSkillsRefreshPlanCommand(args[1:])
	case "sources", "source":
		return runSkillsSourcesCommand(args[1:])
	case "proposals", "proposal-list", "workshop-list":
		return runSkillsProposalsCommand(args[1:])
	case "proposal-plan", "propose-plan", "workshop-plan":
		return runSkillsProposalPlanCommand(args[1:], "auto")
	case "proposal-create-plan", "propose-create":
		return runSkillsProposalPlanCommand(args[1:], "propose-create")
	case "proposal-update-plan", "propose-update":
		return runSkillsProposalPlanCommand(args[1:], "propose-update")
	case "install-plan", "plan":
		return runSkillsInstallPlanCommand(args[1:], "install-plan")
	case "upgrade-plan":
		return runSkillsInstallPlanCommand(args[1:], "upgrade-plan")
	case "bundles", "bundle-list":
		if len(args) > 1 && (args[1] == "risk" || args[1] == "risk-audit") {
			return runBundlesRiskCommand(args[2:])
		}
		if len(args) > 1 && (args[1] == "provenance" || args[1] == "history" || args[1] == "timeline") {
			return runBundlesProvenanceCommand(args[2:])
		}
		return runBundlesListCommand(args[1:])
	case "bundle-risk", "bundles-risk":
		return runBundlesRiskCommand(args[1:])
	case "bundle-provenance", "bundles-provenance":
		return runBundlesProvenanceCommand(args[1:])
	case "bundle", "bundle-info":
		return runBundlesInfoCommand(args[1:])
	case "info":
		return runSkillsInfoCommand(args[1:])
	case "search":
		return runSkillsSearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown skills command %q", args[0])
	}
}

func runSkillsSnapshotCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown skills snapshot argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills snapshot"}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillSnapshotCLIReport(cfg, repoContext))
	return nil
}

func runSkillsCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown skills catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillCatalogCLIReport(repoContext))
	return nil
}

func runSkillsSelectPlanCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw skills select-plan <name>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills select-plan " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillSelectPlanCLIReport(repoContext, args[0]))
	return nil
}

func runSkillsRefreshPlanCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: gitclaw skills refresh-plan")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills refresh-plan"}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillRefreshPlanCLIReport(cfg, repoContext))
	return nil
}

func runSkillsProposalsCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "risk" && args[0] != "list" && args[0] != "status" && args[0] != "quarantine") {
		return fmt.Errorf("usage: gitclaw skills proposals [risk]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillProposalsCLIReport(cfg))
	return nil
}

func runSkillsSourcesCommand(args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		if len(args) > 1 {
			return fmt.Errorf("unknown skills sources list argument %q", args[1])
		}
		fmt.Println(RenderSkillSourcesCLIReport(cfg, repoContext))
		return nil
	}
	if args[0] == "risk" || args[0] == "risk-audit" {
		if len(args) > 1 {
			return fmt.Errorf("unknown skills sources risk argument %q", args[1])
		}
		fmt.Println(RenderSkillSourcesRiskCLIReport(cfg, repoContext))
		return nil
	}
	if args[0] == "verify" || args[0] == "check" {
		if len(args) > 1 {
			return fmt.Errorf("unknown skills sources verify argument %q", args[1])
		}
		fmt.Println(RenderSkillSourcesVerifyCLIReport(cfg, repoContext))
		return nil
	}
	if args[0] == "lock" || args[0] == "lockfile" {
		if len(args) > 1 {
			return fmt.Errorf("unknown skills sources lock argument %q", args[1])
		}
		fmt.Println(RenderSkillSourcesLockCLIReport(cfg, repoContext))
		return nil
	}
	if args[0] == "update-plan" || args[0] == "updates" || args[0] == "sync-plan" {
		if len(args) > 1 {
			return fmt.Errorf("unknown skills sources update-plan argument %q", args[1])
		}
		fmt.Println(RenderSkillSourcesUpdatePlanCLIReport(cfg, repoContext))
		return nil
	}
	if args[0] == "search" || args[0] == "find" {
		if len(args) < 2 {
			return fmt.Errorf("usage: gitclaw skills sources search <query>")
		}
		fmt.Println(RenderSkillSourcesSearchCLIReport(cfg, repoContext, strings.Join(args[1:], " ")))
		return nil
	}
	if args[0] == "provenance" || args[0] == "history" || args[0] == "timeline" {
		if len(args) > 1 {
			return fmt.Errorf("unknown skills sources provenance argument %q", args[1])
		}
		fmt.Println(RenderSkillSourceProvenanceCLIReport(cfg, repoContext))
		return nil
	}
	if args[0] == "info" || args[0] == "show" {
		if len(args) != 2 {
			return fmt.Errorf("usage: gitclaw skills sources info <name>")
		}
		report := RenderSkillSourceInfoCLIReport(cfg, repoContext, args[1])
		fmt.Println(report)
		if len(matchingSkillSourceCards(BuildSkillSourceReport(cfg, repoContext).Cards, args[1])) == 0 {
			return fmt.Errorf("skill source %q not found", args[1])
		}
		return nil
	}
	return fmt.Errorf("usage: gitclaw skills sources [list|verify|lock|update-plan|provenance|risk|info <name>|search <query>]")
}

func runSkillsProposalPlanCommand(args []string, requestedAction string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw skills proposal-plan <name>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills proposal-plan " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillProposalPlanCLIReport(repoContext, requestedAction, args[0]))
	return nil
}

func runBundlesCommand(args []string) error {
	if len(args) == 0 {
		return runBundlesListCommand(nil)
	}
	switch args[0] {
	case "catalog", "index", "eligible", "eligibility":
		return runBundlesCatalogCommand(args[1:])
	case "list":
		return runBundlesListCommand(args[1:])
	case "risk", "risk-audit":
		return runBundlesRiskCommand(args[1:])
	case "provenance", "history", "timeline":
		return runBundlesProvenanceCommand(args[1:])
	case "info", "show":
		return runBundlesInfoCommand(args[1:])
	case "search":
		return runBundlesSearchCommand(args[1:])
	default:
		return fmt.Errorf("usage: gitclaw bundles [catalog|list|risk|provenance|info <name>|search <query>]")
	}
}

func runSkillsVerifyCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown skills verify argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillsVerifyReport(repoContext))
	return nil
}

func runSkillsRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown skills risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillsRiskReport(repoContext))
	return nil
}

func runSkillsRuntimeCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown skills runtime argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillRuntimeCLIReport(repoContext))
	return nil
}

func runSkillsListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown skills list argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillsCLIReport(repoContext))
	return nil
}

func runSkillsProvenanceCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown skills provenance argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillProvenanceCLIReport(cfg, repoContext))
	return nil
}

func runSkillsInstallPlanCommand(args []string, operation string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw skills %s <target>", normalizeSkillInstallOperation(operation))
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills " + normalizeSkillInstallOperation(operation) + " " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillInstallPlanCLIReport(repoContext, operation, args[0]))
	return nil
}

func runBundlesListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown bundles list argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillBundlesCLIReport(repoContext))
	return nil
}

func runBundlesCatalogCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown bundles catalog argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillBundleCatalogCLIReport(repoContext))
	return nil
}

func runBundlesRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown bundles risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillBundlesRiskCLIReport(repoContext))
	return nil
}

func runBundlesProvenanceCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown bundles provenance argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillBundleProvenanceCLIReport(cfg, repoContext))
	return nil
}

func runBundlesInfoCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw bundles info <name>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "bundles info " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	report := RenderSkillBundleInfoCLIReport(repoContext, args[0])
	fmt.Println(report)
	if len(matchingSkillBundleSummaries(repoContext.SkillBundles, args[0])) == 0 {
		return fmt.Errorf("skill bundle %q not found", args[0])
	}
	return nil
}

func runBundlesSearchCommand(args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("usage: gitclaw bundles search <query>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "bundles search " + query}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillBundleSearchCLIReport(repoContext, query))
	return nil
}

func runSkillsSearchCommand(args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("usage: gitclaw skills search <query>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills search " + query}}, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillSearchCLIReport(repoContext, query))
	return nil
}

func runSkillsInfoCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw skills info <name>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills info " + args[0]}}, cfg)
	if err != nil {
		return err
	}
	report := RenderSkillInfoCLIReport(repoContext, args[0])
	fmt.Println(report)
	if len(matchingSkillSummaries(repoContext.SkillSummaries, args[0])) == 0 {
		return fmt.Errorf("skill %q not found", args[0])
	}
	return nil
}

func runSkillsValidateCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown skills validate argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillsValidationReport(repoContext))
	return nil
}

func runBackup(ctx context.Context, args []string) error {
	if len(args) > 0 && (args[0] == "catalog" || args[0] == "commands" || args[0] == "capabilities") {
		return runBackupCatalog(args[1:])
	}
	if len(args) > 0 && args[0] == "index" {
		return runBackupIndex(args[1:])
	}
	if len(args) > 0 && args[0] == "verify" {
		return runBackupVerify(args[1:])
	}
	if len(args) > 0 && (args[0] == "coverage" || args[0] == "covered") {
		return runBackupCoverage(args[1:])
	}
	if len(args) > 0 && (args[0] == "drill" || args[0] == "restore-drill") {
		return runBackupDrill(args[1:])
	}
	if len(args) > 0 && (args[0] == "risk" || args[0] == "risk-audit") {
		return runBackupRisk(args[1:])
	}
	if len(args) > 0 && (args[0] == "provenance" || args[0] == "git-provenance") {
		return runBackupProvenance(args[1:])
	}
	if len(args) > 0 && args[0] == "export-jsonl" {
		return runBackupExportJSONL(args[1:])
	}
	if len(args) > 0 && args[0] == "restore-plan" {
		return runBackupRestorePlan(args[1:])
	}
	if len(args) > 0 && args[0] == "manifest" {
		return runBackupManifest(args[1:])
	}
	if len(args) > 0 && args[0] == "list" {
		return runBackupList(args[1:])
	}
	if len(args) > 0 && (args[0] == "timeline" || args[0] == "history") {
		return runBackupTimeline(args[1:])
	}
	if len(args) > 0 && (args[0] == "continuity" || args[0] == "gaps" || args[0] == "gap") {
		return runBackupContinuity(args[1:])
	}
	if len(args) > 0 && args[0] == "info" {
		return runBackupInfo(args[1:])
	}
	if len(args) > 0 && (args[0] == "freshness" || args[0] == "fresh" || args[0] == "staleness") {
		return runBackupFreshness(args[1:])
	}
	if len(args) > 0 && args[0] == "stats" {
		return runBackupStats(args[1:])
	}
	if len(args) > 0 && (args[0] == "snapshot" || args[0] == "snapshots" || args[0] == "fingerprint" || args[0] == "fingerprints" || args[0] == "lock" || args[0] == "lockfile") {
		return runBackupSnapshot(args[1:])
	}
	if len(args) > 0 && args[0] == "retention-plan" {
		return runBackupRetentionPlan(args[1:])
	}
	if len(args) > 0 && args[0] == "search" {
		return runBackupSearch(args[1:])
	}
	outDir := filepathArg(args, "--out")
	filteredArgs := removeFlagWithValue(args, "--out")
	ev, _, err := loadEventAndConfig(filteredArgs)
	if err != nil {
		return err
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	github := NewRESTGitHubClient(token)
	ev, err = ResolveDispatchIssue(ctx, ev, github)
	if err != nil {
		return err
	}
	path, err := BackupIssue(ctx, ev, github, outDir, time.Now())
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

func runBackupCatalog(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown backup catalog argument %q", args[i])
		}
	}
	fmt.Println(RenderBackupCatalogCLIReport(root, repo))
	return nil
}

func runBackupVerify(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown backup verify argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	result, err := VerifyBackupTree(root, repo)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupVerifyReport(result))
	if !result.OK() {
		return fmt.Errorf("backup verification failed")
	}
	return nil
}

func runBackupCoverage(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	issueNumber := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--issue":
			if i+1 >= len(args) {
				return fmt.Errorf("--issue requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --issue: %q", args[i+1])
			}
			issueNumber = parsed
			i++
		default:
			if issueNumber == 0 {
				parsed, err := strconv.Atoi(strings.TrimPrefix(args[i], "#"))
				if err == nil && parsed > 0 {
					issueNumber = parsed
					continue
				}
			}
			return fmt.Errorf("unknown backup coverage argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	if issueNumber <= 0 {
		return fmt.Errorf("missing --issue")
	}
	coverage, err := BuildBackupCoverage(root, repo, issueNumber)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupCoverage(coverage))
	if !coverage.OK() {
		return fmt.Errorf("backup coverage reported %s", coverage.BackupCoverageStatus)
	}
	return nil
}

func runBackupDrill(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	targetRepo := ""
	issueNumber := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--target-repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--target-repo requires a value")
			}
			targetRepo = args[i+1]
			i++
		case "--issue":
			if i+1 >= len(args) {
				return fmt.Errorf("--issue requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --issue: %q", args[i+1])
			}
			issueNumber = parsed
			i++
		default:
			if issueNumber == 0 {
				parsed, err := strconv.Atoi(strings.TrimPrefix(args[i], "#"))
				if err == nil && parsed > 0 {
					issueNumber = parsed
					continue
				}
			}
			return fmt.Errorf("unknown backup drill argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	if issueNumber <= 0 {
		return fmt.Errorf("missing --issue")
	}
	drill, err := BuildBackupDrill(root, repo, issueNumber, targetRepo)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupDrill(drill))
	if !drill.OK() {
		return fmt.Errorf("backup drill reported %s", drill.BackupDrillStatus)
	}
	return nil
}

func runBackupRisk(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown backup risk argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	report, err := BuildBackupRisk(root, repo)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupRiskReport(report))
	return nil
}

func runBackupProvenance(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown backup provenance argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	report, err := BuildBackupProvenance(root, repo)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupProvenance(report))
	if !report.OK() {
		return fmt.Errorf("backup provenance reported %s", report.BackupProvenanceStatus)
	}
	return nil
}

func runBackupExportJSONL(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	issueNumber := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--issue":
			if i+1 >= len(args) {
				return fmt.Errorf("--issue requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --issue: %q", args[i+1])
			}
			issueNumber = parsed
			i++
		default:
			return fmt.Errorf("unknown backup export-jsonl argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	output, err := ExportBackupJSONL(root, repo, issueNumber)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}

func runBackupRestorePlan(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	targetRepo := ""
	issueNumber := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--target-repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--target-repo requires a value")
			}
			targetRepo = args[i+1]
			i++
		case "--issue":
			if i+1 >= len(args) {
				return fmt.Errorf("--issue requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --issue: %q", args[i+1])
			}
			issueNumber = parsed
			i++
		default:
			return fmt.Errorf("unknown backup restore-plan argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	if issueNumber <= 0 {
		return fmt.Errorf("missing --issue")
	}
	plan, err := PlanBackupRestore(root, repo, issueNumber, targetRepo)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupRestorePlan(plan))
	return nil
}

func runBackupManifest(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	issueNumber := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--issue":
			if i+1 >= len(args) {
				return fmt.Errorf("--issue requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --issue: %q", args[i+1])
			}
			issueNumber = parsed
			i++
		default:
			return fmt.Errorf("unknown backup manifest argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	manifest, err := BuildBackupManifest(root, repo, issueNumber)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupManifest(manifest))
	return nil
}

func runBackupList(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	limit := 20
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--limit":
			if i+1 >= len(args) {
				return fmt.Errorf("--limit requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --limit: %q", args[i+1])
			}
			limit = parsed
			i++
		default:
			return fmt.Errorf("unknown backup list argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	list, err := BuildBackupList(root, repo, limit)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupList(list))
	if list.BackupListStatus != "ok" {
		return fmt.Errorf("backup list reported %s", list.BackupListStatus)
	}
	return nil
}

func runBackupTimeline(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	limit := 20
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--limit":
			if i+1 >= len(args) {
				return fmt.Errorf("--limit requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --limit: %q", args[i+1])
			}
			limit = parsed
			i++
		default:
			return fmt.Errorf("unknown backup timeline argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	timeline, err := BuildBackupTimeline(root, repo, limit)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupTimeline(timeline))
	if timeline.BackupTimelineStatus != "ok" {
		return fmt.Errorf("backup timeline reported %s", timeline.BackupTimelineStatus)
	}
	return nil
}

func runBackupInfo(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	issueNumber := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--issue":
			if i+1 >= len(args) {
				return fmt.Errorf("--issue requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --issue: %q", args[i+1])
			}
			issueNumber = parsed
			i++
		default:
			if issueNumber == 0 {
				parsed, err := strconv.Atoi(args[i])
				if err == nil && parsed > 0 {
					issueNumber = parsed
					continue
				}
			}
			return fmt.Errorf("unknown backup info argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	if issueNumber <= 0 {
		return fmt.Errorf("missing --issue")
	}
	info, err := BuildBackupInfo(root, repo, issueNumber)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupInfo(info))
	if info.BackupInfoStatus != "ok" {
		return fmt.Errorf("backup info reported %s", info.BackupInfoStatus)
	}
	return nil
}

func runBackupStats(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown backup stats argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	stats, err := BuildBackupStats(root, repo)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupStats(stats))
	if stats.BackupStatsStatus != "ok" {
		return fmt.Errorf("backup stats reported %s", stats.BackupStatsStatus)
	}
	return nil
}

func runBackupSnapshot(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown backup snapshot argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	snapshot, err := BuildBackupSnapshot(root, repo)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupSnapshot(snapshot))
	if snapshot.BackupSnapshotStatus != "ok" {
		return fmt.Errorf("backup snapshot reported %s", snapshot.BackupSnapshotStatus)
	}
	return nil
}

func runBackupFreshness(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	maxAge := 24 * time.Hour
	var now time.Time
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--max-age-hours":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-age-hours requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-age-hours: %q", args[i+1])
			}
			maxAge = time.Duration(parsed) * time.Hour
			i++
		case "--max-age-seconds":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-age-seconds requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-age-seconds: %q", args[i+1])
			}
			maxAge = time.Duration(parsed) * time.Second
			i++
		case "--now":
			if i+1 >= len(args) {
				return fmt.Errorf("--now requires a value")
			}
			parsed, err := time.Parse(time.RFC3339, args[i+1])
			if err != nil {
				return fmt.Errorf("invalid --now: %q", args[i+1])
			}
			now = parsed
			i++
		default:
			return fmt.Errorf("unknown backup freshness argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	freshness, err := BuildBackupFreshness(root, repo, maxAge, now)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupFreshness(freshness))
	if freshness.FreshnessGate != "pass" || freshness.BackupFreshnessStatus != "ok" {
		return fmt.Errorf("backup freshness reported %s", freshness.BackupFreshnessStatus)
	}
	return nil
}

func runBackupContinuity(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	maxGap := 168 * time.Hour
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--max-gap-hours":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-gap-hours requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-gap-hours: %q", args[i+1])
			}
			maxGap = time.Duration(parsed) * time.Hour
			i++
		case "--max-gap-seconds":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-gap-seconds requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-gap-seconds: %q", args[i+1])
			}
			maxGap = time.Duration(parsed) * time.Second
			i++
		default:
			return fmt.Errorf("unknown backup continuity argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	continuity, err := BuildBackupContinuity(root, repo, maxGap)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupContinuity(continuity))
	if continuity.ContinuityGate != "pass" || continuity.BackupContinuityStatus != "ok" {
		return fmt.Errorf("backup continuity reported %s", continuity.BackupContinuityStatus)
	}
	return nil
}

func runBackupRetentionPlan(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	keepLatest := 50
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--keep-latest":
			if i+1 >= len(args) {
				return fmt.Errorf("--keep-latest requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --keep-latest: %q", args[i+1])
			}
			keepLatest = parsed
			i++
		default:
			return fmt.Errorf("unknown backup retention-plan argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	plan, err := BuildBackupRetentionPlan(root, repo, keepLatest)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupRetentionPlan(plan))
	if plan.RetentionPlanStatus != "ok" {
		return fmt.Errorf("backup retention plan reported %s", plan.RetentionPlanStatus)
	}
	return nil
}

func runBackupSearch(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	maxResults := defaultBackupSearchMaxResults
	queryFlag := ""
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		case "--query":
			if i+1 >= len(args) {
				return fmt.Errorf("--query requires a value")
			}
			queryFlag = args[i+1]
			i++
		case "--max-results":
			if i+1 >= len(args) {
				return fmt.Errorf("--max-results requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid --max-results: %q", args[i+1])
			}
			maxResults = parsed
			i++
		default:
			queryParts = append(queryParts, args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	query := strings.TrimSpace(strings.Join(queryParts, " "))
	if strings.TrimSpace(queryFlag) != "" {
		query = strings.TrimSpace(queryFlag)
	}
	if query == "" {
		return fmt.Errorf("usage: gitclaw backup search --root .gitclaw/backups --repo <owner/repo> <query>")
	}
	report, err := BuildBackupSearch(root, repo, query, maxResults)
	if err != nil {
		return err
	}
	fmt.Println(RenderBackupSearchReport(report))
	if report.BackupVerifyStatus != "ok" {
		return fmt.Errorf("backup search verification reported %s", report.BackupVerifyStatus)
	}
	return nil
}

func runBackupIndex(args []string) error {
	root := filepath.Join(".gitclaw", "backups")
	repo := os.Getenv("GITHUB_REPOSITORY")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			root = args[i+1]
			i++
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			repo = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown backup index argument %q", args[i])
		}
	}
	if repo == "" {
		return fmt.Errorf("missing --repo or GITHUB_REPOSITORY")
	}
	path, err := WriteBackupIndex(root, repo, time.Now())
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

func runHeartbeatCommand(ctx context.Context, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "status", "list", "verify":
			return runHeartbeatStatusCommand(args[1:])
		case "risk", "risk-audit":
			return runHeartbeatRiskCommand(args[1:])
		}
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := HeartbeatOptions{
		Repo:  os.Getenv("GITHUB_REPOSITORY"),
		Label: envFirst("GITCLAW_HEARTBEAT_LABEL", cfg.HeartbeatLabel),
		Slot:  os.Getenv("GITCLAW_HEARTBEAT_SLOT"),
		Limit: 3,
	}
	if limit := os.Getenv("GITCLAW_HEARTBEAT_LIMIT"); limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil {
			return fmt.Errorf("invalid GITCLAW_HEARTBEAT_LIMIT: %w", err)
		}
		opts.Limit = parsed
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--label":
			if i+1 >= len(args) {
				return fmt.Errorf("--label requires a value")
			}
			opts.Label = args[i+1]
			i++
		case "--slot":
			if i+1 >= len(args) {
				return fmt.Errorf("--slot requires a value")
			}
			opts.Slot = args[i+1]
			i++
		case "--limit":
			if i+1 >= len(args) {
				return fmt.Errorf("--limit requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid --limit: %w", err)
			}
			opts.Limit = parsed
			i++
		default:
			return fmt.Errorf("unknown heartbeat argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	llm, err := NewLLMFromEnv(cfg)
	if err != nil {
		return err
	}
	result, err := RunHeartbeat(ctx, cfg, NewRESTGitHubClient(token), llm, opts)
	if err != nil {
		return err
	}
	fmt.Printf("heartbeat scanned=%d posted=%d skipped=%d\n", result.Scanned, result.Posted, result.Skipped)
	return nil
}

func runHeartbeatStatusCommand(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gitclaw heartbeat status")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderHeartbeatCLIReport(cfg))
	return nil
}

func runHeartbeatRiskCommand(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gitclaw heartbeat risk")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderHeartbeatRiskCLIReport(cfg))
	return nil
}

func runChannelIngestCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelIngestOptions{
		Repo:      os.Getenv("GITHUB_REPOSITORY"),
		Channel:   os.Getenv("GITCLAW_CHANNEL"),
		ThreadID:  os.Getenv("GITCLAW_CHANNEL_THREAD_ID"),
		MessageID: os.Getenv("GITCLAW_CHANNEL_MESSAGE_ID"),
		Author:    os.Getenv("GITCLAW_CHANNEL_AUTHOR"),
		Body:      os.Getenv("GITCLAW_CHANNEL_BODY"),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--thread-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--thread-id requires a value")
			}
			opts.ThreadID = args[i+1]
			i++
		case "--message-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--message-id requires a value")
			}
			opts.MessageID = args[i+1]
			i++
		case "--author":
			if i+1 >= len(args) {
				return fmt.Errorf("--author requires a value")
			}
			opts.Author = args[i+1]
			i++
		case "--body":
			if i+1 >= len(args) {
				return fmt.Errorf("--body requires a value")
			}
			opts.Body = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown channel-ingest argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelIngest(ctx, cfg, NewRESTGitHubClient(token), opts)
	if err != nil {
		return err
	}
	if err := writeChannelIngestOutputs(result); err != nil {
		return err
	}
	fmt.Printf("channel_ingest issue=%d comment=%d created=%t duplicate=%t url=%s\n", result.IssueNumber, result.CommentID, result.Created, result.Duplicate, result.IssueURL)
	return nil
}

func runChannelStateCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelStateOptions{
		Repo:       os.Getenv("GITHUB_REPOSITORY"),
		Channel:    os.Getenv("GITCLAW_CHANNEL"),
		AccountID:  os.Getenv("GITCLAW_CHANNEL_ACCOUNT_ID"),
		Offset:     os.Getenv("GITCLAW_CHANNEL_OFFSET"),
		LeaseRunID: os.Getenv("GITHUB_RUN_ID"),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--account-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--account-id requires a value")
			}
			opts.AccountID = args[i+1]
			i++
		case "--offset":
			if i+1 >= len(args) {
				return fmt.Errorf("--offset requires a value")
			}
			opts.Offset = args[i+1]
			i++
		case "--lease-run-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--lease-run-id requires a value")
			}
			opts.LeaseRunID = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown channel-state argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelState(ctx, cfg, NewRESTGitHubClient(token), opts)
	if err != nil {
		return err
	}
	if err := writeChannelStateOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"channel_state issue=%d comment=%d created=%t updated=%t duplicate=%t account_sha256_12=%s offset_sha256_12=%s url=%s\n",
		result.IssueNumber,
		result.CommentID,
		result.Created,
		result.Updated,
		result.Duplicate,
		result.AccountHash,
		result.OffsetHash,
		result.IssueURL,
	)
	return nil
}

func runChannelSendCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelSendOptions{
		Repo:      os.Getenv("GITHUB_REPOSITORY"),
		Route:     os.Getenv("GITCLAW_CHANNEL_ROUTE"),
		Channel:   os.Getenv("GITCLAW_CHANNEL"),
		ThreadID:  os.Getenv("GITCLAW_CHANNEL_THREAD_ID"),
		MessageID: os.Getenv("GITCLAW_CHANNEL_MESSAGE_ID"),
		Author:    os.Getenv("GITCLAW_CHANNEL_AUTHOR"),
		Body:      os.Getenv("GITCLAW_CHANNEL_BODY"),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--route":
			if i+1 >= len(args) {
				return fmt.Errorf("--route requires a value")
			}
			opts.Route = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--thread-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--thread-id requires a value")
			}
			opts.ThreadID = args[i+1]
			i++
		case "--message-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--message-id requires a value")
			}
			opts.MessageID = args[i+1]
			i++
		case "--author":
			if i+1 >= len(args) {
				return fmt.Errorf("--author requires a value")
			}
			opts.Author = args[i+1]
			i++
		case "--body":
			if i+1 >= len(args) {
				return fmt.Errorf("--body requires a value")
			}
			opts.Body = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown channel-send argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelSend(ctx, cfg, NewRESTGitHubClient(token), opts)
	if err != nil {
		return err
	}
	if err := writeChannelSendOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"channel_send issue=%d comment=%d created=%t duplicate=%t route_resolved=%t route_sha256_12=%s url=%s\n",
		result.IssueNumber,
		result.CommentID,
		result.Created,
		result.Duplicate,
		result.RouteName != "",
		result.RouteHash,
		result.IssueURL,
	)
	return nil
}

func runChannelReactionCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelReactionOptions{
		Repo:      os.Getenv("GITHUB_REPOSITORY"),
		Route:     os.Getenv("GITCLAW_CHANNEL_ROUTE"),
		Channel:   os.Getenv("GITCLAW_CHANNEL"),
		ThreadID:  os.Getenv("GITCLAW_CHANNEL_THREAD_ID"),
		MessageID: os.Getenv("GITCLAW_CHANNEL_MESSAGE_ID"),
		Reaction:  os.Getenv("GITCLAW_CHANNEL_REACTION"),
		Author:    os.Getenv("GITCLAW_CHANNEL_AUTHOR"),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--route":
			if i+1 >= len(args) {
				return fmt.Errorf("--route requires a value")
			}
			opts.Route = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--thread-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--thread-id requires a value")
			}
			opts.ThreadID = args[i+1]
			i++
		case "--message-id", "--target-message-id":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.MessageID = args[i+1]
			i++
		case "--reaction", "--emoji":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.Reaction = args[i+1]
			i++
		case "--author":
			if i+1 >= len(args) {
				return fmt.Errorf("--author requires a value")
			}
			opts.Author = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown channel-react argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelReaction(ctx, cfg, NewRESTGitHubClient(token), opts)
	if err != nil {
		return err
	}
	if err := writeChannelReactionOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"channel_reaction issue=%d comment=%d created=%t duplicate=%t route_resolved=%t route_sha256_12=%s reaction_sha256_12=%s url=%s\n",
		result.IssueNumber,
		result.CommentID,
		result.Created,
		result.Duplicate,
		result.RouteName != "",
		result.RouteHash,
		result.ReactionHash,
		result.IssueURL,
	)
	return nil
}

func runChannelStatusCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelStatusOptions{
		Repo:            os.Getenv("GITHUB_REPOSITORY"),
		Route:           os.Getenv("GITCLAW_CHANNEL_ROUTE"),
		Channel:         os.Getenv("GITCLAW_CHANNEL"),
		ThreadID:        os.Getenv("GITCLAW_CHANNEL_THREAD_ID"),
		TargetMessageID: os.Getenv("GITCLAW_CHANNEL_MESSAGE_ID"),
		StatusID:        os.Getenv("GITCLAW_CHANNEL_STATUS_ID"),
		State:           os.Getenv("GITCLAW_CHANNEL_STATUS_STATE"),
		Author:          os.Getenv("GITCLAW_CHANNEL_AUTHOR"),
		Body:            os.Getenv("GITCLAW_CHANNEL_BODY"),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--route":
			if i+1 >= len(args) {
				return fmt.Errorf("--route requires a value")
			}
			opts.Route = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--thread-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--thread-id requires a value")
			}
			opts.ThreadID = args[i+1]
			i++
		case "--message-id", "--target-message-id":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.TargetMessageID = args[i+1]
			i++
		case "--status-id", "--update-id":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.StatusID = args[i+1]
			i++
		case "--state", "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.State = args[i+1]
			i++
		case "--author":
			if i+1 >= len(args) {
				return fmt.Errorf("--author requires a value")
			}
			opts.Author = args[i+1]
			i++
		case "--body":
			if i+1 >= len(args) {
				return fmt.Errorf("--body requires a value")
			}
			opts.Body = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown channel-status argument %q", args[i])
		}
	}
	if opts.State == "" {
		opts.State = "working"
	}
	if opts.Body == "" {
		opts.Body = defaultChannelStatusBody(opts.State)
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelStatus(ctx, cfg, NewRESTGitHubClient(token), opts)
	if err != nil {
		return err
	}
	if err := writeChannelStatusOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"channel_status issue=%d comment=%d created=%t duplicate=%t route_resolved=%t route_sha256_12=%s status_id_sha256_12=%s status_state_sha256_12=%s url=%s\n",
		result.IssueNumber,
		result.CommentID,
		result.Created,
		result.Duplicate,
		result.RouteName != "",
		result.RouteHash,
		result.StatusIDHash,
		result.StateHash,
		result.IssueURL,
	)
	return nil
}

func runChannelEditCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelEditOptions{
		Repo:            os.Getenv("GITHUB_REPOSITORY"),
		Route:           os.Getenv("GITCLAW_CHANNEL_ROUTE"),
		Channel:         os.Getenv("GITCLAW_CHANNEL"),
		ThreadID:        os.Getenv("GITCLAW_CHANNEL_THREAD_ID"),
		TargetMessageID: os.Getenv("GITCLAW_CHANNEL_MESSAGE_ID"),
		EditID:          os.Getenv("GITCLAW_CHANNEL_EDIT_ID"),
		Author:          os.Getenv("GITCLAW_CHANNEL_AUTHOR"),
		Body:            os.Getenv("GITCLAW_CHANNEL_BODY"),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--route":
			if i+1 >= len(args) {
				return fmt.Errorf("--route requires a value")
			}
			opts.Route = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--thread-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--thread-id requires a value")
			}
			opts.ThreadID = args[i+1]
			i++
		case "--message-id", "--target-message-id":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.TargetMessageID = args[i+1]
			i++
		case "--edit-id", "--update-id":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.EditID = args[i+1]
			i++
		case "--author":
			if i+1 >= len(args) {
				return fmt.Errorf("--author requires a value")
			}
			opts.Author = args[i+1]
			i++
		case "--body":
			if i+1 >= len(args) {
				return fmt.Errorf("--body requires a value")
			}
			opts.Body = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown channel-edit argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelEdit(ctx, cfg, NewRESTGitHubClient(token), opts)
	if err != nil {
		return err
	}
	if err := writeChannelEditOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"channel_edit issue=%d comment=%d created=%t duplicate=%t route_resolved=%t route_sha256_12=%s edit_id_sha256_12=%s url=%s\n",
		result.IssueNumber,
		result.CommentID,
		result.Created,
		result.Duplicate,
		result.RouteName != "",
		result.RouteHash,
		result.EditIDHash,
		result.IssueURL,
	)
	return nil
}

func runChannelGatewayCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelGatewayOptions{
		Repo:        os.Getenv("GITHUB_REPOSITORY"),
		Channel:     os.Getenv("GITCLAW_CHANNEL"),
		AccountID:   os.Getenv("GITCLAW_CHANNEL_ACCOUNT_ID"),
		GatewaySlot: os.Getenv("GITCLAW_CHANNEL_GATEWAY_SLOT"),
		LeaseRunID:  os.Getenv("GITCLAW_CHANNEL_GATEWAY_LEASE_RUN_ID"),
		Renew:       parseBoolEnv(os.Getenv("GITCLAW_CHANNEL_GATEWAY_RENEW")),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--account-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--account-id requires a value")
			}
			opts.AccountID = args[i+1]
			i++
		case "--gateway-slot":
			if i+1 >= len(args) {
				return fmt.Errorf("--gateway-slot requires a value")
			}
			opts.GatewaySlot = args[i+1]
			i++
		case "--lease-run-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--lease-run-id requires a value")
			}
			opts.LeaseRunID = args[i+1]
			i++
		case "--renew":
			opts.Renew = true
		case "--no-renew":
			opts.Renew = false
		default:
			return fmt.Errorf("unknown channel-gateway argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelGateway(ctx, cfg, NewRESTGitHubClient(token), opts, time.Now())
	if err != nil {
		return err
	}
	if err := writeChannelGatewayOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"channel_gateway issue=%d comment=%d created=%t updated=%t duplicate=%t renew=%t gateway_slot=%s account_sha256_12=%s lease_sha256_12=%s url=%s\n",
		result.IssueNumber,
		result.CommentID,
		result.Created,
		result.Updated,
		result.Duplicate,
		result.Renew,
		result.GatewaySlot,
		result.AccountHash,
		result.LeaseHash,
		result.IssueURL,
	)
	return nil
}

func runChannelDeliveryCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelDeliveryOptions{
		Repo:              os.Getenv("GITHUB_REPOSITORY"),
		Channel:           os.Getenv("GITCLAW_CHANNEL"),
		AccountID:         os.Getenv("GITCLAW_CHANNEL_ACCOUNT_ID"),
		ExternalMessageID: os.Getenv("GITCLAW_CHANNEL_EXTERNAL_MESSAGE_ID"),
		GatewayRunID:      os.Getenv("GITCLAW_CHANNEL_GATEWAY_RUN_ID"),
	}
	if value := os.Getenv("GITCLAW_CHANNEL_ISSUE_NUMBER"); value != "" {
		parsed, err := parsePositiveInt(value, "GITCLAW_CHANNEL_ISSUE_NUMBER")
		if err != nil {
			return err
		}
		opts.IssueNumber = parsed
	}
	if value := os.Getenv("GITCLAW_CHANNEL_COMMENT_ID"); value != "" {
		parsed, err := parsePositiveInt64(value, "GITCLAW_CHANNEL_COMMENT_ID")
		if err != nil {
			return err
		}
		opts.CommentID = parsed
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--account-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--account-id requires a value")
			}
			opts.AccountID = args[i+1]
			i++
		case "--issue-number":
			if i+1 >= len(args) {
				return fmt.Errorf("--issue-number requires a value")
			}
			parsed, err := parsePositiveInt(args[i+1], "--issue-number")
			if err != nil {
				return err
			}
			opts.IssueNumber = parsed
			i++
		case "--comment-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--comment-id requires a value")
			}
			parsed, err := parsePositiveInt64(args[i+1], "--comment-id")
			if err != nil {
				return err
			}
			opts.CommentID = parsed
			i++
		case "--external-message-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--external-message-id requires a value")
			}
			opts.ExternalMessageID = args[i+1]
			i++
		case "--gateway-run-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--gateway-run-id requires a value")
			}
			opts.GatewayRunID = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown channel-delivery argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelDelivery(ctx, cfg, NewRESTGitHubClient(token), opts)
	if err != nil {
		return err
	}
	if err := writeChannelDeliveryOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"channel_delivery state_issue=%d receipt_comment=%d created_state_issue=%t delivered=%t duplicate=%t issue=%d source_comment=%d account_sha256_12=%s external_message_sha256_12=%s url=%s\n",
		result.StateIssueNumber,
		result.ReceiptCommentID,
		result.CreatedStateIssue,
		result.Delivered,
		result.Duplicate,
		result.IssueNumber,
		result.SourceCommentID,
		result.AccountHash,
		result.ExternalMessageHash,
		result.StateIssueURL,
	)
	return nil
}

func runChannelOutboxCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ChannelOutboxOptions{
		Repo:        os.Getenv("GITHUB_REPOSITORY"),
		Channel:     os.Getenv("GITCLAW_CHANNEL"),
		AccountID:   os.Getenv("GITCLAW_CHANNEL_ACCOUNT_ID"),
		OutPath:     os.Getenv("GITCLAW_CHANNEL_OUTBOX_PATH"),
		IncludeBody: parseBoolEnv(os.Getenv("GITCLAW_CHANNEL_OUTBOX_INCLUDE_BODY")),
	}
	if value := os.Getenv("GITCLAW_CHANNEL_ISSUE_NUMBER"); value != "" {
		parsed, err := parsePositiveInt(value, "GITCLAW_CHANNEL_ISSUE_NUMBER")
		if err != nil {
			return err
		}
		opts.IssueNumber = parsed
	}
	if value := os.Getenv("GITCLAW_CHANNEL_OUTBOX_LIMIT"); value != "" {
		parsed, err := parsePositiveInt(value, "GITCLAW_CHANNEL_OUTBOX_LIMIT")
		if err != nil {
			return err
		}
		opts.Limit = parsed
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--channel":
			if i+1 >= len(args) {
				return fmt.Errorf("--channel requires a value")
			}
			opts.Channel = args[i+1]
			i++
		case "--account-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--account-id requires a value")
			}
			opts.AccountID = args[i+1]
			i++
		case "--issue-number":
			if i+1 >= len(args) {
				return fmt.Errorf("--issue-number requires a value")
			}
			parsed, err := parsePositiveInt(args[i+1], "--issue-number")
			if err != nil {
				return err
			}
			opts.IssueNumber = parsed
			i++
		case "--out":
			if i+1 >= len(args) {
				return fmt.Errorf("--out requires a value")
			}
			opts.OutPath = args[i+1]
			i++
		case "--limit":
			if i+1 >= len(args) {
				return fmt.Errorf("--limit requires a value")
			}
			parsed, err := parsePositiveInt(args[i+1], "--limit")
			if err != nil {
				return err
			}
			opts.Limit = parsed
			i++
		case "--include-body":
			opts.IncludeBody = true
		case "--no-include-body":
			opts.IncludeBody = false
		default:
			return fmt.Errorf("unknown channel-outbox argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunChannelOutbox(ctx, cfg, NewRESTGitHubClient(token), opts)
	if err != nil {
		return err
	}
	if err := writeChannelOutboxOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"channel_outbox issue=%d state_issue=%d assistant_comments=%d outbound_comments=%d reaction_comments=%d status_comments=%d edit_comments=%d deliverable_comments=%d delivered=%d pending=%d returned=%d body_included=%t account_sha256_12=%s out=%s\n",
		result.IssueNumber,
		result.StateIssueNumber,
		result.SourceAssistantComments,
		result.SourceOutboundComments,
		result.SourceReactionComments,
		result.SourceStatusComments,
		result.SourceEditComments,
		result.SourceDeliverableComments,
		result.DeliveredAssistantComments,
		result.PendingMessages,
		result.MessagesReturned,
		result.BodyIncluded,
		result.AccountHash,
		inlineCodeOrNone(result.OutPath),
	)
	return nil
}

func runProactiveCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw proactive <list|schedule|chain|risk|info|enqueue|init>")
	}
	switch args[0] {
	case "list":
		return runProactiveListCommand(args[1:])
	case "schedule", "schedules", "calendar", "cron":
		return runProactiveScheduleCommand(args[1:])
	case "chain", "chains", "dependencies", "dependency", "context-from", "context":
		return runProactiveChainCommand(args[1:])
	case "risk", "risk-audit":
		return runProactiveRiskCommand(args[1:])
	case "info":
		return runProactiveInfoCommand(args[1:])
	case "enqueue":
		return runProactiveEnqueueCommand(ctx, args[1:])
	case "init":
		return runProactiveInitCommand(args[1:])
	default:
		return fmt.Errorf("unknown proactive command %q", args[0])
	}
}

func runProactiveListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown proactive list argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderProactiveCLIReport(cfg))
	return nil
}

func runProactiveScheduleCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown proactive schedule argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderProactiveScheduleCLIReport(cfg))
	return nil
}

func runProactiveChainCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown proactive chain argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderProactiveChainCLIReport(cfg))
	return nil
}

func runProactiveRiskCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown proactive risk argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderProactiveRiskCLIReport(cfg))
	return nil
}

func runProactiveInfoCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitclaw proactive info <name>")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	report := RenderProactiveInfoCLIReport(cfg, args[0])
	fmt.Println(report)
	matches := matchingProactivePrompts(inspectProactiveSurface(cfg.Workdir).Prompts, args[0])
	if len(matches) == 0 {
		return fmt.Errorf("proactive job %q not found", args[0])
	}
	if len(matches) > 1 {
		return fmt.Errorf("proactive job %q is ambiguous", args[0])
	}
	return nil
}

func runProactiveInitCommand(args []string) error {
	opts := ProactiveInitOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("--root requires a value")
			}
			opts.Root = args[i+1]
			i++
		case "--name":
			if i+1 >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			opts.Name = args[i+1]
			i++
		case "--cron":
			if i+1 >= len(args) {
				return fmt.Errorf("--cron requires a value")
			}
			opts.Cron = args[i+1]
			i++
		case "--prompt", "--prompt-file":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.PromptPath = args[i+1]
			i++
		case "--prompt-body":
			if i+1 >= len(args) {
				return fmt.Errorf("--prompt-body requires a value")
			}
			opts.PromptBody = args[i+1]
			i++
		case "--skill", "--skills":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.Skills = append(opts.Skills, args[i+1])
			i++
		case "--workflow", "--workflow-file":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.WorkflowPath = args[i+1]
			i++
		case "--force":
			opts.Force = true
		case "--dry-run":
			opts.DryRun = true
		default:
			return fmt.Errorf("unknown proactive init argument %q", args[i])
		}
	}
	result, err := RunProactiveInit(opts)
	if err != nil {
		return err
	}
	fmt.Println(RenderProactiveInitReport(result))
	return nil
}

func runProactiveEnqueueCommand(ctx context.Context, args []string) error {
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	opts := ProactiveEnqueueOptions{
		Repo:         os.Getenv("GITHUB_REPOSITORY"),
		Name:         os.Getenv("GITCLAW_PROACTIVE_NAME"),
		Slot:         os.Getenv("GITCLAW_PROACTIVE_SLOT"),
		Prompt:       os.Getenv("GITCLAW_PROACTIVE_PROMPT"),
		PromptFile:   os.Getenv("GITCLAW_PROACTIVE_PROMPT_FILE"),
		NotBefore:    os.Getenv("GITCLAW_PROACTIVE_NOT_BEFORE"),
		NotifyRoutes: splitChannelBroadcastRoutes(os.Getenv("GITCLAW_PROACTIVE_NOTIFY_ROUTES")),
	}
	opts.NotifyRoutes = append(opts.NotifyRoutes, splitChannelBroadcastRoutes(os.Getenv("GITCLAW_PROACTIVE_NOTIFY_ROUTE"))...)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i+1]
			i++
		case "--name":
			if i+1 >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			opts.Name = args[i+1]
			i++
		case "--slot":
			if i+1 >= len(args) {
				return fmt.Errorf("--slot requires a value")
			}
			opts.Slot = args[i+1]
			i++
		case "--prompt":
			if i+1 >= len(args) {
				return fmt.Errorf("--prompt requires a value")
			}
			opts.Prompt = args[i+1]
			i++
		case "--prompt-file":
			if i+1 >= len(args) {
				return fmt.Errorf("--prompt-file requires a value")
			}
			opts.PromptFile = args[i+1]
			i++
		case "--not-before", "--due":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.NotBefore = args[i+1]
			i++
		case "--notify-route", "--notify-routes", "--channel-route", "--channel-routes":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", args[i])
			}
			opts.NotifyRoutes = append(opts.NotifyRoutes, splitChannelBroadcastRoutes(args[i+1])...)
			i++
		default:
			return fmt.Errorf("unknown proactive enqueue argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	result, err := RunProactiveEnqueue(ctx, cfg, NewRESTGitHubClient(token), opts, time.Now())
	if err != nil {
		return err
	}
	if err := writeProactiveOutputs(result); err != nil {
		return err
	}
	fmt.Printf(
		"proactive_enqueue issue=%d name=%s slot=%s created=%t due=%t skipped=%t not_before=%s channel_notification_requested=%t channel_notification_routes=%d channel_notification_queued=%d channel_notification_duplicates=%d channel_notification_target_issues_created=%d channel_notification_routes_sha256_12=%s channel_notification_message_id_sha256_12=%s channel_notification_body_sha256_12=%s llm_e2e_required_after_proactive_not_before_change=true llm_e2e_required_after_proactive_channel_notify_change=true url=%s\n",
		result.IssueNumber,
		result.Name,
		result.Slot,
		result.Created,
		result.Due,
		result.Skipped,
		result.NotBefore,
		result.ChannelNotification.Requested,
		result.ChannelNotification.Routes,
		result.ChannelNotification.Queued,
		result.ChannelNotification.Duplicates,
		result.ChannelNotification.TargetIssuesCreated,
		noneIfEmpty(result.ChannelNotification.RoutesSHA),
		noneIfEmpty(result.ChannelNotification.MessageSHA),
		noneIfEmpty(result.ChannelNotification.BodySHA),
		result.IssueURL,
	)
	return nil
}

func runPreflight(ctx context.Context, args []string) error {
	ev, cfg, err := loadEventAndConfig(args)
	if err != nil {
		return err
	}
	if ev.Kind == EventWorkflowDispatch {
		token := githubTokenFromEnv()
		if token == "" {
			return fmt.Errorf("workflow_dispatch preflight requires GH_TOKEN or GITHUB_TOKEN")
		}
		ev, err = ResolveDispatchIssue(ctx, ev, NewRESTGitHubClient(token))
		if err != nil {
			return err
		}
	}
	decision := Preflight(ev, cfg)
	if outputPath := os.Getenv("GITHUB_OUTPUT"); outputPath != "" {
		file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
		}
		defer file.Close()
		fmt.Fprintf(file, "allowed=%t\n", decision.Allowed)
		fmt.Fprintf(file, "code=%s\n", decision.Code)
		fmt.Fprintf(file, "reason=%s\n", decision.Reason)
	}
	fmt.Printf("allowed=%t code=%s reason=%s\n", decision.Allowed, decision.Code, decision.Reason)
	return nil
}

func runHandle(ctx context.Context, args []string) error {
	ev, cfg, err := loadEventAndConfig(args)
	if err != nil {
		return err
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	if err := validateRepoName(ev.Repo); err != nil {
		return err
	}
	github := NewRESTGitHubClient(token)
	ev, err = ResolveDispatchIssue(ctx, ev, github)
	if err != nil {
		return err
	}
	decision := Preflight(ev, cfg)
	if !decision.Allowed {
		return fmt.Errorf("%s: %s", decision.Code, decision.Reason)
	}

	llm, err := NewLLMFromEnv(cfg)
	if err != nil {
		return err
	}
	return Handle(ctx, ev, cfg, github, llm)
}

func loadEventAndConfig(args []string) (Event, Config, error) {
	eventPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--event":
			if i+1 >= len(args) {
				return Event{}, Config{}, fmt.Errorf("--event requires a path")
			}
			eventPath = args[i+1]
			i++
		default:
			return Event{}, Config{}, fmt.Errorf("unknown handle argument %q", args[i])
		}
	}
	if eventPath == "" {
		eventPath = os.Getenv("GITHUB_EVENT_PATH")
	}
	if eventPath == "" {
		return Event{}, Config{}, fmt.Errorf("missing --event or GITHUB_EVENT_PATH")
	}
	eventName := os.Getenv("GITHUB_EVENT_NAME")
	if eventName == "" {
		return Event{}, Config{}, fmt.Errorf("missing GITHUB_EVENT_NAME")
	}
	payload, err := os.ReadFile(eventPath)
	if err != nil {
		return Event{}, Config{}, fmt.Errorf("read event file: %w", err)
	}
	ev, err := ParseEvent(eventName, payload)
	if err != nil {
		return Event{}, Config{}, err
	}
	if ev.SHA == "" {
		ev.SHA = os.Getenv("GITHUB_SHA")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return Event{}, Config{}, err
	}
	return ev, cfg, nil
}

func filepathArg(args []string, name string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func removeFlagWithValue(args []string, name string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == name && i+1 < len(args) {
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func githubTokenFromEnv() string {
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GITHUB_TOKEN")
}
