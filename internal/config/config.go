// Package config loads runtime settings from environment variables.
package config

import (
	"os"
	"strings"
)

// defaultSlackSourceLabel is appended to Web API ("post as user") messages so they
// remain distinguishable from messages the user posts by hand: a user token makes
// chat.postMessage post under the user's own name/avatar, with none of the "APP"
// badge or bot identity that would otherwise set MCP-originated posts apart.
const defaultSlackSourceLabel = "ap-mcp-slack (MCP) 経由"

// Config holds application settings.
type Config struct {
	SlackWebhookURL  string
	SlackToken       string
	SlackChannelID   string
	SlackSourceLabel string
}

// Load reads environment variables.
func Load() (*Config, error) {
	webhookURL := strings.TrimSpace(os.Getenv("MCP_SLACK_WEBHOOK_URL"))
	slackToken := firstNonEmptyEnv("MCP_SLACK_USER_TOKEN", "MCP_SLACK_TOKEN", "MCP_SLACK_BOT_TOKEN")
	slackChannelID := strings.TrimSpace(os.Getenv("MCP_SLACK_CHANNEL_ID"))
	slackSourceLabel := strings.TrimSpace(os.Getenv("MCP_SLACK_SOURCE_LABEL"))
	if slackSourceLabel == "" {
		slackSourceLabel = defaultSlackSourceLabel
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
