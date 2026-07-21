package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ap-mcp-slack/internal/client"
)

func TestListSlackChannels(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.list" {
			t.Fatalf("path = %s, want /conversations.list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C001","name":"alpha","is_channel":true}],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})

	var out ListSlackChannelsOutput
	result := callTool(t, session, "list_slack_channels", map[string]any{}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Count != 1 || out.Channels[0].ID != "C001" {
		t.Fatalf("out = %+v", out)
	}
}

func TestListSlackChannelsRequiresToken(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{})

	result := callTool(t, session, "list_slack_channels", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want token error")
	}
}

func TestListJoinedSlackChannels(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.conversations" {
			t.Fatalf("path = %s, want /users.conversations", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C001","name":"alpha","is_channel":true,"is_member":true}],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})

	var out ListJoinedSlackChannelsOutput
	result := callTool(t, session, "list_joined_slack_channels", map[string]any{}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Count != 1 || !out.Channels[0].IsMember {
		t.Fatalf("out = %+v", out)
	}
}

func TestGetSlackChannelInfo(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.info" {
			t.Fatalf("path = %s, want /conversations.info", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general","is_channel":true,"num_members":5}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out GetSlackChannelInfoOutput
	result := callTool(t, session, "get_slack_channel_info", map[string]any{
		"include_num_members": true,
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Channel.ID != "C123" || out.Channel.Name != "general" || out.Channel.NumMembers != 5 {
		t.Fatalf("out = %+v", out)
	}
}

func TestGetSlackChannelInfoRequiresChannel(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test"})

	result := callTool(t, session, "get_slack_channel_info", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want channel_id error")
	}
}

func TestGetSlackChannelHistory(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.history" {
			t.Fatalf("path = %s, want /conversations.history", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"hi","ts":"1700000000.000100"}],"has_more":false,"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out GetSlackMessagesOutput
	result := callTool(t, session, "get_slack_channel_history", map[string]any{}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Count != 1 || out.Messages[0].Text != "hi" {
		t.Fatalf("out = %+v", out)
	}
}

func TestGetSlackThreadReplies(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.replies" {
			t.Fatalf("path = %s, want /conversations.replies", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"parent","ts":"1700000000.000100"},{"type":"message","user":"U002","text":"reply","ts":"1700000001.000100","parent_user_id":"U001"}],"has_more":false,"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out GetSlackMessagesOutput
	result := callTool(t, session, "get_slack_thread_replies", map[string]any{
		"ts": "1700000000.000100",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Count != 2 || out.Messages[1].ParentUser != "U001" {
		t.Fatalf("out = %+v", out)
	}
}

func TestGetSlackThreadRepliesRequiresTS(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", DefaultChannelID: "C123"})

	result := callTool(t, session, "get_slack_thread_replies", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want ts required error")
	}
}
