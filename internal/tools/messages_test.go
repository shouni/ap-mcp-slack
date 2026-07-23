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

func TestPreviewSlackMessageWithMentions(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{
		WebhookURL: "https://hooks.slack.com/services/T/B/X",
	})

	var out PreviewSlackMessageOutput
	result := callTool(t, session, "preview_slack_message", map[string]any{
		"text":     "*hello*",
		"mentions": []string{"U001", "U002"},
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if len(out.Mentions) != 2 || out.Mentions[0] != "U001" || out.Mentions[1] != "U002" {
		t.Fatalf("out.Mentions = %+v", out.Mentions)
	}
	if out.Payload.Text != "<@U001> <@U002>\n*hello*" {
		t.Fatalf("out.Payload.Text = %q, want mention prefix", out.Payload.Text)
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

func TestPostSlackMessageWithoutConfirmDoesNotPost(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"})

	var out PostSlackMessageOutput
	result := callTool(t, session, "post_slack_message", map[string]any{"text": "hello"}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if out.Posted || out.Payload.Text != "hello" {
		t.Fatalf("out = %+v, want a preview-only, unposted response", out)
	}
}

func TestPostSlackMessageRequiresWebhookURL(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{})

	result := callTool(t, session, "post_slack_message", map[string]any{"text": "hello", "confirm": true}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want webhook URL error")
	}
}

func TestPreviewSlackMessageAsUser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.info" {
			t.Fatalf("path = %s, want /conversations.info", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general"}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out PreviewSlackMessageAsUserOutput
	result := callTool(t, session, "preview_slack_message_as_user", map[string]any{
		"text": "*hello*",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Transport != "web_api" || out.ChannelID != "C123" || out.ChannelName != "general" || out.Payload.Text != "*hello*" {
		t.Fatalf("out = %+v", out)
	}
}

func TestPreviewSlackMessageAsUserResolvesMentionsAndThreadParent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.info":
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general"}}`))
		case "/users.info":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			switch r.Form.Get("user") {
			case "U001":
				_, _ = w.Write([]byte(`{"ok":true,"user":{"id":"U001","name":"alice","real_name":"Alice A"}}`))
			default:
				t.Fatalf("unexpected user %q", r.Form.Get("user"))
			}
		case "/conversations.replies":
			_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U002","text":"parent message","ts":"1700000000.000100"}],"has_more":false}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out PreviewSlackMessageAsUserOutput
	result := callTool(t, session, "preview_slack_message_as_user", map[string]any{
		"text":      "*hello*",
		"mentions":  []string{"U001"},
		"thread_ts": "1700000000.000100",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if out.ChannelName != "general" {
		t.Fatalf("out.ChannelName = %q, want general", out.ChannelName)
	}
	if len(out.Mentions) != 1 || out.Mentions[0].ID != "U001" || out.Mentions[0].RealName != "Alice A" || out.Mentions[0].Mention != "<@U001>" {
		t.Fatalf("out.Mentions = %+v", out.Mentions)
	}
	if out.ThreadParent == nil || out.ThreadParent.Text != "parent message" || out.ThreadParent.User != "U002" {
		t.Fatalf("out.ThreadParent = %+v", out.ThreadParent)
	}
	if out.Payload.Text != "<@U001>\n*hello*" {
		t.Fatalf("out.Payload.Text = %q, want mention prefix", out.Payload.Text)
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

func TestPostSlackMessageAsUserWithoutConfirmDoesNotPost(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat.postMessage" {
			t.Fatal("chat.postMessage called without confirm=true")
		}
		if r.URL.Path != "/conversations.info" {
			t.Fatalf("path = %s, want /conversations.info", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general"}}`))
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
	if out.Posted || out.ChannelName != "general" || out.TS != "" {
		t.Fatalf("out = %+v, want a preview-only, unposted response", out)
	}
}

func TestPostSlackMessageAsUser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.info":
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general"}}`))
		case "/chat.postMessage":
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out PostSlackMessageAsUserOutput
	result := callTool(t, session, "post_slack_message_as_user", map[string]any{
		"text":    "*hello*",
		"confirm": true,
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || !out.Posted || out.ChannelID != "C123" || out.TS != "1700000000.000100" {
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

func TestUpdateSlackMessageAllowsAttachmentsOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	result := callTool(t, session, "update_slack_message", map[string]any{
		"ts": "1700000000.000100",
		"attachments": []map[string]any{
			{"fallback": "fallback text", "text": "attachment text"},
		},
	}, nil)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, want attachments-only update to succeed, content = %+v", result.Content)
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
