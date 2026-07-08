package server

import (
	"testing"

	"ap-mcp-slack/internal/app"
	"ap-mcp-slack/internal/client"
)

func TestNew(t *testing.T) {
	t.Parallel()

	container := &app.Container{Slack: client.NewSlackClient("http://example.test")}
	if got := New(container); got == nil {
		t.Fatal("New() = nil")
	}
}
