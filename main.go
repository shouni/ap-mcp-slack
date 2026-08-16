// ap-mcp-slack exposes Slack Incoming Webhook posting and Slack Web API messaging,
// channel/message reads, and user lookups as an MCP stdio server.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shouni/ap-mcp-slack/internal/app"
	"github.com/shouni/ap-mcp-slack/internal/config"
	"github.com/shouni/ap-mcp-slack/internal/server"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run initializes and runs the server. It returns the error rather than calling
// os.Exit itself, so its deferred cleanup is not skipped; deciding the exit code is
// left to main.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定の読み込みに失敗しました", "error", err)
		return err
	}

	srv := server.New(app.NewContainer(cfg))
	if err := srv.Run(ctx); err != nil {
		slog.Error("サーバーが異常終了しました", "error", err)
		return err
	}
	return nil
}
