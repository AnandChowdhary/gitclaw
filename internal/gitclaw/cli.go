package gitclaw

import (
	"context"
	"fmt"
	"os"
)

func RunCLI(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gitclaw handle --event <path>")
	}
	switch args[0] {
	case "preflight":
		return runPreflight(args[1:])
	case "handle":
		return runHandle(ctx, args[1:])
	case "version":
		fmt.Println("gitclaw dev")
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runPreflight(args []string) error {
	ev, cfg, err := loadEventAndConfig(args)
	if err != nil {
		return err
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
	decision := Preflight(ev, cfg)
	if !decision.Allowed {
		return fmt.Errorf("%s: %s", decision.Code, decision.Reason)
	}

	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("missing GH_TOKEN or GITHUB_TOKEN")
	}
	if err := validateRepoName(ev.Repo); err != nil {
		return err
	}

	llm, err := NewLLMFromEnv(cfg)
	if err != nil {
		return err
	}
	return Handle(ctx, ev, cfg, NewRESTGitHubClient(token), llm)
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
