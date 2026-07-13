// Package client provides outbound service clients.
package client

import (
	"fmt"
	"strings"
	"time"
)

const requestTimeout = 10 * time.Second

// SlackClient posts and deletes Slack messages through incoming webhooks and Web API.
// It composes a webhook transport (Message/PostMessage) and a token-authenticated Web
// API transport (WebAPIMessage/PostWebAPIMessage/DeleteWebAPIMessage/ListChannels).
// The two share no state, so they're kept as separate embedded types rather than one
// struct with fields that are only meaningful to one side or the other.
type SlackClient struct {
	webhookTransport
	webAPITransport
}

// SlackClientConfig configures SlackClient.
type SlackClientConfig struct {
	WebhookURL       string
	Token            string
	DefaultChannelID string
	APIBaseURL       string
	// SourceLabel, if set, is appended as a Block Kit context footer on every posted
	// message so MCP-originated posts stay distinguishable from messages typed by
	// hand.
	SourceLabel string
}

// NewSlackClient creates a SlackClient.
func NewSlackClient(webhookURL string) *SlackClient {
	return NewSlackClientWithConfig(SlackClientConfig{WebhookURL: webhookURL})
}

// NewSlackClientWithConfig creates a SlackClient with explicit configuration.
func NewSlackClientWithConfig(cfg SlackClientConfig) *SlackClient {
	return &SlackClient{
		webhookTransport: newWebhookTransport(cfg),
		webAPITransport:  newWebAPITransport(cfg),
	}
}

// normalizeListLimit validates and applies defaults to a limit option shared by the
// paginated list operations (channels, joined channels, users).
func normalizeListLimit(limit, defaultLimit, maxLimit int) (int, error) {
	if limit == 0 {
		return defaultLimit, nil
	}
	if limit < 0 {
		return 0, fmt.Errorf("slack: limit must be greater than 0")
	}
	if limit > maxLimit {
		return 0, fmt.Errorf("slack: limit must be %d or less", maxLimit)
	}
	return limit, nil
}

// appendRawSourceLabelBlock appends a Block Kit context footer carrying sourceLabel,
// shared by both the webhook and Web API message payload builders.
func appendRawSourceLabelBlock(blocks []map[string]any, text, sourceLabel string) []map[string]any {
	sourceLabel = strings.TrimSpace(sourceLabel)
	if sourceLabel == "" {
		return blocks
	}

	if len(blocks) == 0 && strings.TrimSpace(text) != "" {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": text,
			},
		})
	}

	return append(blocks, map[string]any{
		"type": "context",
		"elements": []map[string]any{
			{
				"type": "mrkdwn",
				"text": sourceLabel,
			},
		},
	})
}
