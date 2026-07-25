package builder

import (
	"testing"

	"ap-mcp-slack/internal/app"
	"ap-mcp-slack/internal/config"
)

func TestBuildServer(t *testing.T) {
	t.Parallel()

	container, err := app.NewContainer(&config.Config{SlackToken: "xoxp-test"})
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	srv, err := BuildServer(container)
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if srv == nil {
		t.Fatal("BuildServer() = nil")
	}
}
