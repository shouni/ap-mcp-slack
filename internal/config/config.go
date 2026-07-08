// Package config loads runtime settings from environment variables.
package config

import (
	"os"
	"strings"
)

// Config holds application settings.
type Config struct {
	SlackWebhookURL string
	SlackToken      string
	SlackChannelID  string
}

// Load reads environment variables.
func Load() (*Config, error) {
	webhookURL := strings.TrimSpace(os.Getenv("MCP_SLACK_WEBHOOK_URL"))
	slackToken := firstNonEmptyEnv("MCP_SLACK_USER_TOKEN", "MCP_SLACK_TOKEN", "MCP_SLACK_BOT_TOKEN")
	slackChannelID := strings.TrimSpace(os.Getenv("MCP_SLACK_CHANNEL_ID"))

	return &Config{
		SlackWebhookURL: webhookURL,
		SlackToken:      slackToken,
		SlackChannelID:  slackChannelID,
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
