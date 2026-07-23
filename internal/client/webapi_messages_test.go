package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostWebAPIMessage(t *testing.T) {
	t.Parallel()

	got := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.postMessage" {
			t.Fatalf("path = %s, want /chat.postMessage", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		got["token"] = r.Form.Get("token")
		got["channel"] = r.Form.Get("channel")
		got["text"] = r.Form.Get("text")
		got["blocks"] = r.Form.Get("blocks")
		got["attachments"] = r.Form.Get("attachments")
		got["thread_ts"] = r.Form.Get("thread_ts")
		got["icon_emoji"] = r.Form.Get("icon_emoji")
		got["unfurl_links"] = r.Form.Get("unfurl_links")
		got["unfurl_media"] = r.Form.Get("unfurl_media")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100","message":{"text":"*hello* <@shouni>"}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:            " xoxp-test ",
		DefaultChannelID: " C123 ",
		APIBaseURL:       server.URL,
	})
	unfurlLinks := false
	unfurlMedia := false
	resp, err := client.PostWebAPIMessage(context.Background(), WebAPIMessage{
		Text: "*hello* <@shouni>",
		Blocks: []map[string]any{
			{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": "*hello*"}},
		},
		Attachments: []map[string]any{
			{"fallback": "fallback text", "text": "attachment text"},
		},
		ThreadTS:    "123.456",
		IconEmoji:   ":robot_face:",
		UnfurlLinks: &unfurlLinks,
		UnfurlMedia: &unfurlMedia,
	})
	if err != nil {
		t.Fatalf("PostWebAPIMessage() error = %v", err)
	}
	if !resp.OK || resp.ChannelID != "C123" || resp.TS != "1700000000.000100" {
		t.Fatalf("response = %+v", resp)
	}
	if got["token"] != "xoxp-test" || got["channel"] != "C123" || got["text"] != "*hello* <@shouni>" {
		t.Fatalf("payload = %+v", got)
	}
	if got["blocks"] == "" || got["attachments"] == "" {
		t.Fatalf("blocks/attachments missing from payload: %+v", got)
	}
	if got["thread_ts"] != "123.456" || got["icon_emoji"] != ":robot_face:" || got["unfurl_links"] != "false" || got["unfurl_media"] != "false" {
		t.Fatalf("message options missing from payload: %+v", got)
	}
}

func TestPostWebAPIMessageReturnsSlackError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"not_in_channel"}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})
	if _, err := client.PostWebAPIMessage(context.Background(), WebAPIMessage{Text: "hello"}); err == nil {
		t.Fatal("PostWebAPIMessage() error = nil, want error")
	}
}

func TestPreviewWebAPIMessageBuildsPayloadWithoutToken(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{
		DefaultChannelID: " C123 ",
		SourceLabel:      "ap-mcp-slack (MCP) 経由",
	})
	payload, err := client.PreviewWebAPIMessage(WebAPIMessage{
		Text:      "*hello* <@shouni>",
		ThreadTS:  "123.456",
		IconEmoji: ":robot_face:",
	})
	if err != nil {
		t.Fatalf("PreviewWebAPIMessage() error = %v", err)
	}
	if payload.ChannelID != "C123" || payload.Text != "*hello* <@shouni>" || payload.ThreadTS != "123.456" || payload.IconEmoji != ":robot_face:" {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.Blocks) != 2 {
		t.Fatalf("Blocks length = %d, want 2", len(payload.Blocks))
	}
	if payload.Blocks[0]["type"] != "section" || payload.Blocks[1]["type"] != "context" {
		t.Fatalf("Blocks = %+v, want section and context", payload.Blocks)
	}
}

func TestPreviewWebAPIMessageValidatesChannel(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.PreviewWebAPIMessage(WebAPIMessage{Text: "hello"}); err == nil {
		t.Fatal("PreviewWebAPIMessage() error = nil, want channel error")
	}
}

