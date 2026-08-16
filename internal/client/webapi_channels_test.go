package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

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
	if len(resp.Channels) != len(wantNames) {
		t.Fatalf("channels = %+v, want %+v", resp.Channels, wantNames)
	}
	for i, want := range wantNames {
		if resp.Channels[i].Name != want {
			t.Fatalf("channels = %+v, want %q at %d", resp.Channels, want, i)
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
	resp, err := client.ListJoinedChannels(context.Background(), ListChannelsOptions{
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
	if len(resp.Channels) != len(wantNames) {
		t.Fatalf("channels = %+v, want %+v", resp.Channels, wantNames)
	}
	for i, want := range wantNames {
		if resp.Channels[i].Name != want || !resp.Channels[i].IsMember {
			t.Fatalf("channels = %+v, want %q at %d", resp.Channels, want, i)
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
	resp, err := client.ListJoinedChannels(context.Background(), ListChannelsOptions{Limit: 1})
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
	if _, err := client.ListJoinedChannels(context.Background(), ListChannelsOptions{}); err == nil {
		t.Fatal("ListJoinedChannels() error = nil, want token error")
	}

	client = NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.ListJoinedChannels(context.Background(), ListChannelsOptions{Types: []string{"invalid"}}); err == nil {
		t.Fatal("ListJoinedChannels() error = nil, want type error")
	}
	if _, err := client.ListJoinedChannels(context.Background(), ListChannelsOptions{Sort: "updated_desc"}); err == nil {
		t.Fatal("ListJoinedChannels() error = nil, want sort error")
	}
	if _, err := client.ListJoinedChannels(context.Background(), ListChannelsOptions{Limit: maxChannelListLimit + 1}); err == nil {
		t.Fatal("ListJoinedChannels() error = nil, want limit error")
	}
}

// TestSortChannels covers every accepted sort value, including the tie-breaks. Slack's
// conversations.list takes no sort argument, so this ordering is entirely ours: nothing
// upstream would surface a mistake in it, and a caller paging a large workspace sees only
// the order, not the comparison that produced it.
func TestSortChannels(t *testing.T) {
	t.Parallel()

	// Deliberately awkward input: two channels created at the same second (so the
	// created sorts must fall back to ID), one with no name at all (so the name sorts
	// must fall back through name_normalized/user/id), and mixed case.
	newChannels := func() []SlackChannelSummary {
		return []SlackChannelSummary{
			{ID: "C003", Name: "Zeta", Created: 300},
			{ID: "C001", Name: "alpha", Created: 100},
			{ID: "C002", NameNormalized: "beta", Created: 100},
			{ID: "C004", Created: 400},
		}
	}

	tests := []struct {
		sort    string
		wantIDs []string
	}{
		// Case-insensitive, and the unnamed channel sorts by its ID ("c004").
		{sort: ChannelSortNameAsc, wantIDs: []string{"C001", "C002", "C004", "C003"}},
		{sort: ChannelSortNameDesc, wantIDs: []string{"C003", "C004", "C002", "C001"}},
		// C001 and C002 share Created=100, so the ID breaks the tie in both directions.
		{sort: ChannelSortCreatedAsc, wantIDs: []string{"C001", "C002", "C003", "C004"}},
		{sort: ChannelSortCreatedDesc, wantIDs: []string{"C004", "C003", "C001", "C002"}},
		// "none" must leave Slack's own order untouched.
		{sort: ChannelSortNone, wantIDs: []string{"C003", "C001", "C002", "C004"}},
	}

	for _, tc := range tests {
		t.Run(tc.sort, func(t *testing.T) {
			t.Parallel()

			channels := newChannels()
			sortChannels(channels, tc.sort)

			gotIDs := make([]string, 0, len(channels))
			for _, channel := range channels {
				gotIDs = append(gotIDs, channel.ID)
			}
			if !slices.Equal(gotIDs, tc.wantIDs) {
				t.Fatalf("sort %q = %v, want %v", tc.sort, gotIDs, tc.wantIDs)
			}
		})
	}
}

// TestListChannelsAppliesSort pins that the sort reaches the response, not just
// sortChannels: the option is normalized on the way in and echoed back on the way out.
func TestListChannelsAppliesSort(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channels":[
			{"id":"C001","name":"alpha","created":100},
			{"id":"C002","name":"beta","created":300}
		],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ListChannels(context.Background(), ListChannelsOptions{Sort: " CREATED_DESC "})
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if resp.Sort != ChannelSortCreatedDesc {
		t.Fatalf("resp.Sort = %q, want the normalized %q", resp.Sort, ChannelSortCreatedDesc)
	}
	if resp.Channels[0].ID != "C002" || resp.Channels[1].ID != "C001" {
		t.Fatalf("channels = %+v, want newest first", resp.Channels)
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

// TestListChannelsReturnsPartialOnRateLimit pins what a mid-walk HTTP 429 turns into:
// the pages already fetched, the cursor of the very page Slack refused, and Slack's
// retry-after — not an error. Failing would discard fetched pages only for the retry
// to re-spend the request budget that just ran out, and the cursor guarantees the
// refused page is re-fetched rather than skipped.
func TestListChannelsReturnsPartialOnRateLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		switch r.Form.Get("cursor") {
		case "":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C001","name":"alpha","is_channel":true}],"response_metadata":{"next_cursor":"cursor-2"}}`))
		case "cursor-2":
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			t.Fatalf("unexpected cursor %q", r.Form.Get("cursor"))
		}
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.ListChannels(context.Background(), ListChannelsOptions{})
	if err != nil {
		t.Fatalf("ListChannels() error = %v, want the fetched pages back", err)
	}
	if resp.Count != 1 || resp.Channels[0].ID != "C001" {
		t.Fatalf("response = %+v, want the one channel fetched before the rate limit", resp)
	}
	if resp.NextCursor != "cursor-2" {
		t.Fatalf("next_cursor = %q, want cursor-2 so the refused page is re-fetched, not skipped", resp.NextCursor)
	}
	if resp.RetryAfterSeconds != 30 {
		t.Fatalf("retry_after_seconds = %d, want 30 from Slack's Retry-After", resp.RetryAfterSeconds)
	}
}

// TestListChannelsFirstPageRateLimitIsError pins the other half of the rate-limit
// contract: with nothing fetched yet there is no partial result worth returning, so
// the 429 surfaces as an error whose message carries Slack's retry-after.
func TestListChannelsFirstPageRateLimitIsError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	_, err := client.ListChannels(context.Background(), ListChannelsOptions{})
	if err == nil {
		t.Fatal("ListChannels() error = nil, want a rate-limit error")
	}
	if !strings.Contains(err.Error(), "retry after") {
		t.Fatalf("error = %v, want Slack's retry-after in the message", err)
	}
}
