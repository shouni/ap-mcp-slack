package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAuthInfo(t *testing.T) {
	t.Parallel()

	var gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth.test" {
			t.Fatalf("path = %s, want /auth.test", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotToken = r.Form.Get("token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"url":"https://example.slack.com/","team":"Example","team_id":"T123","user":"bot","user_id":"U123","bot_id":"B123"}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.GetAuthInfo(context.Background())
	if err != nil {
		t.Fatalf("GetAuthInfo() error = %v", err)
	}
	if !resp.OK || resp.Team != "Example" || resp.TeamID != "T123" || resp.User != "bot" || resp.UserID != "U123" || resp.BotID != "B123" {
		t.Fatalf("response = %+v", resp)
	}
	if gotToken != "xoxp-test" {
		t.Fatalf("token sent = %q, want xoxp-test", gotToken)
	}
}

func TestGetAuthInfoRequiresToken(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.GetAuthInfo(context.Background()); err == nil {
		t.Fatal("GetAuthInfo() error = nil, want token error")
	}
}
