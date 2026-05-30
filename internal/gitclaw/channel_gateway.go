package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ChannelGatewayOptions struct {
	Repo        string
	Channel     string
	AccountID   string
	GatewaySlot string
	LeaseRunID  string
	Renew       bool
}

type ChannelGatewayResult struct {
	IssueNumber int
	IssueURL    string
	CommentID   int64
	Created     bool
	Updated     bool
	Duplicate   bool
	Renew       bool
	GatewaySlot string
	AccountHash string
	LeaseHash   string
}

func RunChannelGateway(ctx context.Context, cfg Config, github ChannelGatewayGitHubClient, opts ChannelGatewayOptions, now time.Time) (ChannelGatewayResult, error) {
	opts = normalizeChannelGatewayOptions(opts, now)
	if err := validateChannelGatewayOptions(opts); err != nil {
		return ChannelGatewayResult{}, err
	}
	leaseOffset := channelGatewayLeaseOffset(opts)
	state, err := RunChannelState(ctx, cfg, github, ChannelStateOptions{
		Repo:       opts.Repo,
		Channel:    opts.Channel,
		AccountID:  opts.AccountID,
		Offset:     leaseOffset,
		LeaseRunID: opts.LeaseRunID,
	})
	if err != nil {
		return ChannelGatewayResult{}, err
	}
	return ChannelGatewayResult{
		IssueNumber: state.IssueNumber,
		IssueURL:    state.IssueURL,
		CommentID:   state.CommentID,
		Created:     state.Created,
		Updated:     state.Updated,
		Duplicate:   state.Duplicate,
		Renew:       opts.Renew,
		GatewaySlot: opts.GatewaySlot,
		AccountHash: state.AccountHash,
		LeaseHash:   state.OffsetHash,
	}, nil
}

func normalizeChannelGatewayOptions(opts ChannelGatewayOptions, now time.Time) ChannelGatewayOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.AccountID = strings.TrimSpace(opts.AccountID)
	opts.GatewaySlot = strings.TrimSpace(opts.GatewaySlot)
	opts.LeaseRunID = strings.TrimSpace(opts.LeaseRunID)
	if opts.GatewaySlot == "" {
		if now.IsZero() {
			now = time.Now().UTC()
		}
		opts.GatewaySlot = now.UTC().Format("20060102T150405Z")
	}
	if opts.LeaseRunID == "" {
		opts.LeaseRunID = opts.GatewaySlot
	}
	return opts
}

func validateChannelGatewayOptions(opts ChannelGatewayOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.AccountID == "" {
		return fmt.Errorf("missing channel account id")
	}
	if opts.GatewaySlot == "" {
		return fmt.Errorf("missing channel gateway slot")
	}
	if opts.LeaseRunID == "" {
		return fmt.Errorf("missing channel gateway lease run id")
	}
	return nil
}

func channelGatewayLeaseOffset(opts ChannelGatewayOptions) string {
	return strings.Join([]string{
		"gateway-lease",
		"channel=" + opts.Channel,
		"account_id=" + opts.AccountID,
		"slot=" + opts.GatewaySlot,
		"run_id=" + opts.LeaseRunID,
	}, "|")
}

func parseBoolEnv(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}

func writeChannelGatewayOutputs(result ChannelGatewayResult) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil
	}
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	defer file.Close()
	fmt.Fprintf(file, "issue_number=%d\n", result.IssueNumber)
	fmt.Fprintf(file, "issue_url=%s\n", result.IssueURL)
	fmt.Fprintf(file, "comment_id=%d\n", result.CommentID)
	fmt.Fprintf(file, "created=%t\n", result.Created)
	fmt.Fprintf(file, "updated=%t\n", result.Updated)
	fmt.Fprintf(file, "duplicate=%t\n", result.Duplicate)
	fmt.Fprintf(file, "renew=%t\n", result.Renew)
	fmt.Fprintf(file, "gateway_slot=%s\n", result.GatewaySlot)
	fmt.Fprintf(file, "account_sha256_12=%s\n", result.AccountHash)
	fmt.Fprintf(file, "lease_sha256_12=%s\n", result.LeaseHash)
	return nil
}
