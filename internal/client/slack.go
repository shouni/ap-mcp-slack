// Package client provides outbound service clients.
package client

import (
	"fmt"
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
	// SourceLabel, if set, is appended as a Block Kit context footer on every Web
	// API ("post as user") message. Incoming Webhook posts don't need it: Slack
	// already renders those under the app's own name/icon with an "APP" badge, so
	// they're already distinguishable from a message the user typed themselves. A
	// user-token chat.postMessage call has no such marker.
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
