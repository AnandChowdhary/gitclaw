package gitclaw

import (
	"context"
	"fmt"
	"os"
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
	case "version":
		fmt.Println("gitclaw dev")
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runBackup(ctx context.Context, args []string) error {
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

func runHeartbeatCommand(ctx context.Context, args []string) error {
	cfg := DefaultConfig()
	if workdir := os.Getenv("GITCLAW_WORKDIR"); workdir != "" {
		cfg.Workdir = workdir
	}
	if model := os.Getenv("GITCLAW_MODEL"); model != "" {
		cfg.Model = model
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
	cfg := DefaultConfig()
	if model := os.Getenv("GITCLAW_MODEL"); model != "" {
		cfg.Model = model
	}
	if workdir := os.Getenv("GITCLAW_WORKDIR"); workdir != "" {
		cfg.Workdir = workdir
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
