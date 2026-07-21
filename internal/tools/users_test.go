package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ap-mcp-slack/internal/client"
)

func TestListSlackUsers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.list" {
			t.Fatalf("path = %s, want /users.list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"members":[{"id":"U001","name":"alice","real_name":"Alice A"}],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})

	var out ListSlackUsersOutput
	result := callTool(t, session, "list_slack_users", map[string]any{}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Count != 1 || out.Users[0].ID != "U001" {
		t.Fatalf("out = %+v", out)
	}
}

func TestListSlackUsersRequiresToken(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{})

	result := callTool(t, session, "list_slack_users", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want token error")
	}
}

func TestLookupSlackUserByEmail(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.lookupByEmail" {
			t.Fatalf("path = %s, want /users.lookupByEmail", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"user":{"id":"U001","name":"alice","profile":{"email":"alice@example.com"}}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})

	var out LookupSlackUserByEmailOutput
	result := callTool(t, session, "lookup_slack_user_by_email", map[string]any{
		"email": "alice@example.com",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.User == nil || out.User.ID != "U001" {
		t.Fatalf("out = %+v", out)
	}
}

func TestLookupSlackUserByEmailRequiresEmail(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test"})

	result := callTool(t, session, "lookup_slack_user_by_email", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want email required error")
	}
}

func TestResolveSlackUser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.lookupByEmail" {
			t.Fatalf("path = %s, want /users.lookupByEmail", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"user":{"id":"U001","name":"alice","profile":{"email":"alice@example.com"}}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})

	var out ResolveSlackUserOutput
	result := callTool(t, session, "resolve_slack_user", map[string]any{
		"email": "alice@example.com",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Status != client.ResolveUserStatusFound || out.User == nil || out.Mention != "<@U001>" {
		t.Fatalf("out = %+v", out)
	}
}

func TestResolveSlackUserRequiresNameOrEmail(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test"})

	result := callTool(t, session, "resolve_slack_user", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want name/email required error")
	}
}
