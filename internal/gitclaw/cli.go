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
	case "channel-ingest":
		return runChannelIngestCommand(ctx, args[1:])
	case "channels", "channel":
		return runChannelsCommand(args[1:])
	case "proactive":
		return runProactiveCommand(ctx, args[1:])
	case "skills":
		return runSkillsCommand(args[1:])
	case "memory":
		return runMemoryCommand(args[1:])
	case "soul":
		return runSoulCommand(args[1:])
	case "tools":
		return runToolsCommand(args[1:])
	case "models", "model":
		return runModelsCommand(args[1:])
	case "config", "configuration":
		return runConfigCommand(args[1:])
	case "policy":
		return runPolicyCommand(args[1:])
	case "context":
		return runContextCommand(args[1:])
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

func runMemoryCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw memory validate|list|search <query>")
	}
	switch args[0] {
	case "validate":
		return runMemoryValidateCommand(args[1:])
	case "list":
		return runMemoryListCommand(args[1:])
	case "search":
		return runMemorySearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown memory command %q", args[0])
	}
}

func runMemoryListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown memory list argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryCLIReport(cfg, repoContext))
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
	repoContext, err := LoadRepoContext(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "memory search " + query}})
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
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderMemoryValidationReport(Event{}, cfg, repoContext))
	return nil
}

func runSoulCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw soul verify|validate|list|search <query>")
	}
	switch args[0] {
	case "verify":
		return runSoulVerifyCommand(args[1:])
	case "validate":
		return runSoulValidateCommand(args[1:])
	case "list":
		return runSoulListCommand(args[1:])
	case "search":
		return runSoulSearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown soul command %q", args[0])
	}
}

func runSoulVerifyCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown soul verify argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulVerifyReport(repoContext))
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
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderSoulCLIReport(repoContext))
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
	repoContext, err := LoadRepoContext(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "soul search " + query}})
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
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
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
	if len(args) > 1 || (len(args) == 1 && args[0] != "list") {
		return fmt.Errorf("usage: gitclaw channels [list]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderChannelCLIReport(cfg))
	return nil
}

func runModelsCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "list") {
		return fmt.Errorf("usage: gitclaw models [list]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderModelCLIReport(cfg))
	return nil
}

func runConfigCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "list") {
		return fmt.Errorf("usage: gitclaw config [list]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	fmt.Println(RenderConfigCLIReport(cfg))
	return nil
}

func runPolicyCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "list") {
		return fmt.Errorf("usage: gitclaw policy [list]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderPolicyCLIReport(cfg, repoContext))
	return nil
}

func runContextCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "list") {
		return fmt.Errorf("usage: gitclaw context [list]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderContextCLIReport(cfg, repoContext))
	return nil
}

func runPromptCommand(args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "list") {
		return fmt.Errorf("usage: gitclaw prompt [list]")
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderPromptCLIReport(cfg, repoContext))
	return nil
}

func runSessionCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw session list --backup <issue.json> | gitclaw session search <query> --backup <issue.json>")
	}
	switch args[0] {
	case "list":
		return runSessionListCommand(args[1:])
	case "search":
		return runSessionSearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown session command %q", args[0])
	}
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
		return fmt.Errorf("usage: gitclaw tools validate|list|search <query>")
	}
	switch args[0] {
	case "validate":
		return runToolsValidateCommand(args[1:])
	case "list":
		return runToolsListCommand(args[1:])
	case "search":
		return runToolsSearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown tools command %q", args[0])
	}
}

func runToolsListCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown tools list argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderToolsCLIReport(repoContext))
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
	repoContext, err := LoadRepoContext(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "tools search " + query}})
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
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
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
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderDoctorCLIReport(cfg, repoContext))
	return nil
}

func runSkillsCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw skills verify|validate|check|list|info <name>|search <query>")
	}
	switch args[0] {
	case "verify":
		return runSkillsVerifyCommand(args[1:])
	case "validate", "check":
		return runSkillsValidateCommand(args[1:])
	case "list":
		return runSkillsListCommand(args[1:])
	case "info":
		return runSkillsInfoCommand(args[1:])
	case "search":
		return runSkillsSearchCommand(args[1:])
	default:
		return fmt.Errorf("unknown skills command %q", args[0])
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
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillsVerifyReport(repoContext))
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
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillsCLIReport(repoContext))
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
	repoContext, err := LoadRepoContext(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills search " + query}})
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
	repoContext, err := LoadRepoContext(cfg.Workdir, []TranscriptMessage{{Role: "user", Body: "skills info " + args[0]}})
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
	repoContext, err := LoadRepoContext(cfg.Workdir, nil)
	if err != nil {
		return err
	}
	fmt.Println(RenderSkillsValidationReport(repoContext))
	return nil
}

func runBackup(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "index" {
		return runBackupIndex(args[1:])
	}
	if len(args) > 0 && args[0] == "verify" {
		return runBackupVerify(args[1:])
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
	if len(args) > 0 && args[0] == "stats" {
		return runBackupStats(args[1:])
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

func runProactiveCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw proactive <list|enqueue|init>")
	}
	switch args[0] {
	case "list":
		return runProactiveListCommand(args[1:])
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
		Repo:       os.Getenv("GITHUB_REPOSITORY"),
		Name:       os.Getenv("GITCLAW_PROACTIVE_NAME"),
		Slot:       os.Getenv("GITCLAW_PROACTIVE_SLOT"),
		Prompt:     os.Getenv("GITCLAW_PROACTIVE_PROMPT"),
		PromptFile: os.Getenv("GITCLAW_PROACTIVE_PROMPT_FILE"),
	}
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
		default:
			return fmt.Errorf("unknown proactive enqueue argument %q", args[i])
		}
	}
	token := githubTokenFromEnv()
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	result, err := RunProactiveEnqueue(ctx, cfg, NewRESTGitHubClient(token), opts, time.Now())
	if err != nil {
		return err
	}
	if err := writeProactiveOutputs(result); err != nil {
		return err
	}
	fmt.Printf("proactive_enqueue issue=%d name=%s slot=%s created=%t url=%s\n", result.IssueNumber, result.Name, result.Slot, result.Created, result.IssueURL)
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
