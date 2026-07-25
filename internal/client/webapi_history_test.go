package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"
)

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

// TestSummarizeMessagesExtractsRichTextTree covers the rich_text walk, which is the
// format the current Slack clients actually produce — so it is what most real messages
// with an empty top-level text hit. It goes through the API stub rather than hand-built
// slackapi values so the block decoding is exercised too: an element type slack-go
// decodes into something other than what the walk switches on would silently yield empty
// text, and building the values in Go would hide exactly that.
func TestSummarizeMessagesExtractsRichTextTree(t *testing.T) {
	t.Parallel()

	const richTextMessage = `{"ok":true,"messages":[{
	 "type":"message","user":"U001","ts":"1700000000.000100","text":"",
	 "blocks":[
	  {"type":"rich_text","elements":[
	    {"type":"rich_text_section","elements":[
	      {"type":"text","text":"Deploy finished for "},
	      {"type":"link","url":"https://example.test/build/42","text":"build 42"},
	      {"type":"text","text":" — "},
	      {"type":"user","user_id":"U999"},
	      {"type":"link","url":"https://example.test/bare"}
	    ]},
	    {"type":"rich_text_list","style":"bullet","elements":[
	      {"type":"rich_text_section","elements":[{"type":"text","text":"api: ok"}]},
	      {"type":"rich_text_section","elements":[{"type":"text","text":"web: ok"}]}
	    ]},
	    {"type":"rich_text_quote","elements":[{"type":"text","text":"quoted line"}]},
	    {"type":"rich_text_preformatted","elements":[{"type":"text","text":"$ make deploy"}]}
	  ]}
	 ]}],"has_more":false}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(richTextMessage))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.GetConversationHistory(context.Background(), ConversationHistoryOptions{ChannelID: "C123"})
	if err != nil {
		t.Fatalf("GetConversationHistory() error = %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("messages = %+v, want 1", resp.Messages)
	}

	// A link renders as its display text, or its URL when it has none; a user mention
	// is skipped, since this is a best-effort fallback and not a rich_text renderer.
	// Lists, quotes and code blocks each contribute a line.
	wantText := "Deploy finished for build 42 — https://example.test/bare\napi: ok\nweb: ok\nquoted line\n$ make deploy"
	if resp.Messages[0].Text != wantText {
		t.Fatalf("Text = %q, want %q", resp.Messages[0].Text, wantText)
	}
}

// TestSummarizeMessagesExtractsRemainingBlockKinds covers the block and attachment
// shapes the rich_text and header/section cases leave out, so every branch that decides
// whether a message's content survives into Text is pinned.
func TestSummarizeMessagesExtractsRemainingBlockKinds(t *testing.T) {
	t.Parallel()

	const message = `{"ok":true,"messages":[{
	 "type":"message","bot_id":"B001","ts":"1700000000.000100","text":"",
	 "blocks":[
	  {"type":"section","fields":[
	    {"type":"mrkdwn","text":"*Env*\nprod"},
	    {"type":"mrkdwn","text":"*Status*\ngreen"}
	  ]},
	  {"type":"image","image_url":"https://example.test/titled.png","alt_text":"alt ignored","title":{"type":"plain_text","text":"Titled chart"}},
	  {"type":"image","image_url":"https://example.test/untitled.png","alt_text":"alt text used"},
	  {"type":"context","elements":[{"type":"mrkdwn","text":"context footer"}]}
	 ],
	 "attachments":[
	  {"pretext":"pretext line","title":"attachment title","text":"attachment body","footer":"attachment footer",
	   "fields":[{"title":"Field","value":"value"}]},
	  {"fallback":"fallback only"}
	 ]}],"has_more":false}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(message))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	resp, err := client.GetConversationHistory(context.Background(), ConversationHistoryOptions{ChannelID: "C123"})
	if err != nil {
		t.Fatalf("GetConversationHistory() error = %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("messages = %+v, want 1", resp.Messages)
	}

	// An image contributes its title, or its alt text when untitled; an attachment
	// contributes its text, or its fallback when it has no text.
	wantLines := []string{
		"*Env*\nprod",
		"*Status*\ngreen",
		"Titled chart",
		"alt text used",
		"context footer",
		"pretext line",
		"attachment title",
		"attachment body",
		"Field: value",
		"attachment footer",
		"fallback only",
	}
	wantText := strings.Join(wantLines, "\n")
	if resp.Messages[0].Text != wantText {
		t.Fatalf("Text = %q, want %q", resp.Messages[0].Text, wantText)
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

func TestGetMessageFindsTopLevelMessage(t *testing.T) {
	t.Parallel()

	const ts = "1700000000.000100"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.history" {
			t.Fatalf("path = %s, want conversations.history only", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"target","ts":"` + ts + `"}],"has_more":false}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	message, _, err := client.GetMessage(context.Background(), "C123", ts)
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if message == nil || message.TS != ts || message.Text != "target" {
		t.Fatalf("message = %+v, want the message at ts", message)
	}
}

