package app

import (
	"testing"

	"ap-mcp-slack/internal/config"
)

// TestNewContainerWiresConfigIntoTheClient checks the wiring that decides which tools
// the server ends up advertising: registration is gated on the client's view of which
// transports are configured, so a config field dropped here would silently remove
// tools rather than fail.
func TestNewContainerWiresConfigIntoTheClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cfg            *config.Config
		wantWebhook    bool
		wantWebAPIAuth bool
	}{
		{
			name:        "webhook only",
			cfg:         &config.Config{SlackWebhookURL: "https://hooks.slack.com/services/T/B/X"},
			wantWebhook: true,
		},
		{
			name:           "token only",
			cfg:            &config.Config{SlackToken: "xoxp-test"},
			wantWebAPIAuth: true,
		},
		{
			name: "both",
			cfg: &config.Config{
				SlackWebhookURL: "https://hooks.slack.com/services/T/B/X",
				SlackToken:      "xoxp-test",
			},
			wantWebhook:    true,
			wantWebAPIAuth: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			container := NewContainer(tc.cfg)
			if container.Slack == nil {
				t.Fatal("Container.Slack = nil")
			}
			if container.Config != tc.cfg {
				t.Fatalf("Container.Config = %+v, want the config passed in", container.Config)
			}
			if got := container.Slack.WebhookConfigured(); got != tc.wantWebhook {
				t.Errorf("WebhookConfigured() = %v, want %v", got, tc.wantWebhook)
			}
			if got := container.Slack.WebAPIConfigured(); got != tc.wantWebAPIAuth {
				t.Errorf("WebAPIConfigured() = %v, want %v", got, tc.wantWebAPIAuth)
			}
		})
	}
}
