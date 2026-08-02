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

// run はサーバーの初期化と起動を行います。defer によるクリーンアップが
// os.Exit で無視されないよう、終了コードの決定は main 側に委ねます。
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