// TestGetMessageFallsBackToRepliesForThreadReply is the reason GetMessage compares
// timestamps instead of trusting the first history result. conversations.history
// carries only top-level messages, so for a thread reply Slack answers with the
// nearest older parent — which, shown above a delete confirmation, would describe the
// wrong message entirely.
func TestGetMessageFallsBackToRepliesForThreadReply(t *testing.T) {
	t.Parallel()

	const (
		parentTS = "1700000000.000100"
		replyTS  = "1700000009.000200"
	)
	var repliesCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.history":
			_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"unrelated parent","ts":"` + parentTS + `"}],"has_more":false}`))
		case "/conversations.replies":
			repliesCalled = true
			_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"unrelated parent","ts":"` + parentTS + `"},{"type":"message","user":"U002","text":"the reply","ts":"` + replyTS + `"}],"has_more":false}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	message, _, err := client.GetMessage(context.Background(), "C123", replyTS)
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if !repliesCalled {
		t.Fatal("conversations.replies was not called, want a fallback lookup")
	}
	if message == nil || message.TS != replyTS || message.Text != "the reply" {
		t.Fatalf("message = %+v, want the reply at %s, not the parent", message, replyTS)
	}
}

func TestGetMessageReportsMissingMessageAsNil(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"messages":[],"has_more":false}`))
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	message, truncated, err := client.GetMessage(context.Background(), "C123", "1700000000.000100")
	if err != nil {
		t.Fatalf("GetMessage() error = %v, want a nil message and no error", err)
	}
	if message != nil {
		t.Fatalf("message = %+v, want nil", message)
	}
	if truncated {
		t.Fatal("truncated = true, want false: the thread was exhausted, not given up on")
	}
}

// TestGetMessageFollowsRepliesCursorForDeepThreadReply covers a reply past the first
// page of its thread. conversations.replies starts at the thread's beginning whatever ts
// it is handed, so reading a single page would report a perfectly valid ts as missing —
// and the note above a delete confirmation would tell the caller to re-check a ts that
// is fine, for a delete that would have succeeded.
func TestGetMessageFollowsRepliesCursorForDeepThreadReply(t *testing.T) {
	t.Parallel()

	const (
		parentTS = "1700000000.000100"
		targetTS = "1700000900.000900"
	)
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.history":
			_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"unrelated parent","ts":"` + parentTS + `"}],"has_more":false}`))
		case "/conversations.replies":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			cursor := r.Form.Get("cursor")
			cursors = append(cursors, cursor)
			switch cursor {
			case "":
				_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U001","text":"parent","ts":"` + parentTS + `"}],"has_more":true,"response_metadata":{"next_cursor":"page-2"}}`))
			case "page-2":
				_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U002","text":"a middle reply","ts":"1700000500.000500"}],"has_more":true,"response_metadata":{"next_cursor":"page-3"}}`))
			case "page-3":
				_, _ = w.Write([]byte(`{"ok":true,"messages":[{"type":"message","user":"U003","text":"the deep reply","ts":"` + targetTS + `"}],"has_more":false}`))
			default:
				t.Fatalf("unexpected cursor %q", cursor)
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	message, truncated, err := client.GetMessage(context.Background(), "C123", targetTS)
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if truncated {
		t.Fatal("truncated = true, want false: the reply was found within the page budget")
	}
	if message == nil || message.TS != targetTS || message.Text != "the deep reply" {
		t.Fatalf("message = %+v, want the reply at %s from the third page", message, targetTS)
	}
	if len(cursors) != 3 {
		t.Fatalf("replies cursors = %v, want three pages followed", cursors)
	}
}

// TestGetMessageReportsTruncatedSearchForVeryDeepReply pins that giving up is reported
// as such rather than as "no such message": the ts may be valid and the update/delete
// may well succeed, so the caller must not be sent off to re-check it.
func TestGetMessageReportsTruncatedSearchForVeryDeepReply(t *testing.T) {
	t.Parallel()

	var pages int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.history":
			_, _ = w.Write([]byte(`{"ok":true,"messages":[],"has_more":false}`))
		case "/conversations.replies":
			pages++
			// Always another page, and never the requested ts.
			_, _ = fmt.Fprintf(w, `{"ok":true,"messages":[{"type":"message","user":"U001","text":"filler","ts":"170000%04d.000100"}],"has_more":true,"response_metadata":{"next_cursor":"page-%d"}}`, pages, pages+1)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewSlackClientWithConfig(SlackClientConfig{
		Token:      "xoxp-test",
		APIBaseURL: server.URL,
	})
	message, truncated, err := client.GetMessage(context.Background(), "C123", "1799999999.999999")
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if message != nil {
		t.Fatalf("message = %+v, want nil", message)
	}
	if !truncated {
		t.Fatal("truncated = false, want true when the page budget ran out")
	}
	if pages != maxMessageSearchPages {
		t.Fatalf("replies pages = %d, want the walk bounded at %d", pages, maxMessageSearchPages)
	}
}

func TestGetMessageRequiresTokenAndTS(t *testing.T) {
	t.Parallel()

	if _, _, err := NewSlackClientWithConfig(SlackClientConfig{}).GetMessage(context.Background(), "C123", "1.1"); err == nil {
		t.Fatal("GetMessage() error = nil without a token, want error")
	}
	client := NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, _, err := client.GetMessage(context.Background(), "C123", "  "); err == nil {
		t.Fatal("GetMessage() error = nil for blank ts, want error")
	}
	if _, _, err := client.GetMessage(context.Background(), "", "1.1"); err == nil {
		t.Fatal("GetMessage() error = nil without a channel, want error")
	}
}
