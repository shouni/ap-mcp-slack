// Package app provides a DI container that assembles and holds the service clients
// used by the MCP server.
package app

import (
	"ap-mcp-slack/internal/client"
	"ap-mcp-slack/internal/config"
)

// Container holds the clients shared across the server.
type Container struct {
	Config *config.Config
	Slack  *client.SlackClient
}

// NewContainer builds a Container from Config.
func NewContainer(cfg *config.Config) (*Container, error) {
	return &Container{
		Config: cfg,
		Slack: client.NewSlackClientWithConfig(client.SlackClientConfig{
			WebhookURL:       cfg.SlackWebhookURL,
			Token:            cfg.SlackToken,
			DefaultChannelID: cfg.SlackChannelID,
		}),
	}, nil
}
