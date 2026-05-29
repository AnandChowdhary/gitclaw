package gitclaw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	case "proactive":
		return runProactiveCommand(ctx, args[1:])
	case "skills":
		return runSkillsCommand(args[1:])
	case "soul":
		return runSoulCommand(args[1:])
	case "doctor":
		return runDoctorCommand(args[1:])
	case "version":
		fmt.Println("gitclaw dev")
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runSoulCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw soul validate")
	}
	switch args[0] {
	case "validate":
		return runSoulValidateCommand(args[1:])
	default:
		return fmt.Errorf("unknown soul command %q", args[0])
	}
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

func runDoctorCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown doctor argument %q", args[0])
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		return err
	}
	PrintDoctorReport(cfg)
	return nil
}

func runSkillsCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitclaw skills validate")
	}
	switch args[0] {
	case "validate":
		return runSkillsValidateCommand(args[1:])
	default:
		return fmt.Errorf("unknown skills command %q", args[0])
	}
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
		return fmt.Errorf("usage: gitclaw proactive <enqueue|init>")
	}
	switch args[0] {
	case "enqueue":
		return runProactiveEnqueueCommand(ctx, args[1:])
	case "init":
		return runProactiveInitCommand(args[1:])
	default:
		return fmt.Errorf("unknown proactive command %q", args[0])
	}
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
