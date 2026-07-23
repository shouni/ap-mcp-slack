package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

func TestResolveMentions(t *testing.T) {
	t.Parallel()

	var gotUsers []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.info" {
			t.Fatalf("path = %s, want /users.info", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		user := r.Form.Get("user")
		gotUsers = append(gotUsers, user)
		w.Header().Set("Content-Type", "application/json")
		switch user {
		case "U001":
			_, _ = w.Write([]byte(`{"ok":true,"user":{"id":"U001","name":"alice","real_name":"Alice A","profile":{"display_name":"Ali"}}}`))
		case "U002":
			_, _ = w.Write([]byte(`{"ok":true,"user":{"id":"U002","name":"bob","real_name":"Bob B","profile":{"display_name":"Bobby"}}}`))
		default:
			t.Fatalf("unexpected user %q", user)
		}
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	mentions, err := client.ResolveMentions(context.Background(), []string{"U001", " ", "U002"})
	if err != nil {
		t.Fatalf("ResolveMentions() error = %v", err)
	}
	if len(mentions) != 2 {
		t.Fatalf("mentions = %+v, want 2 (blank entry skipped)", mentions)
	}
	if mentions[0].ID != "U001" || mentions[0].DisplayName != "Ali" || mentions[0].Mention != "<@U001>" {
		t.Fatalf("mentions[0] = %+v", mentions[0])
	}
	if mentions[1].ID != "U002" || mentions[1].DisplayName != "Bobby" || mentions[1].Mention != "<@U002>" {
		t.Fatalf("mentions[1] = %+v", mentions[1])
	}
	if len(gotUsers) != 2 {
		t.Fatalf("requests = %+v, want 2 users.info calls", gotUsers)
	}
}

func TestResolveMentionsRequiresToken(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{})
	if _, err := client.ResolveMentions(context.Background(), []string{"U001"}); err == nil {
		t.Fatal("ResolveMentions() error = nil, want token error")
	}
}
