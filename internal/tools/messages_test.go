package tools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

// TestPostSlackMessageWithoutConfirmReturnsPreview pins the confirm gate on the
// webhook path: an unconfirmed call must send nothing and still hand back the exact
// payload a confirmed one would deliver, source-label footer included. There is no
// separate preview tool, so this response is the only preview a caller ever sees.
func TestPostSlackMessageWithoutConfirmReturnsPreview(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{
		WebhookURL:  "https://hooks.slack.com/services/T/B/X",
		SourceLabel: "ap-mcp-slack (MCP) 経由",
	})

	var out PostSlackMessageOutput
	result := callTool(t, session, "post_slack_message", map[string]any{
		"text": "*hello*",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Posted || out.Payload.Text != "*hello*" {
		t.Fatalf("out = %+v, want a preview-only, unposted response", out)
	}
	if len(out.Payload.Blocks) != 2 {
		t.Fatalf("Blocks = %+v, want section+context (source label)", out.Payload.Blocks)
	}
}

func TestPostSlackMessageMentionsArePrependedToText(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{
		WebhookURL: "https://hooks.slack.com/services/T/B/X",
	})

	var out PostSlackMessageOutput
	result := callTool(t, session, "post_slack_message", map[string]any{
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

func TestPostSlackMessageRequiresText(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"})

	result := callTool(t, session, "post_slack_message", map[string]any{}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want error for missing text")
	}
}

func TestPostSlackMessageRejectsMentionsWithBlocks(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"})

	result := callTool(t, session, "post_slack_message", map[string]any{
		"text":     "hello",
		"mentions": []string{"U001"},
		"blocks": []map[string]any{
			{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": "hello"}},
		},
	}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want mentions+blocks rejection")
	}
}

func TestPostSlackMessageAsUserPreviewResolvesMentionsAndThreadParent(t *testing.T) {
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

	var out PostSlackMessageAsUserOutput
	result := callTool(t, session, "post_slack_message_as_user", map[string]any{
		"text":      "*hello*",
		"mentions":  []string{"U001"},
		"thread_ts": "1700000000.000100",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if out.Posted || out.ChannelName != "general" {
		t.Fatalf("out.Posted = %v, out.ChannelName = %q, want an unposted preview for #general", out.Posted, out.ChannelName)
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

func TestPostSlackMessageAsUserRequiresChannel(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test"})

	result := callTool(t, session, "post_slack_message_as_user", map[string]any{"text": "hello"}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want channel_id error")
	}
}

// TestPostSlackMessageAsUserWithoutConfirmDoesNotPost pins the confirm gate on the Web
// API path, including the source-label footer the preview must show: this response is
// the only preview a caller gets, so anything missing from it is something a human
// approves without seeing.
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
		SourceLabel:      "ap-mcp-slack (MCP) 経由",
	})

	var out PostSlackMessageAsUserOutput
	result := callTool(t, session, "post_slack_message_as_user", map[string]any{
		"text": "*hello*",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || out.Posted || out.ChannelID != "C123" || out.ChannelName != "general" || out.TS != "" {
		t.Fatalf("out = %+v, want a preview-only, unposted response", out)
	}
	if out.Payload.Text != "*hello*" || len(out.Payload.Blocks) != 2 {
		t.Fatalf("out.Payload = %+v, want the text plus section+context (source label)", out.Payload)
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

// targetSession starts a Slack API stub that can resolve a channel and the message at
// targetTS, and returns a session against it plus a counter of the write calls
// (chat.update / chat.delete) it received, so tests can assert that an unconfirmed
// call performed no write at all.
func targetSession(t *testing.T, targetTS string) (*mcp.ClientSession, *atomic.Int32) {
	t.Helper()

	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.info":
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general"}}`))
		case "/conversations.history":
			_, _ = w.Write(fmt.Appendf(nil,
				`{"ok":true,"messages":[{"type":"message","user":"U001","text":"original body","ts":%q}],"has_more":false}`,
				targetTS))
		case "/chat.update":
			writes.Add(1)
			_, _ = w.Write(fmt.Appendf(nil, `{"ok":true,"channel":"C123","ts":%q,"text":"*updated*"}`, targetTS))
		case "/chat.delete":
			writes.Add(1)
			_, _ = w.Write(fmt.Appendf(nil, `{"ok":true,"channel":"C123","ts":%q}`, targetTS))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			_, _ = w.Write([]byte(`{"ok":false,"error":"unexpected"}`))
		}
	}))
	t.Cleanup(server.Close)

	return newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	}), &writes
}

func TestUpdateSlackMessageWithoutConfirmDoesNotUpdate(t *testing.T) {
	t.Parallel()

	const ts = "1700000000.000100"
	session, writes := targetSession(t, ts)

	var out UpdateSlackMessageOutput
	result := callTool(t, session, "update_slack_message", map[string]any{
		"ts":   ts,
		"text": "*updated*",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if out.Updated || writes.Load() != 0 {
		t.Fatalf("out.Updated = %v, chat.update calls = %d, want an unconfirmed no-op", out.Updated, writes.Load())
	}
	if out.Current == nil || out.Current.Text != "original body" {
		t.Fatalf("out.Current = %+v, want the pre-update body", out.Current)
	}
	if out.ChannelName != "general" || out.Text != "*updated*" {
		t.Fatalf("out = %+v, want the resolved channel and the proposed new text", out)
	}
}

func TestUpdateSlackMessage(t *testing.T) {
	t.Parallel()

	const ts = "1700000000.000100"
	session, writes := targetSession(t, ts)

	var out UpdateSlackMessageOutput
	result := callTool(t, session, "update_slack_message", map[string]any{
		"ts":      ts,
		"text":    "*updated*",
		"confirm": true,
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || !out.Updated || out.ChannelID != "C123" || out.TS != ts || out.Text != "*updated*" {
		t.Fatalf("out = %+v", out)
	}
	if writes.Load() != 1 {
		t.Fatalf("chat.update calls = %d, want 1", writes.Load())
	}
}

// TestUpdateSlackMessagePreviewShowsBlocksOnlyReplacement covers a blocks-only update:
// text is empty, so the resolved payload is the only thing that can tell a human what
// the message is about to be overwritten with. Reporting just text here would ask them
// to approve an apparently blank replacement.
func TestUpdateSlackMessagePreviewShowsBlocksOnlyReplacement(t *testing.T) {
	t.Parallel()

	const ts = "1700000000.000100"
	session, writes := targetSession(t, ts)

	var out UpdateSlackMessageOutput
	result := callTool(t, session, "update_slack_message", map[string]any{
		"ts": ts,
		"blocks": []map[string]any{
			{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": "new body"}},
		},
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if out.Updated || writes.Load() != 0 {
		t.Fatalf("out.Updated = %v, chat.update calls = %d, want an unconfirmed no-op", out.Updated, writes.Load())
	}
	if out.Current == nil || out.Current.Text != "original body" {
		t.Fatalf("out.Current = %+v, want the pre-update body", out.Current)
	}
	if len(out.Payload.Blocks) != 1 {
		t.Fatalf("out.Payload.Blocks = %+v, want the replacement blocks", out.Payload.Blocks)
	}
	if got := out.Payload.Blocks[0]["type"]; got != "section" {
		t.Fatalf("out.Payload.Blocks[0] = %+v, want the section block passed in", out.Payload.Blocks[0])
	}
}

func TestUpdateSlackMessageAllowsAttachmentsOnly(t *testing.T) {
	t.Parallel()

	const ts = "1700000000.000100"
	session, writes := targetSession(t, ts)

	result := callTool(t, session, "update_slack_message", map[string]any{
		"ts":      ts,
		"confirm": true,
		"attachments": []map[string]any{
			{"fallback": "fallback text", "text": "attachment text"},
		},
	}, nil)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, want attachments-only update to succeed, content = %+v", result.Content)
	}
	if writes.Load() != 1 {
		t.Fatalf("chat.update calls = %d, want 1", writes.Load())
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

func TestUpdateSlackMessageRequiresContent(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test", DefaultChannelID: "C123"})

	result := callTool(t, session, "update_slack_message", map[string]any{"ts": "1700000000.000100"}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want text/blocks/attachments required error")
	}
}

func TestDeleteSlackMessageWithoutConfirmDoesNotDelete(t *testing.T) {
	t.Parallel()

	const ts = "1700000000.000100"
	session, writes := targetSession(t, ts)

	var out DeleteSlackMessageOutput
	result := callTool(t, session, "delete_slack_message", map[string]any{"ts": ts}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if out.Deleted || writes.Load() != 0 {
		t.Fatalf("out.Deleted = %v, chat.delete calls = %d, want an unconfirmed no-op", out.Deleted, writes.Load())
	}
	if out.Target == nil || out.Target.Text != "original body" {
		t.Fatalf("out.Target = %+v, want the message that would be deleted", out.Target)
	}
	if out.ChannelName != "general" {
		t.Fatalf("out.ChannelName = %q, want general", out.ChannelName)
	}
}

func TestDeleteSlackMessage(t *testing.T) {
	t.Parallel()

	const ts = "1700000000.000100"
	session, writes := targetSession(t, ts)

	var out DeleteSlackMessageOutput
	result := callTool(t, session, "delete_slack_message", map[string]any{
		"ts":      ts,
		"confirm": true,
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, content = %+v", result.Content)
	}
	if !out.OK || !out.Deleted || out.ChannelID != "C123" || out.TS != ts {
		t.Fatalf("out = %+v", out)
	}
	if writes.Load() != 1 {
		t.Fatalf("chat.delete calls = %d, want 1", writes.Load())
	}
}

// TestDeleteSlackMessagePreviewReportsUnreadableTarget covers the token that may
// delete but not read history: the delete must stay available, with the missing old
// content called out rather than silently rendered as an empty message.
func TestDeleteSlackMessagePreviewReportsUnreadableTarget(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.info":
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general"}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":false,"error":"missing_scope"}`))
		}
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:            "xoxp-test",
		DefaultChannelID: "C123",
		APIBaseURL:       server.URL,
	})

	var out DeleteSlackMessageOutput
	result := callTool(t, session, "delete_slack_message", map[string]any{"ts": "1700000000.000100"}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, want a preview despite the unreadable target, content = %+v", result.Content)
	}
	if out.Target != nil || out.TargetNote == "" {
		t.Fatalf("out = %+v, want no target and an explanatory note", out)
	}
}

// TestDeleteSlackMessageWithoutChannelReadScope covers a token that may delete but can
// read neither the message nor its channel's name: chat.delete needs no read scope, so
// the delete has to stay available, with both gaps called out. Failing the whole call
// would leave the caller with a message they can delete through no tool here.
func TestDeleteSlackMessageWithoutChannelReadScope(t *testing.T) {
	t.Parallel()

	var deletes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/chat.delete":
			deletes.Add(1)
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
		default:
			// conversations.info and conversations.history alike are out of scope.
			_, _ = w.Write([]byte(`{"ok":false,"error":"missing_scope"}`))
		}
	}))
	defer server.Close()

	session := newTestSession(t, client.SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})

	var out DeleteSlackMessageOutput
	result := callTool(t, session, "delete_slack_message", map[string]any{
		"channel_id": "C123",
		"ts":         "1700000000.000100",
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, want a preview despite the unreadable channel, content = %+v", result.Content)
	}
	if out.Deleted || deletes.Load() != 0 {
		t.Fatalf("out.Deleted = %v, chat.delete calls = %d, want an unconfirmed no-op", out.Deleted, deletes.Load())
	}
	if out.ChannelID != "C123" || out.ChannelName != "" {
		t.Fatalf("out = %+v, want the raw channel_id and no resolved name", out)
	}
	if !strings.Contains(out.TargetNote, "チャンネル名") {
		t.Fatalf("out.TargetNote = %q, want the unresolved-channel warning", out.TargetNote)
	}

	result = callTool(t, session, "delete_slack_message", map[string]any{
		"channel_id": "C123",
		"ts":         "1700000000.000100",
		"confirm":    true,
	}, &out)
	if result.IsError {
		t.Fatalf("CallTool() IsError = true, want the delete to go through, content = %+v", result.Content)
	}
	if !out.Deleted || deletes.Load() != 1 {
		t.Fatalf("out.Deleted = %v, chat.delete calls = %d, want 1 delete", out.Deleted, deletes.Load())
	}
}

// TestDeleteSlackMessageRequiresResolvableChannel pins the one case that stays an
// error: with no channel_id argument and no default channel there is no destination to
// preview or delete from.
func TestDeleteSlackMessageRequiresResolvableChannel(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, client.SlackClientConfig{Token: "xoxp-test"})

	result := callTool(t, session, "delete_slack_message", map[string]any{"ts": "1700000000.000100"}, nil)
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want channel_id error")
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
