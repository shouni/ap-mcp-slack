package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shouni/go-http-kit/httpkit"
)

// newTestWebhookClient builds a SlackClient whose webhook transport skips
// go-http-kit's SSRF/DNS-rebinding validation, for tests that point webhookURL at a
// loopback httptest server. This lives here rather than as a SlackClientConfig field
// so production callers have no way to disable that validation.
//
// webAPITransport is initialized through the normal (empty-config) constructor
// rather than left as a zero value: every webAPITransport method checks
// requireToken() before touching slackAPIClient, so today a nil client field
// wouldn't actually panic, but a real, well-formed value keeps that true even if a
// future caller extends this helper to exercise Web API methods too.
func newTestWebhookClient(webhookURL string) *SlackClient {
	return &SlackClient{
		webhookTransport: webhookTransport{
			webhookURL:    webhookURL,
			httpKitClient: httpkit.New(requestTimeout, httpkit.WithNoRetry(), httpkit.WithSkipNetworkValidation(true)),
		},
		webAPITransport: newWebAPITransport(SlackClientConfig{}),
	}
}

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

	client := newTestWebhookClient(server.URL)
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

	client := newTestWebhookClient(server.URL)
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

func TestListUsers(t *testing.T) {
	t.Parallel()

	type request struct {
		Cursor string
		Limit  string
		Token  string
	}
	var requests []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.list" {
			t.Fatalf("path = %s, want /users.list", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		requests = append(requests, request{
			Cursor: r.Form.Get("cursor"),
			Limit:  r.Form.Get("limit"),
			Token:  r.Form.Get("token"),
		})

		w.Header().Set("Content-Type", "application/json")
		switch r.Form.Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"ok":true,"members":[{"id":"U002","name":"zeta","real_name":"Zeta Z"},{"id":"U001","name":"alpha","real_name":"Alpha A"}],"response_metadata":{"next_cursor":"cursor-2"}}`))
		case "cursor-2":
			_, _ = w.Write([]byte(`{"ok":true,"members":[{"id":"U003","name":"beta","real_name":"Beta B"}],"response_metadata":{"next_cursor":""}}`))
		default:
			t.Fatalf("unexpected cursor %q", r.Form.Get("cursor"))
		}
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ListUsers(context.Background(), ListUsersOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if !resp.OK || resp.Count != 3 || resp.NextCursor != "" {
		t.Fatalf("response = %+v", resp)
	}
	wantIDs := []string{"U002", "U001", "U003"}
	for i, want := range wantIDs {
		if resp.Users[i].ID != want {
			t.Fatalf("users = %+v, want id %q at %d", resp.Users, want, i)
		}
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %+v, want 2 requests", requests)
	}
	if requests[0].Token != "xoxp-test" {
		t.Fatalf("first request = %+v", requests[0])
	}
	if requests[1].Cursor != "cursor-2" {
		t.Fatalf("second request = %+v", requests[1])
	}
}

func TestListUsersExcludesDeletedByDefault(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.list" {
			t.Fatalf("path = %s, want /users.list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"members":[{"id":"U001","name":"alpha","deleted":false},{"id":"U999","name":"ghost","deleted":true},{"id":"U002","name":"beta","deleted":false}],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})

	resp, err := client.ListUsers(context.Background(), ListUsersOptions{})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if resp.Count != 2 {
		t.Fatalf("count = %d, want 2 (deleted excluded): %+v", resp.Count, resp.Users)
	}

	resp, err = client.ListUsers(context.Background(), ListUsersOptions{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if resp.Count != 3 {
		t.Fatalf("count = %d, want 3 (deleted included): %+v", resp.Count, resp.Users)
	}
}

func TestListUsersQueryFilter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"members":[{"id":"U001","name":"alice","real_name":"Alice A","profile":{"display_name":"Ali","email":"alice@example.com"}},{"id":"U002","name":"bob","real_name":"Bob B","profile":{"display_name":"Bobby","email":"bob@example.com"}}],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ListUsers(context.Background(), ListUsersOptions{Query: "ALI"})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if resp.Count != 1 || resp.Users[0].ID != "U001" {
		t.Fatalf("users = %+v, want only U001 (alice)", resp.Users)
	}
}

func TestListUsersValidatesInputs(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.ListUsers(context.Background(), ListUsersOptions{}); err == nil {
		t.Fatal("ListUsers() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.ListUsers(context.Background(), ListUsersOptions{Limit: -1}); err == nil {
		t.Fatal("ListUsers() error = nil, want limit error")
	}
	if _, err := client.ListUsers(context.Background(), ListUsersOptions{Limit: maxUserListLimit + 1}); err == nil {
		t.Fatal("ListUsers() error = nil, want limit error")
	}
}

func TestLookupUserByEmail(t *testing.T) {
	t.Parallel()

	var gotEmail string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.lookupByEmail" {
			t.Fatalf("path = %s, want /users.lookupByEmail", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotEmail = r.Form.Get("email")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"user":{"id":"U001","name":"alice","real_name":"Alice A","profile":{"display_name":"Ali","email":"alice@example.com"}}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	user, err := client.LookupUserByEmail(context.Background(), " alice@example.com ")
	if err != nil {
		t.Fatalf("LookupUserByEmail() error = %v", err)
	}
	if user.ID != "U001" || user.Email != "alice@example.com" || user.DisplayName != "Ali" {
		t.Fatalf("user = %+v", user)
	}
	if gotEmail != "alice@example.com" {
		t.Fatalf("email sent = %q, want trimmed email", gotEmail)
	}
}

func TestLookupUserByEmailNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"users_not_found"}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	if _, err := client.LookupUserByEmail(context.Background(), "nobody@example.com"); err == nil {
		t.Fatal("LookupUserByEmail() error = nil, want users_not_found error")
	}
}

func TestLookupUserByEmailRequiresTokenAndEmail(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.LookupUserByEmail(context.Background(), "alice@example.com"); err == nil {
		t.Fatal("LookupUserByEmail() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.LookupUserByEmail(context.Background(), "  "); err == nil {
		t.Fatal("LookupUserByEmail() error = nil, want email error")
	}
}

func TestResolveUserByEmail(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.lookupByEmail" {
			t.Fatalf("path = %s, want /users.lookupByEmail", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"user":{"id":"U001","name":"alice","real_name":"Alice A","profile":{"display_name":"Ali","email":"alice@example.com"}}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ResolveUser(context.Background(), "", "alice@example.com", "")
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if resp.Status != ResolveUserStatusFound || resp.User == nil || resp.User.ID != "U001" || resp.Mention != "<@U001>" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestResolveUserByEmailNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"users_not_found"}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ResolveUser(context.Background(), "", "nobody@example.com", "")
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if resp.Status != ResolveUserStatusNotFound || resp.User != nil {
		t.Fatalf("response = %+v", resp)
	}
}

func newResolveByNameTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.list" {
			t.Fatalf("path = %s, want /users.list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"members":[
			{"id":"U001","name":"alice","real_name":"Alice Smith","profile":{"display_name":"Alice"}},
			{"id":"U002","name":"alice.wong","real_name":"Alice Wong","profile":{"display_name":"AWong"}},
			{"id":"U003","name":"bob","real_name":"Bob Lee","profile":{"display_name":"Bobby"}}
		],"response_metadata":{"next_cursor":""}}`))
	}))
}