func TestUpdateWebAPIMessage(t *testing.T) {
	t.Parallel()

	got := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.update" {
			t.Fatalf("path = %s, want /chat.update", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		got["token"] = r.Form.Get("token")
		got["channel"] = r.Form.Get("channel")
		got["ts"] = r.Form.Get("ts")
		got["text"] = r.Form.Get("text")
		got["blocks"] = r.Form.Get("blocks")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100","text":"*updated*"}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
		SourceLabel:      "ap-mcp-slack (MCP) 経由",
	})
	resp, err := client.UpdateWebAPIMessage(context.Background(), UpdateWebAPIMessage{
		TS:   "1700000000.000100",
		Text: "*updated*",
	})
	if err != nil {
		t.Fatalf("UpdateWebAPIMessage() error = %v", err)
	}
	if !resp.OK || resp.ChannelID != "C123" || resp.TS != "1700000000.000100" || resp.Text != "*updated*" {
		t.Fatalf("response = %+v", resp)
	}
	if got["token"] != "xoxp-test" || got["channel"] != "C123" || got["ts"] != "1700000000.000100" || got["text"] != "*updated*" {
		t.Fatalf("payload = %+v", got)
	}
	if got["blocks"] == "" {
		t.Fatalf("blocks missing from payload (source label footer should be appended): %+v", got)
	}
}

func TestUpdateWebAPIMessageValidatesInputs(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.UpdateWebAPIMessage(context.Background(), UpdateWebAPIMessage{ChannelID: "C123", TS: "123.456", Text: "hi"}); err == nil {
		t.Fatal("UpdateWebAPIMessage() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.UpdateWebAPIMessage(context.Background(), UpdateWebAPIMessage{TS: "123.456", Text: "hi"}); err == nil {
		t.Fatal("UpdateWebAPIMessage() error = nil, want channel error")
	}
	if _, err := client.UpdateWebAPIMessage(context.Background(), UpdateWebAPIMessage{ChannelID: "C123", Text: "hi"}); err == nil {
		t.Fatal("UpdateWebAPIMessage() error = nil, want ts error")
	}
	if _, err := client.UpdateWebAPIMessage(context.Background(), UpdateWebAPIMessage{ChannelID: "C123", TS: "123.456"}); err == nil {
		t.Fatal("UpdateWebAPIMessage() error = nil, want text/blocks/attachments error")
	}
}

func TestUpdateWebAPIMessageAllowsAttachmentsOnly(t *testing.T) {
	t.Parallel()

	// Slack's chat.update accepts an update carrying only attachments (no text or
	// blocks), same as chat.postMessage; the content-required check must not treat
	// attachments as insufficient on its own.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	_, err := client.UpdateWebAPIMessage(context.Background(), UpdateWebAPIMessage{
		ChannelID: "C123",
		TS:        "1700000000.000100",
		Attachments: []map[string]any{
			{"fallback": "fallback text", "text": "attachment text"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateWebAPIMessage() error = %v, want attachments-only update to succeed", err)
	}
}

func TestDeleteWebAPIMessage(t *testing.T) {
	t.Parallel()

	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.delete" {
			t.Fatalf("path = %s, want /chat.delete", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		got = map[string]string{
			"token":   r.Form.Get("token"),
			"channel": r.Form.Get("channel"),
			"ts":      r.Form.Get("ts"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})
	resp, err := client.DeleteWebAPIMessage(context.Background(), "", "1700000000.000100")
	if err != nil {
		t.Fatalf("DeleteWebAPIMessage() error = %v", err)
	}
	if !resp.OK || resp.ChannelID != "C123" || resp.TS != "1700000000.000100" {
		t.Fatalf("response = %+v", resp)
	}
	if got["token"] != "xoxp-test" || got["channel"] != "C123" || got["ts"] != "1700000000.000100" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestWebAPIRequiresTokenChannelAndTS(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.PostWebAPIMessage(context.Background(), WebAPIMessage{ChannelID: "C123", Text: "hello"}); err == nil {
		t.Fatal("PostWebAPIMessage() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.PostWebAPIMessage(context.Background(), WebAPIMessage{Text: "hello"}); err == nil {
		t.Fatal("PostWebAPIMessage() error = nil, want channel error")
	}
	if _, err := client.DeleteWebAPIMessage(context.Background(), "C123", " "); err == nil {
		t.Fatal("DeleteWebAPIMessage() error = nil, want ts error")
	}
}
