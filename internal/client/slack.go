// Package client provides outbound service clients.
package client

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const requestTimeout = 10 * time.Second

// SlackClient exposes Slack messaging and workspace lookups over two independent
// transports: a webhook transport (Incoming Webhook posting, webhook.go) and a
// token-authenticated Web API transport (message post/update/delete, channel and
// message reads, user lookups, auth.test — webapi*.go, users.go, auth.go). The two
// share no state, so they're kept as separate embedded types rather than one struct
// with fields that are only meaningful to one side or the other.
type SlackClient struct {
	webhookTransport
	webAPITransport
}

// SlackClientConfig configures SlackClient.
type SlackClientConfig struct {
	WebhookURL       string
	Token            string
	DefaultChannelID string
	APIBaseURL       string
	// SourceLabel, if set, is appended as a Block Kit context footer on every posted
	// message so MCP-originated posts stay distinguishable from messages typed by
	// hand.
	SourceLabel string
}

// NewSlackClientWithConfig creates a SlackClient with explicit configuration.
func NewSlackClientWithConfig(cfg SlackClientConfig) *SlackClient {
	return &SlackClient{
		webhookTransport: newWebhookTransport(cfg),
		webAPITransport:  newWebAPITransport(cfg),
	}
}

// normalizeListLimit validates and applies defaults to a limit option shared by the
// paginated list operations (channels, joined channels, users).
func normalizeListLimit(limit, defaultLimit, maxLimit int) (int, error) {
	if limit == 0 {
		return defaultLimit, nil
	}
	if limit < 0 {
		return 0, fmt.Errorf("slack: limit must be greater than 0")
	}
	if limit > maxLimit {
		return 0, fmt.Errorf("slack: limit must be %d or less", maxLimit)
	}
	return limit, nil
}

// pageFetcher fetches one page of already-summarized items at cursor, requesting at
// most requestLimit of them, and returns the page along with Slack's cursor for the
// next one ("" once the listing is exhausted).
type pageFetcher[T any] func(ctx context.Context, cursor string, requestLimit int) (items []T, nextCursor string, err error)

// maxListPages bounds how many Slack requests one paginated listing may make. A filter
// that matches little (a rare users.list query, say) reaches limit slowly or never, and
// without a bound a single tool call would page through an entire workspace: the
// per-request timeout does nothing about request *count*. Hitting the bound is not an
// error — the resume cursor is returned as usual, so a caller who genuinely wants to go
// further simply calls again with it.
const maxListPages = 20

// collectPages pages through fetch, starting at cursor, until limit items that keep
// accepts have been collected, Slack has no more pages, or maxListPages requests have
// been made. It returns the collected items and the cursor to resume from (empty once
// the listing is exhausted). A nil keep accepts everything. It backs every paginated
// listing here (channels, joined channels, users), which differ only in the API they
// call and how they filter.
//
// Every item a page yields is kept, even when that pushes the total past limit.
// Slack's cursor resumes *after* the whole page no matter how many of the page's items
// the caller kept, so discarding the overshoot would lose those items permanently:
// the next call with the returned cursor would skip straight past them. requestLimit
// is narrowed to the outstanding shortfall on each page to keep the overshoot small,
// but Slack's pagination guide allows a page to exceed what was asked for.
func collectPages[T any](ctx context.Context, apiMethod string, limit, pageSize int, cursor string, fetch pageFetcher[T], keep func(T) bool) ([]T, string, error) {
	items := make([]T, 0, min(limit, pageSize))
	seenCursors := map[string]struct{}{}

	for range maxListPages {
		if len(items) >= limit {
			break
		}

		page, nextCursor, err := fetch(ctx, cursor, min(pageSize, limit-len(items)))
		if err != nil {
			return nil, "", fmt.Errorf("slack: %s failed: %w", apiMethod, err)
		}

		for _, item := range page {
			if keep == nil || keep(item) {
				items = append(items, item)
			}
		}

		nextCursor = strings.TrimSpace(nextCursor)
		if nextCursor == "" {
			return items, "", nil
		}
		if _, ok := seenCursors[nextCursor]; ok {
			return nil, "", fmt.Errorf("slack: %s returned duplicate cursor %q", apiMethod, nextCursor)
		}
		seenCursors[nextCursor] = struct{}{}
		cursor = nextCursor
	}

	return items, cursor, nil
}

// appendSourceLabelBlock appends a Block Kit context footer naming the message's
// source (e.g. "ap-mcp-slack (MCP) 経由"). This exists because a user-token
// chat.postMessage call posts under the human user's own name and avatar with no "APP"
// badge, so without this footer there is no way to tell an MCP-originated post apart
// from one the user typed by hand.
//
// If the caller supplied no blocks, text is first turned into a section block so the
// visible body isn't replaced by just the footer: Slack renders blocks (when present)
// in place of text, using text only as the fallback/notification string.
//
// This is the single place the footer is applied, for both transports. The payload a
// preview shows a human is the payload that gets sent, so a second implementation on
// the send path could only ever drift away from the one the human approved.
func appendSourceLabelBlock(blocks []map[string]any, text, sourceLabel string) []map[string]any {
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
