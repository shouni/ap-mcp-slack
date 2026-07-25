// Package config loads runtime settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

// defaultSlackSourceLabel is appended to Slack posts so MCP-originated messages stay
// distinguishable from messages the user posts by hand.
const defaultSlackSourceLabel = "ap-mcp-slack (MCP) 経由"

// Config holds application settings.
type Config struct {
	SlackWebhookURL  string
	SlackToken       string
	SlackChannelID   string
	SlackSourceLabel string
}

// Load reads environment variables.
//
// At least one of the webhook URL or the Web API token must be present. Tools are
// registered per configured transport, so with neither set the server would start
// cleanly, advertise zero tools, and leave the operator to work out from an empty
// tool list that their environment never reached the process. Failing here puts the
// reason on stderr at startup instead.
func Load() (*Config, error) {
	webhookURL := strings.TrimSpace(os.Getenv("MCP_SLACK_WEBHOOK_URL"))
	slackToken := firstNonEmptyEnv("MCP_SLACK_USER_TOKEN", "MCP_SLACK_TOKEN", "MCP_SLACK_BOT_TOKEN")
	slackChannelID := strings.TrimSpace(os.Getenv("MCP_SLACK_CHANNEL_ID"))
	slackSourceLabel := strings.TrimSpace(os.Getenv("MCP_SLACK_SOURCE_LABEL"))
	if slackSourceLabel == "" {
		slackSourceLabel = defaultSlackSourceLabel
	}

	if webhookURL == "" && slackToken == "" {
		return nil, fmt.Errorf("config: set MCP_SLACK_WEBHOOK_URL or MCP_SLACK_USER_TOKEN/MCP_SLACK_TOKEN/MCP_SLACK_BOT_TOKEN")
	}

	return &Config{
		SlackWebhookURL:  webhookURL,
		SlackToken:       slackToken,
		SlackChannelID:   slackChannelID,
		SlackSourceLabel: slackSourceLabel,
	}, nil
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}
