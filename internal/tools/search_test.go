package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shouni/ap-mcp-slack/internal/client"
)

func TestSearchSlackMessages(t *testing.T) {
	t.Parallel()

	var query, count string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search.messages" {
			t.Fatalf("path = %s, want /search.messages", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		query = r.Form.Get("query")
		count = r.Form.Get("count")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"query":"in:#general deploy","messages":{"total":2,"paging":{"count":5,"total":2,"page":1,"pages":1},"matches":[
			{"type":"message","channel":{"id":"C001","name":"general"},"user":"U001","ts":"1700000000.000100","text":"deploy done","permalink":"https://example.slack.com/archives/C001/p1700000000000100"}
		]}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})

	var out SearchSlackMessagesOutput
	result := callTool(t, session, "search_slack_messages", map[string]any{
		"query": "in:#general deploy",
		"limit": 5,
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if query != "in:#general deploy" || count != "5" {
		t.Fatalf("request query = %q count = %q", query, count)
	}
	if !out.OK || out.Count != 1 || out.Total != 2 || out.Matches[0].ChannelName != "general" {
		t.Fatalf("out = %+v", out)
	}
}

// TestSearchSlackMessagesRequiresQuery pins that the query is a required field of the
// tool's schema, so an omitted one is rejected by the MCP client before any Slack call
// rather than becoming a workspace-wide search for nothing.
func TestSearchSlackMessagesRequiresQuery(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s: query should be rejected by schema validation", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"messages":{"matches":[]}}`))
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", APIBaseURL: server.URL})

	result := callTool(t, session, "search_slack_messages", map[string]any{}, nil)
	if !result.IsError {
		t.Fatalf("CallTool() IsError = false, want error for missing query")
	}
}
