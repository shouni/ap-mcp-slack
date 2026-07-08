package server

import (
	"testing"

	"ap-mcp-slack/internal/client"
)

func TestNew(t *testing.T) {
	t.Parallel()

	if got := New(client.NewSlackClient("http://example.test")); got == nil {
		t.Fatal("New() = nil")
	}
}
