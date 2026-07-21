package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	slackapi "github.com/slack-go/slack"
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

func TestListChannelsKeepsPageOvershoot(t *testing.T) {
	t.Parallel()

	// Slack's pagination guide notes that a page may return more items than the
	// requested limit; the response must still include all of them instead of
	// silently dropping the overshoot when advancing past nextCursor, since those
	// items would otherwise never be retrievable again.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.list" {
			t.Fatalf("path = %s, want /conversations.list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C001","name":"alpha","created":10,"is_channel":true},{"id":"C002","name":"beta","created":20,"is_channel":true}],"response_metadata":{"next_cursor":""}}`))
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
	if resp.Count != 2 {
		t.Fatalf("response = %+v, want count 2 (overshoot kept, not dropped)", resp)
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

func TestListJoinedChannels(t *testing.T) {
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
		if r.URL.Path != "/users.conversations" {
			t.Fatalf("path = %s, want /users.conversations", r.URL.Path)
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
			_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C002","name":"zeta","created":20,"is_channel":true,"is_member":true,"num_members":2},{"id":"C001","name":"alpha","created":10,"is_channel":true,"is_member":true,"num_members":1}],"response_metadata":{"next_cursor":"cursor-2"}}`))
		case "cursor-2":
			_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C003","name":"beta","created":30,"is_channel":true,"is_member":true,"num_members":3}],"response_metadata":{"next_cursor":""}}`))
		default:
			t.Fatalf("unexpected cursor %q", r.Form.Get("cursor"))
		}
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ListJoinedChannels(context.Background(), ListJoinedChannelsOptions{
		Types:           []string{"public_channel", "private_channel", "public_channel"},
		ExcludeArchived: true,
		Limit:           3,
	})
	if err != nil {
		t.Fatalf("ListJoinedChannels() error = %v", err)
	}
	if !resp.OK || resp.Count != 3 || resp.Sort != ChannelSortNameAsc || resp.NextCursor != "" {
		t.Fatalf("response = %+v", resp)
	}
	wantNames := []string{"alpha", "beta", "zeta"}
	if len(resp.Names) != len(wantNames) {
		t.Fatalf("names = %+v, want %+v", resp.Names, wantNames)
	}
	for i, want := range wantNames {
		if resp.Names[i] != want || resp.Channels[i].Name != want || !resp.Channels[i].IsMember {
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

func TestListJoinedChannelsKeepsPageOvershoot(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.conversations" {
			t.Fatalf("path = %s, want /users.conversations", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C001","name":"alpha","created":10,"is_channel":true,"is_member":true},{"id":"C002","name":"beta","created":20,"is_channel":true,"is_member":true}],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ListJoinedChannels(context.Background(), ListJoinedChannelsOptions{Limit: 1})
	if err != nil {
		t.Fatalf("ListJoinedChannels() error = %v", err)
	}
	if resp.Count != 2 {
		t.Fatalf("response = %+v, want count 2 (overshoot kept, not dropped)", resp)
	}
}

func TestListJoinedChannelsValidatesInputs(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.ListJoinedChannels(context.Background(), ListJoinedChannelsOptions{}); err == nil {
		t.Fatal("ListJoinedChannels() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.ListJoinedChannels(context.Background(), ListJoinedChannelsOptions{Types: []string{"invalid"}}); err == nil {
		t.Fatal("ListJoinedChannels() error = nil, want type error")
	}
	if _, err := client.ListJoinedChannels(context.Background(), ListJoinedChannelsOptions{Sort: "updated_desc"}); err == nil {
		t.Fatal("ListJoinedChannels() error = nil, want sort error")
	}
	if _, err := client.ListJoinedChannels(context.Background(), ListJoinedChannelsOptions{Limit: maxChannelListLimit + 1}); err == nil {
		t.Fatal("ListJoinedChannels() error = nil, want limit error")
	}
}

func TestGetChannelInfo(t *testing.T) {
	t.Parallel()

	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.info" {
			t.Fatalf("path = %s, want /conversations.info", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		got = map[string]string{
			"token":               r.Form.Get("token"),
			"channel":             r.Form.Get("channel"),
			"include_num_members": r.Form.Get("include_num_members"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general","is_channel":true,"num_members":5}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.GetChannelInfo(context.Background(), GetChannelInfoOptions{
		ChannelID:         "C123",
		IncludeNumMembers: true,
	})
	if err != nil {
		t.Fatalf("GetChannelInfo() error = %v", err)
	}
	if !resp.OK || resp.Channel.ID != "C123" || resp.Channel.Name != "general" || resp.Channel.NumMembers != 5 {
		t.Fatalf("response = %+v", resp)
	}
	if got["token"] != "xoxp-test" || got["channel"] != "C123" || got["include_num_members"] != "true" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestGetChannelInfoRequiresTokenAndChannel(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.GetChannelInfo(context.Background(), GetChannelInfoOptions{ChannelID: "C123"}); err == nil {
		t.Fatal("GetChannelInfo() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.GetChannelInfo(context.Background(), GetChannelInfoOptions{}); err == nil {
		t.Fatal("GetChannelInfo() error = nil, want channel error")
	}
}

func TestGetConversationHistory(t *testing.T) {
	t.Parallel()

	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.history" {
			t.Fatalf("path = %s, want /conversations.history", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		got = map[string]string{
			"token":                r.Form.Get("token"),
			"channel":              r.Form.Get("channel"),
			"limit":                r.Form.Get("limit"),
			"cursor":               r.Form.Get("cursor"),
			"oldest":               r.Form.Get("oldest"),
			"latest":               r.Form.Get("latest"),
			"inclusive":            r.Form.Get("inclusive"),
			"include_all_metadata": r.Form.Get("include_all_metadata"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"parent","ts":"1700000000.000100","thread_ts":"1700000000.000100","reply_count":2,"reply_users":["U002"]},{"type":"message","subtype":"bot_message","bot_id":"B001","username":"mk","text":"bot","ts":"1700000001.000100"}],"has_more":true,"response_metadata":{"next_cursor":"cursor-2"}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})
	resp, err := client.GetConversationHistory(context.Background(), ConversationHistoryOptions{
		Limit:              2,
		Cursor:             "cursor-1",
		Oldest:             "1699999999.000100",
		Latest:             "1700000100.000100",
		Inclusive:          true,
		IncludeAllMetadata: true,
	})
	if err != nil {
		t.Fatalf("GetConversationHistory() error = %v", err)
	}
	if !resp.OK || resp.Count != 2 || !resp.HasMore || resp.NextCursor != "cursor-2" {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Messages[0].TS != "1700000000.000100" || resp.Messages[0].ReplyCount != 2 || len(resp.Messages[0].ReplyUsers) != 1 {
		t.Fatalf("first message = %+v", resp.Messages[0])
	}
	if got["token"] != "xoxp-test" || got["channel"] != "C123" || got["limit"] != "2" || got["cursor"] != "cursor-1" || got["inclusive"] != "1" || got["include_all_metadata"] != "1" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestGetConversationReplies(t *testing.T) {
	t.Parallel()

	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.replies" {
			t.Fatalf("path = %s, want /conversations.replies", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		got = map[string]string{
			"token":   r.Form.Get("token"),
			"channel": r.Form.Get("channel"),
			"ts":      r.Form.Get("ts"),
			"limit":   r.Form.Get("limit"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"parent","ts":"1700000000.000100","thread_ts":"1700000000.000100"},{"type":"message","user":"U002","text":"reply","ts":"1700000001.000100","thread_ts":"1700000000.000100","parent_user_id":"U001"}],"has_more":false,"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.GetConversationReplies(context.Background(), ConversationRepliesOptions{
		ChannelID: "C123",
		TS:        "1700000000.000100",
		Limit:     2,
	})
	if err != nil {
		t.Fatalf("GetConversationReplies() error = %v", err)
	}
	if !resp.OK || resp.Count != 2 || resp.HasMore {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Messages[1].ParentUser != "U001" || resp.Messages[1].ThreadTS != "1700000000.000100" {
		t.Fatalf("reply = %+v", resp.Messages[1])
	}
	if got["token"] != "xoxp-test" || got["channel"] != "C123" || got["ts"] != "1700000000.000100" || got["limit"] != "2" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestGetConversationRepliesValidatesInputs(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.GetConversationReplies(context.Background(), ConversationRepliesOptions{}); err == nil {
		t.Fatal("GetConversationReplies() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.GetConversationReplies(context.Background(), ConversationRepliesOptions{ChannelID: "C123"}); err == nil {
		t.Fatal("GetConversationReplies() error = nil, want ts error")
	}
	if _, err := client.GetConversationReplies(context.Background(), ConversationRepliesOptions{ChannelID: "C123", TS: "1700000000.000100", Limit: maxMessageListLimit + 1}); err == nil {
		t.Fatal("GetConversationReplies() error = nil, want limit error")
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

// blockOnlyMessage is a bot/app message whose content lives entirely in
// blocks and attachments, with the top-level Text left empty (as Slack
// commonly sends for Block Kit messages).
func blockOnlyMessage() slackapi.Message {
	return slackapi.Message{
		Msg: slackapi.Msg{
			Type:     "message",
			SubType:  "bot_message",
			BotID:    "B001",
			Username: "reporter",
			Blocks: slackapi.Blocks{
				BlockSet: []slackapi.Block{
					slackapi.NewHeaderBlock(slackapi.NewTextBlockObject(slackapi.PlainTextType, "Weekly Report", false, false)),
					slackapi.NewSectionBlock(slackapi.NewTextBlockObject(slackapi.MarkdownType, "Everything is green.", false, false), nil, nil),
				},
			},
			Attachments: []slackapi.Attachment{
				{Text: "Fallback attachment text", Footer: "generated by reporter"},
			},
			Timestamp: "1700000002.000100",
		},
	}
}

func TestSummarizeMessagesFallsBackToBlocksTextByDefault(t *testing.T) {
	t.Parallel()

	summaries := summarizeMessages([]slackapi.Message{blockOnlyMessage()}, false)
	if len(summaries) != 1 {
		t.Fatalf("summaries = %+v, want 1", summaries)
	}
	summary := summaries[0]
	if summary.Blocks != nil || summary.Attachments != nil {
		t.Fatalf("summary = %+v, want raw Blocks/Attachments omitted by default", summary)
	}
	wantText := "Weekly Report\nEverything is green.\nFallback attachment text\ngenerated by reporter"
	if summary.Text != wantText {
		t.Fatalf("Text = %q, want %q", summary.Text, wantText)
	}
}

func TestSummarizeMessagesIncludesRawBlocksWhenRequested(t *testing.T) {
	t.Parallel()

	summaries := summarizeMessages([]slackapi.Message{blockOnlyMessage()}, true)
	if len(summaries) != 1 {
		t.Fatalf("summaries = %+v, want 1", summaries)
	}
	summary := summaries[0]
	if summary.Blocks == nil || summary.Attachments == nil {
		t.Fatalf("summary = %+v, want raw Blocks/Attachments included", summary)
	}
}
