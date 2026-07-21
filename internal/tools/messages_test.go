package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ap-mcp-slack/internal/client"
)

func TestPreviewSlackMessage(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{
		WebhookURL:  "https://hooks.slack.com/services/T/B/X",
		SourceLabel: "ap-mcp-slack (MCP) 経由",
	})

	var out PreviewSlackMessageOutput
	result := callTool(t, session, "preview_slack_message", map[string]any{
		"text": "*hello*",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Transport != "webhook" || out.Payload.Text != "*hello*" {
		t.Fatalf("out = %+v", out)
	}
	if len(out.Payload.Blocks) != 2 {
		t.Fatalf("Blocks = %+v, want section+context (source label)", out.Payload.Blocks)
	}
}

func TestPreviewSlackMessageRequiresText(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"})

	result := callTool(t, session, "preview_slack_message", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want error for missing text")
	}
}

func TestPostSlackMessageRequiresWebhookURL(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{})

	result := callTool(t, session, "post_slack_message", map[string]any{"text": "hello"}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want webhook URL error")
	}
}

func TestPreviewSlackMessageAsUser(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
	})

	var out PreviewSlackMessageAsUserOutput
	result := callTool(t, session, "preview_slack_message_as_user", map[string]any{
		"text": "*hello*",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Transport != "web_api" || out.ChannelID != "C123" || out.Payload.Text != "*hello*" {
		t.Fatalf("out = %+v", out)
	}
}

func TestPreviewSlackMessageAsUserRequiresChannel(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test"})

	result := callTool(t, session, "preview_slack_message_as_user", map[string]any{"text": "hello"}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want channel_id error")
	}
}

func TestPostSlackMessageAsUser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.postMessage" {
			t.Fatalf("path = %s, want /chat.postMessage", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out PostSlackMessageAsUserOutput
	result := callTool(t, session, "post_slack_message_as_user", map[string]any{
		"text": "*hello*",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.ChannelID != "C123" || out.TS != "1700000000.000100" {
		t.Fatalf("out = %+v", out)
	}
}

func TestUpdateSlackMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.update" {
			t.Fatalf("path = %s, want /chat.update", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100","text":"*updated*"}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out UpdateSlackMessageOutput
	result := callTool(t, session, "update_slack_message", map[string]any{
		"ts":   "1700000000.000100",
		"text": "*updated*",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.ChannelID != "C123" || out.TS != "1700000000.000100" || out.Text != "*updated*" {
		t.Fatalf("out = %+v", out)
	}
}

func TestUpdateSlackMessageRequiresTS(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", DefaultChannelID: "C123"})

	result := callTool(t, session, "update_slack_message", map[string]any{"text": "hi"}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want ts required error")
	}
}

func TestDeleteSlackMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.delete" {
			t.Fatalf("path = %s, want /chat.delete", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out DeleteSlackMessageOutput
	result := callTool(t, session, "delete_slack_message", map[string]any{
		"ts": "1700000000.000100",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.ChannelID != "C123" || out.TS != "1700000000.000100" {
		t.Fatalf("out = %+v", out)
	}
}

func TestDeleteSlackMessageRequiresTS(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", DefaultChannelID: "C123"})

	result := callTool(t, session, "delete_slack_message", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want ts required error")
	}
}
