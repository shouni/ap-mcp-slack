package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("MCP_SLACK_WEBHOOK_URL", "https://hooks.slack.com/services/T000/B000/secret")
	t.Setenv("MCP_SLACK_USER_TOKEN", "xoxp-secret")
	t.Setenv("MCP_SLACK_CHANNEL_ID", "C123")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SlackWebhookURL != "https://hooks.slack.com/services/T000/B000/secret" {
		t.Fatalf("SlackWebhookURL = %q", cfg.SlackWebhookURL)
	}
	if cfg.SlackToken != "xoxp-secret" {
		t.Fatalf("SlackToken = %q", cfg.SlackToken)
	}
	if cfg.SlackChannelID != "C123" {
		t.Fatalf("SlackChannelID = %q", cfg.SlackChannelID)
	}
}

func TestLoadAllowsTokenOnly(t *testing.T) {
	t.Setenv("MCP_SLACK_WEBHOOK_URL", "")
	t.Setenv("MCP_SLACK_USER_TOKEN", "xoxp-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SlackWebhookURL != "" {
		t.Fatalf("SlackWebhookURL = %q, want empty", cfg.SlackWebhookURL)
	}
	if cfg.SlackToken != "xoxp-secret" {
		t.Fatalf("SlackToken = %q", cfg.SlackToken)
	}
}

func TestLoadAllowsWebhookOnly(t *testing.T) {
	t.Setenv("MCP_SLACK_WEBHOOK_URL", "https://hooks.slack.com/services/T000/B000/secret")
	t.Setenv("MCP_SLACK_USER_TOKEN", "")
	t.Setenv("MCP_SLACK_TOKEN", "")
	t.Setenv("MCP_SLACK_BOT_TOKEN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SlackToken != "" {
		t.Fatalf("SlackToken = %q, want empty", cfg.SlackToken)
	}
}

// TestLoadRequiresSlackCredentials pins the startup failure: with neither transport
// configured every tool would be gated out of registration, leaving a server that
// connects successfully and advertises nothing.
func TestLoadRequiresSlackCredentials(t *testing.T) {
	t.Setenv("MCP_SLACK_WEBHOOK_URL", "")
	t.Setenv("MCP_SLACK_USER_TOKEN", "")
	t.Setenv("MCP_SLACK_TOKEN", "")
	t.Setenv("MCP_SLACK_BOT_TOKEN", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing-credentials error")
	}
}
