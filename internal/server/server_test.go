package server

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/app"
	"github.com/shouni/ap-mcp-slack/internal/client"
	"github.com/shouni/ap-mcp-slack/internal/tools"
)

func TestNew(t *testing.T) {
	t.Parallel()

	container := &app.Container{
		Slack: client.NewSlackClientWithConfig(client.SlackClientConfig{WebhookURL: "http://example.test"}),
	}
	if got := New(container); got == nil {
		t.Fatal("New() = nil")
	}
}

// TestResolveVersion pins which of the two version sources wins. They reach a build by
// different routes — -ldflags from a local `go build`, the module version from
// `go install path@version` — so a regression here surfaces only in one of them.
func TestResolveVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		injected string
		module   string
		want     string
	}{
		{"どちらも無ければ既定値", defaultVersion, "", defaultVersion},
		{"go install はタグをそのまま報告する", defaultVersion, "v1.2.3", "v1.2.3"},
		{"リポジトリ内の go build は擬似バージョンを報告する",
			defaultVersion, "v1.0.1-0.20260802190324-af39274744ae+dirty", "v1.0.1-0.20260802190324-af39274744ae+dirty"},
		{"テストや -buildvcs=false の (devel) は採用しない", defaultVersion, "(devel)", defaultVersion},
		{"-ldflags があればそれを使う", "v1.2.3-4-gabcdef", "", "v1.2.3-4-gabcdef"},
		{"-ldflags は擬似バージョンより優先する",
			"v1.2.3-4-gabcdef", "v1.0.1-0.20260802190324-af39274744ae+dirty", "v1.2.3-4-gabcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveVersion(tt.injected, tt.module); got != tt.want {
				t.Errorf("resolveVersion(%q, %q) = %q, want %q", tt.injected, tt.module, got, tt.want)
			}
		})
	}
}

// TestNewAdvertisesInstructions pins that the cross-tool contract (confirm gate,
// default channel, pagination) reaches the client at initialize time. It lives in
// ServerOptions rather than in each tool description, so nothing else fails if the
// wiring in New is dropped — only this test notices.
func TestNewAdvertisesInstructions(t *testing.T) {
	t.Parallel()

	container := &app.Container{
		Slack: client.NewSlackClientWithConfig(client.SlackClientConfig{Token: "xoxp-test"}),
	}
	srv := New(container)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := srv.mcpServer.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	if got := session.InitializeResult().Instructions; got != tools.ServerInstructions {
		t.Errorf("instructions = %q, want tools.ServerInstructions", got)
	}
}
