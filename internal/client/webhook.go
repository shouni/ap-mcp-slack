package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/shouni/go-http-kit/httpkit"
)

// webhookUserAgent is how this server identifies itself to Slack's webhook endpoint.
const webhookUserAgent = "ap-mcp-slack"

// webhookTransport posts messages through Slack Incoming Webhooks.
//
// Response bodies are capped by go-http-kit itself (httpkit.MaxResponseBodySize, a
// fixed 25MB), so a malicious or misbehaving webhook endpoint can't force unbounded
// memory growth; a real Slack incoming webhook only ever returns a few bytes.
type webhookTransport struct {
	webhookURL    string
	sourceLabel   string
	httpKitClient *httpkit.Client
}

func newWebhookTransport(cfg SlackClientConfig) webhookTransport {
	// Webhook posts are non-idempotent (they create a new Slack message), so retries
	// are disabled to avoid duplicate posts on transient errors. SSRF/DNS-rebinding
	// validation always stays on here; tests that need a loopback httptest server
	// build a webhookTransport literal directly rather than going through this
	// production constructor, so there's no config flag that could flip it off.
	//
	// The identity options are what stop go-http-kit's helpers from introducing
	// themselves as a browser. Its defaults exist for scraping sites that block
	// non-browser agents: PostJSONAndFetchBytes runs every request through
	// addCommonHeaders, which sets a Chrome User-Agent plus the sec-ch-ua client
	// hints. Slack's webhook endpoint is an API and asks for none of that, so we
	// name the program instead and drop the hints.
	return webhookTransport{
		webhookURL:  strings.TrimSpace(cfg.WebhookURL),
		sourceLabel: strings.TrimSpace(cfg.SourceLabel),
		httpKitClient: httpkit.New(
			requestTimeout,
			httpkit.WithNoRetry(),
			httpkit.WithUserAgent(webhookUserAgent),
			httpkit.WithoutBrowserHeaders(),
		),
	}
}

// Message is the JSON payload sent to Slack Incoming Webhooks.
type Message struct {
	Text        string           `json:"text,omitempty"`
	Blocks      []map[string]any `json:"blocks,omitempty"`
	Attachments []map[string]any `json:"attachments,omitempty"`
	ThreadTS    string           `json:"thread_ts,omitempty"`
	IconEmoji   string           `json:"icon_emoji,omitempty"`
	UnfurlLinks *bool            `json:"unfurl_links,omitempty"`
	UnfurlMedia *bool            `json:"unfurl_media,omitempty"`
}

// PostMessageResponse contains the relevant Slack webhook response details.
type PostMessageResponse struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

// WebhookConfigured reports whether an Incoming Webhook URL was configured. Tool
// registration uses it to leave the webhook tools out entirely when they could only
// ever fail, so a caller can't pick one and discover that at send time.
func (w *webhookTransport) WebhookConfigured() bool {
	return w.webhookURL != ""
}

// PreviewMessage builds the webhook payload without sending it.
//
// This checks the webhook URL even though it sends nothing: a preview whose payload
// can never actually be delivered is worse than useless, because the confirm-gated
// post flow asks a human to approve that preview first. Failing here surfaces the
// misconfiguration before anyone signs off on it, not after.
func (w *webhookTransport) PreviewMessage(msg Message) (Message, error) {
	if w.webhookURL == "" {
		return Message{}, fmt.Errorf("slack: webhook URL is required")
	}
	if strings.TrimSpace(msg.Text) == "" {
		return Message{}, fmt.Errorf("slack: text is required")
	}
	msg.Blocks = appendSourceLabelBlock(msg.Blocks, msg.Text, w.sourceLabel)
	return msg, nil
}

// PostMessage posts a message to Slack. It resolves the payload through PreviewMessage
// so what is delivered matches what a preview of the same input would have shown.
func (w *webhookTransport) PostMessage(ctx context.Context, msg Message) (*PostMessageResponse, error) {
	msg, err := w.PreviewMessage(msg)
	if err != nil {
		return nil, err
	}

	responseBody, err := w.httpKitClient.PostJSONAndFetchBytes(ctx, w.webhookURL, msg)
	if err != nil {
		return nil, fmt.Errorf("slack: post webhook: %w", err)
	}

	// go-http-kit's PostJSONAndFetchBytes abstracts away the exact 2xx status code
	// (it only surfaces non-2xx as an error), and Slack's incoming webhooks are
	// documented to respond 200 on every accepted post, so that's what we report here.
	return &PostMessageResponse{
		StatusCode: http.StatusOK,
		Body:       strings.TrimSpace(string(responseBody)),
	}, nil
}
