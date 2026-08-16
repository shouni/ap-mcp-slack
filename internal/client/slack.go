// Package client provides outbound service clients.
package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"
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
// accepts have been collected, Slack has no more pages, maxListPages requests have
// been made, or Slack rate-limits the walk. It returns the collected items, the cursor
// to resume from (empty once the listing is exhausted), and — only when the walk was
// cut off by rate limiting — how long Slack asked to wait before resuming. A nil keep
// accepts everything. It backs every paginated listing here (channels, joined
// channels, users), which differ only in the API they call and how they filter.
//
// Every item a page yields is kept, even when that pushes the total past limit.
// Slack's cursor resumes *after* the whole page no matter how many of the page's items
// the caller kept, so discarding the overshoot would lose those items permanently:
// the next call with the returned cursor would skip straight past them. requestLimit
// is narrowed to the outstanding shortfall on each page to keep the overshoot small,
// but Slack's pagination guide allows a page to exceed what was asked for.
//
// A rate limit partway through the walk is not a failure of the listing: the pages
// already fetched are complete, and cursor still points at the page Slack refused, so
// returning them with that cursor strands nothing — the caller resumes exactly where
// the walk was cut off, after retryAfter. Failing instead would discard fetched pages
// only to have the retry re-spend the very budget that was just exhausted. A rate
// limit on the *first* fetch has nothing to hand back, so it stays an error (whose
// message carries Slack's retry-after).
func collectPages[T any](ctx context.Context, apiMethod string, limit, pageSize int, cursor string, fetch pageFetcher[T], keep func(T) bool) (items []T, nextCursor string, retryAfter time.Duration, err error) {
	items = make([]T, 0, min(limit, pageSize))
	seenCursors := map[string]struct{}{}
	fetched := false

	for range maxListPages {
		if len(items) >= limit {
			break
		}

		page, pageCursor, err := fetch(ctx, cursor, min(pageSize, limit-len(items)))
		if err != nil {
			if wait, ok := rateLimitRetryAfter(err); ok && fetched {
				return items, cursor, wait, nil
			}
			return nil, "", 0, fmt.Errorf("slack: %s failed: %w", apiMethod, err)
		}
		fetched = true

		for _, item := range page {
			if keep == nil || keep(item) {
				items = append(items, item)
			}
		}

		pageCursor = strings.TrimSpace(pageCursor)
		if pageCursor == "" {
			return items, "", 0, nil
		}
		if _, ok := seenCursors[pageCursor]; ok {
			return nil, "", 0, fmt.Errorf("slack: %s returned duplicate cursor %q", apiMethod, pageCursor)
		}
		seenCursors[pageCursor] = struct{}{}
		cursor = pageCursor
	}

	return items, cursor, 0, nil
}

// rateLimitRetryAfter reports whether err is (or wraps) Slack's HTTP 429 response,
// and if so how long Slack asked to wait before the next request.
func rateLimitRetryAfter(err error) (time.Duration, bool) {
	rateLimited, ok := errors.AsType[*slackapi.RateLimitedError](err)
	if !ok {
		return 0, false
	}
	return rateLimited.RetryAfter, true
}

// retryAfterSeconds converts Slack's retry-after duration into the whole seconds
// reported on listing responses, rounding up so a caller that waits exactly this long
// is never early. Zero stays zero: it means the listing was not rate-limited at all.
func retryAfterSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int((d + time.Second - 1) / time.Second)
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