func TestResolveUserByNameExactMatch(t *testing.T) {
	t.Parallel()

	server := newResolveByNameTestServer(t)
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ResolveUser(context.Background(), "alice", "", "")
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if resp.Status != ResolveUserStatusFound || resp.User == nil || resp.User.ID != "U001" || resp.Mention != "<@U001>" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestResolveUserByNamePartialMatchIsAmbiguous(t *testing.T) {
	t.Parallel()

	server := newResolveByNameTestServer(t)
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ResolveUser(context.Background(), "ali", "", "")
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if resp.Status != ResolveUserStatusAmbiguous || len(resp.Candidates) != 2 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestResolveUserByNameNotFound(t *testing.T) {
	t.Parallel()

	server := newResolveByNameTestServer(t)
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ResolveUser(context.Background(), "zzz-nobody", "", "")
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if resp.Status != ResolveUserStatusNotFound || resp.User != nil || resp.Candidates != nil {
		t.Fatalf("response = %+v", resp)
	}
}

func TestResolveUserRequiresTokenAndNameOrEmail(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.ResolveUser(context.Background(), "alice", "", ""); err == nil {
		t.Fatal("ResolveUser() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.ResolveUser(context.Background(), " ", " ", ""); err == nil {
		t.Fatal("ResolveUser() error = nil, want name/email error")
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
