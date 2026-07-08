// ap-mcp-slack exposes Slack Incoming Webhook posting as an MCP stdio server.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ap-mcp-slack/internal/client"
	"ap-mcp-slack/internal/config"
	"ap-mcp-slack/internal/server"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ap-mcp-slack: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slackClient := client.NewSlackClientWithConfig(client.SlackClientConfig{
		WebhookURL:       cfg.SlackWebhookURL,
		Token:            cfg.SlackToken,
		DefaultChannelID: cfg.SlackChannelID,
	})
	srv := server.New(slackClient)
	return srv.Run(ctx)
}
