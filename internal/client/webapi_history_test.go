package client

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	message, err := client.GetMessage(context.Background(), "C123", ts)
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
	message, err := client.GetMessage(context.Background(), "C123", replyTS)
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
	message, err := client.GetMessage(context.Background(), "C123", "1700000000.000100")
	if err != nil {
		t.Fatalf("GetMessage() error = %v, want a nil message and no error", err)
	}
	if message != nil {
		t.Fatalf("message = %+v, want nil", message)
	}
}

func TestGetMessageRequiresTokenAndTS(t *testing.T) {
	t.Parallel()

	if _, err := NewSlackClientWithConfig(SlackClientConfig{}).GetMessage(context.Background(), "C123", "1.1"); err == nil {
		t.Fatal("GetMessage() error = nil without a token, want error")
	}
	client := NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"})
	if _, err := client.GetMessage(context.Background(), "C123", "  "); err == nil {
		t.Fatal("GetMessage() error = nil for blank ts, want error")
	}
	if _, err := client.GetMessage(context.Background(), "", "1.1"); err == nil {
		t.Fatal("GetMessage() error = nil without a channel, want error")
	}
}
