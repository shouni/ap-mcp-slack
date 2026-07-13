package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/shouni/go-http-kit/httpkit"
)

// webhookTransport posts messages through Slack Incoming Webhooks.
//
// Response bodies are capped by go-http-kit itself at httpkit.MaxResponseBodySize
// (25MB, unconditional, not caller-configurable in v1.6.0) rather than the tighter
// 64KB this package enforced manually before adopting go-http-kit. A malicious or
// misbehaving webhook endpoint can't force unbounded memory growth, only up to that
// fixed ceiling; a real Slack incoming webhook only ever returns a few bytes.
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
	return webhookTransport{
		webhookURL:    strings.TrimSpace(cfg.WebhookURL),
		sourceLabel:   strings.TrimSpace(cfg.SourceLabel),
		httpKitClient: httpkit.New(requestTimeout, httpkit.WithNoRetry()),
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

// PreviewMessage builds the webhook payload without sending it.
func (w *webhookTransport) PreviewMessage(msg Message) (Message, error) {
	if strings.TrimSpace(msg.Text) == "" {
		return Message{}, fmt.Errorf("slack: text is required")
	}
	msg.Blocks = appendRawSourceLabelBlock(msg.Blocks, msg.Text, w.sourceLabel)
	return msg, nil
}

// PostMessage posts a message to Slack.
func (w *webhookTransport) PostMessage(ctx context.Context, msg Message) (*PostMessageResponse, error) {
	if w.webhookURL == "" {
		return nil, fmt.Errorf("slack: webhook URL is required")
	}
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

func appendRawSourceLabelBlock(blocks []map[string]any, text, sourceLabel string) []map[string]any {
	sourceLabel = strings.TrimSpace(sourceLabel)
	if sourceLabel == "" {
		return blocks
	}

	if len(blocks) == 0 && strings.TrimSpace(text) != "" {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": text,
			},
		})
	}

	return append(blocks, map[string]any{
		"type": "context",
		"elements": []map[string]any{
			{
				"type": "mrkdwn",
				"text": sourceLabel,
			},
		},
	})
}
