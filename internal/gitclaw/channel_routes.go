package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const channelRoutesPath = ".gitclaw/channels/routes.yaml"

type ChannelRoute struct {
	Name             string `yaml:"name"`
	Channel          string `yaml:"channel"`
	ThreadID         string `yaml:"thread_id"`
	ThreadIDTemplate string `yaml:"thread_id_template"`
	Author           string `yaml:"author"`
	AccountID        string `yaml:"account_id"`
}

type channelRoutesFile struct {
	Routes []ChannelRoute `yaml:"routes"`
}

func LoadChannelRoutes(workdir string) ([]ChannelRoute, error) {
	if workdir == "" {
		workdir = "."
	}
	path := filepath.Join(workdir, filepath.FromSlash(channelRoutesPath))
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", channelRoutesPath, err)
	}
	var file channelRoutesFile
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("parse %s: %w", channelRoutesPath, err)
	}
	routes := make([]ChannelRoute, 0, len(file.Routes))
	seen := map[string]bool{}
	for i, route := range file.Routes {
		normalized, err := normalizeChannelRoute(route, i)
		if err != nil {
			return nil, err
		}
		if seen[normalized.Name] {
			return nil, fmt.Errorf("%s route %q is duplicated", channelRoutesPath, normalized.Name)
		}
		seen[normalized.Name] = true
		routes = append(routes, normalized)
	}
	return routes, nil
}

func normalizeChannelRoute(route ChannelRoute, index int) (ChannelRoute, error) {
	route.Name = cleanChannelRouteName(route.Name)
	route.Channel = strings.ToLower(strings.TrimSpace(route.Channel))
	route.ThreadID = strings.TrimSpace(route.ThreadID)
	route.ThreadIDTemplate = strings.TrimSpace(route.ThreadIDTemplate)
	route.Author = strings.TrimSpace(route.Author)
	route.AccountID = strings.TrimSpace(route.AccountID)
	if route.Name == "" {
		return ChannelRoute{}, fmt.Errorf("%s route %d missing name", channelRoutesPath, index+1)
	}
	if route.Channel == "" {
		return ChannelRoute{}, fmt.Errorf("%s route %q missing channel", channelRoutesPath, route.Name)
	}
	if route.ThreadID == "" && route.ThreadIDTemplate == "" {
		return ChannelRoute{}, fmt.Errorf("%s route %q missing thread_id or thread_id_template", channelRoutesPath, route.Name)
	}
	if route.ThreadID != "" && route.ThreadIDTemplate != "" {
		return ChannelRoute{}, fmt.Errorf("%s route %q must use either thread_id or thread_id_template", channelRoutesPath, route.Name)
	}
	return route, nil
}

func applyChannelSendRoute(cfg Config, opts ChannelSendOptions) (ChannelSendOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routes, err := LoadChannelRoutes(cfg.Workdir)
	if err != nil {
		return opts, err
	}
	route, ok := findChannelRoute(routes, opts.Route)
	if !ok {
		return opts, fmt.Errorf("channel route %q not found in %s", opts.Route, channelRoutesPath)
	}
	threadID, err := resolveChannelRouteThreadID(route, opts)
	if err != nil {
		return opts, err
	}
	if opts.Channel != "" && opts.Channel != route.Channel {
		return opts, fmt.Errorf("channel route %q resolves channel %q but --channel was %q", route.Name, route.Channel, opts.Channel)
	}
	if opts.ThreadID != "" && opts.ThreadID != threadID {
		return opts, fmt.Errorf("channel route %q resolves thread id hash %s but --thread-id hash was %s", route.Name, shortDocumentHash(threadID), shortDocumentHash(opts.ThreadID))
	}
	opts.Route = route.Name
	opts.Channel = route.Channel
	opts.ThreadID = threadID
	if opts.Author == "" && route.Author != "" {
		opts.Author = route.Author
	}
	return opts, nil
}

func findChannelRoute(routes []ChannelRoute, name string) (ChannelRoute, bool) {
	name = cleanChannelRouteName(name)
	for _, route := range routes {
		if route.Name == name {
			return route, true
		}
	}
	return ChannelRoute{}, false
}

func resolveChannelRouteThreadID(route ChannelRoute, opts ChannelSendOptions) (string, error) {
	if route.ThreadID != "" {
		return route.ThreadID, nil
	}
	if opts.MessageID == "" {
		return "", fmt.Errorf("channel route %q uses thread_id_template and requires --message-id", route.Name)
	}
	threadID := route.ThreadIDTemplate
	threadID = strings.ReplaceAll(threadID, "{message_id}", opts.MessageID)
	threadID = strings.ReplaceAll(threadID, "{route}", route.Name)
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return "", fmt.Errorf("channel route %q resolved an empty thread id", route.Name)
	}
	return threadID, nil
}

func cleanChannelRouteName(name string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(name), " \t\r\n.,:;!?`\"'"))
}
