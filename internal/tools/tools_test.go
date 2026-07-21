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
