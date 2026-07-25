package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

// newTestSession registers SlackTools built from cfg on a real in-process MCP
// server, connects a client to it over mcp.NewInMemoryTransports, and returns the
// client session. Driving tools through an actual ClientSession.CallTool (rather
// than calling the unexported handler methods directly) exercises the same JSON
// Schema validation and (de)serialization a real MCP client would go through.
func newTestSession(t *testing.T, cfg client.SlackClientConfig) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	NewSlackTools(client.NewSlackClientWithConfig(cfg)).Register(server)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session
}

// listToolNames returns the names of the tools session's server advertises.
func listToolNames(t *testing.T, session *mcp.ClientSession) map[string]bool {
	t.Helper()

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	names := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	return names
}

// TestRegisterGatesToolsByConfiguredTransport pins the registration gate: a model
// chooses from the advertised tool list, so a transport without credentials must not
// appear there at all. Offering it would let the model build a payload and collect a
// human's approval for a call that was always going to fail.
func TestRegisterGatesToolsByConfiguredTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     client.SlackClientConfig
		want    []string
		notWant []string
	}{
		{
			name:    "token only",
			cfg:     client.SlackClientConfig{Token: "xoxp-test"},
			want:    []string{"post_slack_message_as_user", "delete_slack_message", "list_slack_users"},
			notWant: []string{"post_slack_message", "preview_slack_message"},
		},
		{
			name:    "webhook only",
			cfg:     client.SlackClientConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"},
			want:    []string{"post_slack_message", "preview_slack_message"},
			notWant: []string{"post_slack_message_as_user", "delete_slack_message", "list_slack_users"},
		},
		{
			name: "both",
			cfg: client.SlackClientConfig{
				Token:      "xoxp-test",
				WebhookURL: "https://hooks.slack.com/services/T/B/X",
			},
			want: []string{"post_slack_message", "post_slack_message_as_user"},
		},
		{
			name:    "neither",
			cfg:     client.SlackClientConfig{},
			notWant: []string{"post_slack_message", "post_slack_message_as_user"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			names := listToolNames(t, newTestSession(t, tc.cfg))
			for _, name := range tc.want {
				if !names[name] {
					t.Errorf("tool %q not registered, want registered", name)
				}
			}
			for _, name := range tc.notWant {
				if names[name] {
					t.Errorf("tool %q registered, want absent", name)
				}
			}
		})
	}
}

// callTool invokes name with args on session. If out is non-nil and the call
// succeeds, the structured result is decoded into out.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", name, err)
	}
	if out != nil && !result.IsError {
		data, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatalf("CallTool(%s): marshal structured content: %v", name, err)
		}
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("CallTool(%s): unmarshal structured content: %v", name, err)
		}
	}
	return result
}
