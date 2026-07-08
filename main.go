// ap-mcp-slack exposes Slack Incoming Webhook posting as an MCP stdio server.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ap-mcp-slack/internal/app"
	"ap-mcp-slack/internal/builder"
	"ap-mcp-slack/internal/config"
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

	container, err := app.NewContainer(cfg)
	if err != nil {
		slog.Error("コンテナの初期化に失敗しました", "error", err)
		return err
	}

	srv, err := builder.BuildServer(container)
	if err != nil {
		slog.Error("MCPサーバーの構築に失敗しました", "error", err)
		return err
	}

	if err := srv.Run(ctx); err != nil {
		slog.Error("サーバーが異常終了しました", "error", err)
		return err
	}
	return nil
}
