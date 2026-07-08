package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostMessage(t *testing.T) {
	t.Parallel()

	var got Message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewSlackClient(server.URL)
	unfurlLinks := false
	resp, err := client.PostMessage(context.Background(), Message{
		Text: "*hello* <@shouni>",
		Blocks: []map[string]any{
			{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": "*hello*"}},
		},
		ThreadTS:    "123.456",
		UnfurlLinks: &unfurlLinks,
	})
	if err != nil {
		t.Fatalf("PostMessage() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK || resp.Body != "ok" {
		t.Fatalf("response = %+v, want status 200 body ok", resp)
	}
	if got.Text != "*hello* <@shouni>" || got.ThreadTS != "123.456" {
		t.Fatalf("payload = %+v", got)
	}
	if got.UnfurlLinks == nil || *got.UnfurlLinks {
		t.Fatalf("UnfurlLinks = %v, want false pointer", got.UnfurlLinks)
	}
	if len(got.Blocks) != 1 {
		t.Fatalf("Blocks length = %d, want 1", len(got.Blocks))
	}
}

func TestPostMessageRequiresText(t *testing.T) {
	t.Parallel()

	client := NewSlackClient("http://example.test")
	if _, err := client.PostMessage(context.Background(), Message{Text: "  "}); err == nil {
		t.Fatal("PostMessage() error = nil, want error")
	}
}

func TestPostMessageReturnsSlackError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid_payload", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewSlackClient(server.URL)
	if _, err := client.PostMessage(context.Background(), Message{Text: "hello"}); err == nil {
		t.Fatal("PostMessage() error = nil, want error")
	}
}

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

func TestListChannels(t *testing.T) {
	t.Parallel()

	type request struct {
		Cursor          string
		Limit           string
		Types           string
		ExcludeArchived string
		Token           string
	}
	var requests []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.list" {
			t.Fatalf("path = %s, want /conversations.list", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		requests = append(requests, request{
			Cursor:          r.Form.Get("cursor"),
			Limit:           r.Form.Get("limit"),
			Types:           r.Form.Get("types"),
			ExcludeArchived: r.Form.Get("exclude_archived"),
			Token:           r.Form.Get("token"),
		})

		w.Header().Set("Content-Type", "application/json")
		switch r.Form.Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C002","name":"zeta","created":20,"is_channel":true,"num_members":2},{"id":"C001","name":"alpha","created":10,"is_channel":true,"num_members":1}],"response_metadata":{"next_cursor":"cursor-2"}}`))
		case "cursor-2":
			_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C003","name":"beta","created":30,"is_channel":true,"num_members":3}],"response_metadata":{"next_cursor":""}}`))
		default:
			t.Fatalf("unexpected cursor %q", r.Form.Get("cursor"))
		}
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ListChannels(context.Background(), ListChannelsOptions{
		Types:           []string{"public_channel", "private_channel", "public_channel"},
		ExcludeArchived: true,
		Limit:           3,
	})
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if !resp.OK || resp.Count != 3 || resp.Sort != ChannelSortNameAsc || resp.NextCursor != "" {
		t.Fatalf("response = %+v", resp)
	}
	wantNames := []string{"alpha", "beta", "zeta"}
	if len(resp.Names) != len(wantNames) {
		t.Fatalf("names = %+v, want %+v", resp.Names, wantNames)
	}
	for i, want := range wantNames {
		if resp.Names[i] != want || resp.Channels[i].Name != want {
			t.Fatalf("names/channels = %+v / %+v, want %q at %d", resp.Names, resp.Channels, want, i)
		}
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %+v, want 2 requests", requests)
	}
	if requests[0].Token != "xoxp-test" || requests[0].Limit != "3" || requests[0].Types != "public_channel,private_channel" || requests[0].ExcludeArchived != "true" {
		t.Fatalf("first request = %+v", requests[0])
	}
	if requests[1].Cursor != "cursor-2" || requests[1].Limit != "1" {
		t.Fatalf("second request = %+v", requests[1])
	}
}

func TestListChannelsReturnsNextCursorWhenLimitReached(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.list" {
			t.Fatalf("path = %s, want /conversations.list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C001","name":"alpha","created":10,"is_channel":true}],"response_metadata":{"next_cursor":"cursor-2"}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ListChannels(context.Background(), ListChannelsOptions{Limit: 1})
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if resp.Count != 1 || resp.NextCursor != "cursor-2" {
		t.Fatalf("response = %+v, want count 1 next cursor-2", resp)
	}
}

func TestListChannelsValidatesInputs(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.ListChannels(context.Background(), ListChannelsOptions{}); err == nil {
		t.Fatal("ListChannels() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.ListChannels(context.Background(), ListChannelsOptions{Types: []string{"invalid"}}); err == nil {
		t.Fatal("ListChannels() error = nil, want type error")
	}
	if _, err := client.ListChannels(context.Background(), ListChannelsOptions{Sort: "updated_desc"}); err == nil {
		t.Fatal("ListChannels() error = nil, want sort error")
	}
	if _, err := client.ListChannels(context.Background(), ListChannelsOptions{Limit: maxChannelListLimit + 1}); err == nil {
		t.Fatal("ListChannels() error = nil, want limit error")
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
