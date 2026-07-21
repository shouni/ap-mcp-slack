package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ap-mcp-slack/internal/client"
)

func TestGetSlackAuthInfo(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth.test" {
			t.Fatalf("path = %s, want /auth.test", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"team":"Example","team_id":"T123","user":"bot","user_id":"U123","bot_id":"B123"}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})

	var out GetSlackAuthInfoOutput
	result := callTool(t, session, "get_slack_auth_info", map[string]any{}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Team != "Example" || out.TeamID != "T123" || out.BotID != "B123" {
		t.Fatalf("out = %+v", out)
	}
}

func TestGetSlackAuthInfoRequiresToken(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{})

	result := callTool(t, session, "get_slack_auth_info", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want token error")
	}
}
