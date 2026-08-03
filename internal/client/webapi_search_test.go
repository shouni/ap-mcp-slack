package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchMessages(t *testing.T) {
	t.Parallel()

	var form struct {
		Query   string
		Count   string
		Page    string
		Sort    string
		SortDir string
		Token   string
		TeamID  string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search.messages" {
			t.Fatalf("path = %s, want /search.messages", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		form.Query = r.Form.Get("query")
		form.Count = r.Form.Get("count")
		form.Page = r.Form.Get("page")
		form.Sort = r.Form.Get("sort")
		form.SortDir = r.Form.Get("sort_dir")
		form.Token = r.Form.Get("token")
		form.TeamID = r.Form.Get("team_id")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"query":"deploy","messages":{"total":42,"paging":{"count":50,"total":42,"page":2,"pages":3},"matches":[
			{"type":"message","channel":{"id":"C001","name":"general","is_private":false},"user":"U001","username":"alice","ts":"1700000000.000100","text":"deploy done","permalink":"https://example.slack.com/archives/C001/p1700000000000100"},
			{"type":"message","channel":{"id":"C002","name":"secret","is_private":true},"user":"U002","username":"bob","ts":"1700000100.000200","text":"deploy failed","permalink":"https://example.slack.com/archives/C002/p1700000100000200?thread_ts=1700000000.000900&cid=C002"}
		]}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})
	resp, err := client.SearchMessages(context.Background(), SearchMessagesOptions{
		Query:         "  deploy  ",
		Limit:         50,
		Page:          2,
		Sort:          "TIMESTAMP",
		SortDirection: "Asc",
		TeamID:        "T123",
	})
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}

	if form.Token != "xoxp-test" || form.Query != "deploy" || form.Count != "50" || form.Page != "2" ||
		form.Sort != SearchSortTimestamp || form.SortDir != SearchSortDirAsc || form.TeamID != "T123" {
		t.Fatalf("request form = %+v", form)
	}
	if !resp.OK || resp.Query != "deploy" || resp.Count != 2 || resp.Total != 42 ||
		resp.Page != 2 || resp.PageCount != 3 || !resp.HasMore || resp.NextPage != 3 ||
		resp.Sort != SearchSortTimestamp || resp.SortDir != SearchSortDirAsc {
		t.Fatalf("response = %+v", resp)
	}

	first := resp.Matches[0]
	if first.ChannelID != "C001" || first.ChannelName != "general" || first.IsPrivate ||
		first.User != "U001" || first.Username != "alice" || first.Text != "deploy done" ||
		first.TS != "1700000000.000100" || first.ThreadTS != "" {
		t.Fatalf("first match = %+v", first)
	}
	// A thread reply is only distinguishable as one via its permalink, and the parent's
	// ts recovered from there is what get_slack_thread_replies needs.
	second := resp.Matches[1]
	if second.ChannelID != "C002" || !second.IsPrivate || second.ThreadTS != "1700000000.000900" {
		t.Fatalf("second match = %+v", second)
	}
	if first.Blocks != nil || first.Attachments != nil {
		t.Fatalf("raw blocks returned without include_raw_blocks: %+v", first)
	}
}

func TestSearchMessagesDefaultsToFirstPageByScore(t *testing.T) {
	t.Parallel()

	// slack-go omits sort/sort_dir/count/page from the request when they equal Slack's
	// own defaults, so the assertion here is that nothing unexpected is sent and the
	// applied values still come back in the response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.Form.Get("highlight"); got != "" {
			t.Fatalf("highlight = %q, want unset", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"query":"deploy","messages":{"total":0,"paging":{"count":20,"total":0,"page":0,"pages":0},"matches":[]}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})
	resp, err := client.SearchMessages(context.Background(), SearchMessagesOptions{Query: "deploy"})
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	// No match means Slack returns an empty paging block; page must still report the
	// page that was actually asked for rather than 0.
	if resp.Count != 0 || resp.Page != 1 || resp.PageCount != 0 || resp.HasMore || resp.NextPage != 0 ||
		resp.Sort != SearchSortScore || resp.SortDir != SearchSortDirDesc {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Matches == nil {
		t.Fatalf("matches = nil, want empty slice")
	}
}

func TestSearchMessagesFallsBackToBlockText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"query":"deploy","messages":{"total":1,"paging":{"count":20,"total":1,"page":1,"pages":1},"matches":[
			{"type":"message","channel":{"id":"C001","name":"general"},"user":"U001","ts":"1700000000.000100","text":"","blocks":[{"type":"section","text":{"type":"mrkdwn","text":"deploy finished"}}],"permalink":"https://example.slack.com/archives/C001/p1700000000000100"}
		]}}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})
	resp, err := client.SearchMessages(context.Background(), SearchMessagesOptions{Query: "deploy", IncludeRawBlocks: true})
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if resp.Count != 1 || resp.Matches[0].Text != "deploy finished" {
		t.Fatalf("matches = %+v", resp.Matches)
	}
	if resp.Matches[0].Blocks == nil {
		t.Fatalf("blocks = nil with include_raw_blocks, match = %+v", resp.Matches[0])
	}
	if resp.HasMore || resp.NextPage != 0 {
		t.Fatalf("response = %+v, want no further pages", resp)
	}
}

func TestSearchMessagesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    SearchMessagesOptions
		wantErr string
	}{
		{name: "empty query", opts: SearchMessagesOptions{Query: "   "}, wantErr: "query is required"},
		{name: "negative limit", opts: SearchMessagesOptions{Query: "x", Limit: -1}, wantErr: "limit must be greater than 0"},
		{name: "limit above max", opts: SearchMessagesOptions{Query: "x", Limit: maxSearchLimit + 1}, wantErr: "limit must be 100 or less"},
		{name: "negative page", opts: SearchMessagesOptions{Query: "x", Page: -1}, wantErr: "page must be 1 or greater"},
		{name: "page above max", opts: SearchMessagesOptions{Query: "x", Page: maxSearchPage + 1}, wantErr: "page must be 100 or less"},
		{name: "unsupported sort", opts: SearchMessagesOptions{Query: "x", Sort: "relevance"}, wantErr: `unsupported sort "relevance"`},
		{name: "unsupported sort_dir", opts: SearchMessagesOptions{Query: "x", SortDirection: "sideways"}, wantErr: `unsupported sort_dir "sideways"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected request to %s: input should be rejected before calling Slack", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true,"messages":{"matches":[]}}`))
			}))
			defer server.Close()

			client := NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})
			_, err := client.SearchMessages(context.Background(), tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("SearchMessages() error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestSearchMessagesRequiresToken(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.SearchMessages(context.Background(), SearchMessagesOptions{Query: "deploy"}); err == nil {
		t.Fatal("SearchMessages() error = nil, want token required")
	}
}

// TestSearchMessagesSurfacesAPIError pins that a Slack-side refusal (a bot token, most
// commonly, which search.messages does not accept) is reported rather than turned into
// an empty result set: "we were not allowed to look" must not read as "nothing matched".
func TestSearchMessagesSurfacesAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"not_allowed_token_type"}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{Token: "xoxb-test", APIBaseURL: server.URL})
	_, err := client.SearchMessages(context.Background(), SearchMessagesOptions{Query: "deploy"})
	if err == nil || !strings.Contains(err.Error(), "not_allowed_token_type") {
		t.Fatalf("SearchMessages() error = %v, want not_allowed_token_type", err)
	}
}
