package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/client"
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
			want:    []string{"post_slack_message_as_user", "delete_slack_message", "list_slack_users", "search_slack_messages"},
			notWant: []string{"post_slack_message"},
		},
		{
			name:    "webhook only",
			cfg:     client.SlackClientConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"},
			want:    []string{"post_slack_message"},
			notWant: []string{"post_slack_message_as_user", "delete_slack_message", "list_slack_users", "search_slack_messages"},
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

// TestRegisterAdvertisesNoSeparatePreviewTools pins that previewing stays a mode of
// the mutating tools rather than its own tool. A standalone preview_* tool is a second
// path to the same payload that a model can reach for instead of the confirm-gated one,
// and it leaves the gate as the only thing keeping an unreviewed post from going out.
func TestRegisterAdvertisesNoSeparatePreviewTools(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{
		Token:      "xoxp-test",
		WebhookURL: "https://hooks.slack.com/services/T/B/X",
	})

	for name := range listToolNames(t, session) {
		if strings.HasPrefix(name, "preview_") {
			t.Errorf("tool %q registered, want previewing folded into confirm=false", name)
		}
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

// TestRegisterAdvertisesToolAnnotations pins each tool's advertised behaviour hints,
// as decoded by a real client session. Clients gate confirmation UI on these rather
// than on tool names, so a read tool missing ReadOnlyHint degrades UX, while a
// mutating tool wrongly marked read-only would let a client skip confirmation for a
// call that writes to Slack — the annotations must track what the handlers do.
func TestRegisterAdvertisesToolAnnotations(t *testing.T) {
	t.Parallel()

	// true = never writes to Slack; false = writes when confirmed.
	wantReadOnly := map[string]bool{
		"post_slack_message":         false,
		"post_slack_message_as_user": false,
		"update_slack_message":       false,
		"delete_slack_message":       false,
		"get_slack_channel_info":     true,
		"list_slack_channels":        true,
		"list_joined_slack_channels": true,
		"get_slack_channel_history":  true,
		"get_slack_thread_replies":   true,
		"search_slack_messages":      true,
		"list_slack_users":           true,
		"lookup_slack_user_by_email": true,
		"resolve_slack_user":         true,
		"get_slack_auth_info":        true,
	}
	// Posting only adds content; update/delete irreversibly replace or remove it.
	wantDestructive := map[string]bool{
		"post_slack_message":         false,
		"post_slack_message_as_user": false,
		"update_slack_message":       true,
		"delete_slack_message":       true,
	}

	session := newTestSession(t, client.SlackClientConfig{
		Token:      "xoxp-test",
		WebhookURL: "https://hooks.slack.com/services/T/B/X",
	})
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(result.Tools) != len(wantReadOnly) {
		t.Fatalf("got %d tools, want %d — update the expectations with the new tool's hints", len(result.Tools), len(wantReadOnly))
	}

	for _, tool := range result.Tools {
		readOnly, known := wantReadOnly[tool.Name]
		if !known {
			t.Errorf("tool %q not in expectations — add it with its hints", tool.Name)
			continue
		}
		if tool.Title == "" {
			t.Errorf("tool %q has no title", tool.Name)
		}
		if tool.Annotations == nil {
			t.Errorf("tool %q has no annotations", tool.Name)
			continue
		}
		if tool.Annotations.ReadOnlyHint != readOnly {
			t.Errorf("tool %q readOnlyHint = %v, want %v", tool.Name, tool.Annotations.ReadOnlyHint, readOnly)
		}
		if tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Errorf("tool %q openWorldHint = %v, want explicit false: every tool talks to the one configured workspace", tool.Name, tool.Annotations.OpenWorldHint)
		}
		if destructive, mutating := wantDestructive[tool.Name]; mutating {
			if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint != destructive {
				t.Errorf("tool %q destructiveHint = %v, want explicit %v", tool.Name, tool.Annotations.DestructiveHint, destructive)
			}
			// Only update/delete are idempotent: repeating them changes nothing more,
			// while repeating a post adds another message.
			if tool.Annotations.IdempotentHint != destructive {
				t.Errorf("tool %q idempotentHint = %v, want %v", tool.Name, tool.Annotations.IdempotentHint, destructive)
			}
		}
	}
}
