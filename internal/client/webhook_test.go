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
// so production callers have no way to disable that validation. That skip is the
// only intended difference from newWebhookTransport — the other options are
// repeated so a request under test carries the headers a real one would.
//
// webAPITransport is initialized through the normal (empty-config) constructor
// rather than left as a zero value: every webAPITransport method checks
// requireToken() before touching slackAPIClient, so today a nil client field
// wouldn't actually panic, but a real, well-formed value keeps that true even if a
// future caller extends this helper to exercise Web API methods too.
func newTestWebhookClient(webhookURL string) *SlackClient {
	return newTestWebhookClientWithSourceLabel(webhookURL, "")
}

func newTestWebhookClientWithSourceLabel(webhookURL, sourceLabel string) *SlackClient {
	return &SlackClient{
		webhookTransport: webhookTransport{
			webhookURL:  webhookURL,
			sourceLabel: sourceLabel,
			httpKitClient: httpkit.New(
				requestTimeout,
				httpkit.WithNoRetry(),
				httpkit.WithUserAgent(webhookUserAgent),
				httpkit.WithoutBrowserHeaders(),
				httpkit.WithSkipNetworkValidation(true),
			),
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

func TestPostMessageAppendsSourceLabelBlock(t *testing.T) {
	t.Parallel()

	var got Message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := newTestWebhookClientWithSourceLabel(server.URL, "ap-mcp-slack (MCP) 経由")
	resp, err := client.PostMessage(context.Background(), Message{Text: "*hello* <@shouni>"})
	if err != nil {
		t.Fatalf("PostMessage() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK || resp.Body != "ok" {
		t.Fatalf("response = %+v, want status 200 body ok", resp)
	}
	if len(got.Blocks) != 2 {
		t.Fatalf("Blocks length = %d, want 2", len(got.Blocks))
	}
	if got.Blocks[0]["type"] != "section" {
		t.Fatalf("first block = %+v, want section", got.Blocks[0])
	}
	if got.Blocks[1]["type"] != "context" {
		t.Fatalf("last block = %+v, want context", got.Blocks[1])
	}
	elements, ok := got.Blocks[1]["elements"].([]any)
	if !ok || len(elements) != 1 {
		t.Fatalf("context elements = %#v, want one element", got.Blocks[1]["elements"])
	}
	element, ok := elements[0].(map[string]any)
	if !ok || element["text"] != "ap-mcp-slack (MCP) 経由" {
		t.Fatalf("context element = %#v, want source label", elements[0])
	}
}

// TestPreviewMessageRequiresWebhookURL guards the confirm-gated post flow: a preview
// is what a human approves before anything is sent, so a payload that could never be
// delivered must fail while it is still being built, not after sign-off.
func TestPreviewMessageRequiresWebhookURL(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{SourceLabel: "ap-mcp-slack (MCP) 経由"})
	if _, err := client.PreviewMessage(Message{Text: "hello"}); err == nil {
		t.Fatal("PreviewMessage() error = nil, want webhook URL error")
	}
}

func TestWebhookConfigured(t *testing.T) {
	t.Parallel()

	if NewSlackClientWithConfig(SlackClientConfig{}).WebhookConfigured() {
		t.Fatal("WebhookConfigured() = true for empty config, want false")
	}
	if !NewSlackClientWithConfig(SlackClientConfig{WebhookURL: "http://example.test"}).WebhookConfigured() {
		t.Fatal("WebhookConfigured() = false with a webhook URL, want true")
	}
}

func TestWebAPIConfigured(t *testing.T) {
	t.Parallel()

	if NewSlackClientWithConfig(SlackClientConfig{}).WebAPIConfigured() {
		t.Fatal("WebAPIConfigured() = true for empty config, want false")
	}
	if !NewSlackClientWithConfig(SlackClientConfig{Token: "xoxp-test"}).WebAPIConfigured() {
		t.Fatal("WebAPIConfigured() = false with a token, want true")
	}
}

func TestPreviewMessageBuildsPayloadWithoutSending(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{
		WebhookURL:  "https://hooks.slack.com/services/T/B/X",
		SourceLabel: "ap-mcp-slack (MCP) 経由",
	})
	payload, err := client.PreviewMessage(Message{Text: "*hello* <@shouni>"})
	if err != nil {
		t.Fatalf("PreviewMessage() error = %v", err)
	}
	if payload.Text != "*hello* <@shouni>" {
		t.Fatalf("Text = %q, want input text", payload.Text)
	}
	if len(payload.Blocks) != 2 {
		t.Fatalf("Blocks length = %d, want 2", len(payload.Blocks))
	}
	if payload.Blocks[0]["type"] != "section" || payload.Blocks[1]["type"] != "context" {
		t.Fatalf("Blocks = %+v, want section and context", payload.Blocks)
	}
}

func TestPostMessageRequiresText(t *testing.T) {
	t.Parallel()

	client := NewSlackClientWithConfig(SlackClientConfig{WebhookURL: "http://example.test"})
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
