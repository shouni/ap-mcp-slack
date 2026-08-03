package client

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

const (
	defaultMessageListLimit = 100
	maxMessageListLimit     = 1000

	// messageSearchPageSize and maxMessageSearchPages bound the thread walk GetMessage
	// performs to locate one ts, so resolving an update/delete target cannot page
	// through an arbitrarily long thread. Together they cover the first 1000 messages
	// of a thread; beyond that GetMessage reports the search as truncated instead of
	// claiming the message does not exist.
	messageSearchPageSize = 100
	maxMessageSearchPages = 10
)

// ConversationHistoryOptions configures Slack conversations.history requests.
type ConversationHistoryOptions struct {
	ChannelID          string `json:"channel_id,omitempty"`
	Limit              int    `json:"limit,omitempty"`
	Cursor             string `json:"cursor,omitempty"`
	Oldest             string `json:"oldest,omitempty"`
	Latest             string `json:"latest,omitempty"`
	Inclusive          bool   `json:"inclusive,omitempty"`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty"`
	IncludeRawBlocks   bool   `json:"include_raw_blocks,omitempty"`
}

// ConversationRepliesOptions configures Slack conversations.replies requests.
type ConversationRepliesOptions struct {
	ChannelID          string `json:"channel_id,omitempty"`
	TS                 string `json:"ts,omitempty"`
	Limit              int    `json:"limit,omitempty"`
	Cursor             string `json:"cursor,omitempty"`
	Oldest             string `json:"oldest,omitempty"`
	Latest             string `json:"latest,omitempty"`
	Inclusive          bool   `json:"inclusive,omitempty"`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty"`
	IncludeRawBlocks   bool   `json:"include_raw_blocks,omitempty"`
}

// SlackMessageSummary contains the message fields returned by history/replies tools.
type SlackMessageSummary struct {
	Type     string `json:"type,omitempty"`
	SubType  string `json:"subtype,omitempty"`
	User     string `json:"user,omitempty"`
	BotID    string `json:"bot_id,omitempty"`
	Username string `json:"username,omitempty"`
	Text     string `json:"text,omitempty"`
	// Blocks and Attachments carry the raw Block Kit / attachment payload and are
	// only populated when the caller sets IncludeRawBlocks; otherwise their
	// content is folded into Text via a best-effort plain-text extraction to
	// avoid returning large, mostly-boilerplate JSON to callers that just want
	// the message content.
	Blocks      any `json:"blocks,omitempty"`
	Attachments any `json:"attachments,omitempty"`
	// Metadata carries Slack's message metadata when the message has any. In practice
	// that means the caller set IncludeAllMetadata, since Slack only returns metadata
	// when asked — but this is populated from what arrived rather than from the request
	// flag, so it stays correct if that ever stops holding.
	Metadata   any      `json:"metadata,omitempty"`
	TS         string   `json:"ts,omitempty"`
	ThreadTS   string   `json:"thread_ts,omitempty"`
	ParentUser string   `json:"parent_user_id,omitempty"`
	ReplyCount int      `json:"reply_count,omitempty"`
	ReplyUsers []string `json:"reply_users,omitempty"`
}

// ConversationMessagesResponse contains the relevant conversations.history/replies response fields.
type ConversationMessagesResponse struct {
	OK         bool                  `json:"ok"`
	Messages   []SlackMessageSummary `json:"messages"`
	Count      int                   `json:"count"`
	HasMore    bool                  `json:"has_more"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

// GetConversationHistory fetches messages from a Slack conversation with conversations.history.
func (w *webAPITransport) GetConversationHistory(ctx context.Context, opts ConversationHistoryOptions) (*ConversationMessagesResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	channelID, err := w.ResolveChannelID(opts.ChannelID)
	if err != nil {
		return nil, err
	}
	limit, err := normalizeListLimit(opts.Limit, defaultMessageListLimit, maxMessageListLimit)
	if err != nil {
		return nil, err
	}

	resp, err := w.slackAPIClient.GetConversationHistoryContext(ctx, &slackapi.GetConversationHistoryParameters{
		ChannelID:          channelID,
		Cursor:             strings.TrimSpace(opts.Cursor),
		Inclusive:          opts.Inclusive,
		Latest:             strings.TrimSpace(opts.Latest),
		Limit:              limit,
		Oldest:             strings.TrimSpace(opts.Oldest),
		IncludeAllMetadata: opts.IncludeAllMetadata,
	})
	if err != nil {
		return nil, fmt.Errorf("slack: conversations.history failed: %w", err)
	}
	messages := summarizeMessages(resp.Messages, opts.IncludeRawBlocks)
	return &ConversationMessagesResponse{
		OK:         true,
		Messages:   messages,
		Count:      len(messages),
		HasMore:    resp.HasMore,
		NextCursor: strings.TrimSpace(resp.ResponseMetaData.NextCursor),
	}, nil
}

// GetConversationReplies fetches the thread rooted at TS with conversations.replies.
func (w *webAPITransport) GetConversationReplies(ctx context.Context, opts ConversationRepliesOptions) (*ConversationMessagesResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	channelID, err := w.ResolveChannelID(opts.ChannelID)
	if err != nil {
		return nil, err
	}
	ts := strings.TrimSpace(opts.TS)
	if ts == "" {
		return nil, fmt.Errorf("slack: ts is required")
	}
	limit, err := normalizeListLimit(opts.Limit, defaultMessageListLimit, maxMessageListLimit)
	if err != nil {
		return nil, err
	}

	apiMessages, hasMore, nextCursor, err := w.slackAPIClient.GetConversationRepliesContext(ctx, &slackapi.GetConversationRepliesParameters{
		ChannelID:          channelID,
		Timestamp:          ts,
		Cursor:             strings.TrimSpace(opts.Cursor),
		Inclusive:          opts.Inclusive,
		Latest:             strings.TrimSpace(opts.Latest),
		Limit:              limit,
		Oldest:             strings.TrimSpace(opts.Oldest),
		IncludeAllMetadata: opts.IncludeAllMetadata,
	})
	if err != nil {
		return nil, fmt.Errorf("slack: conversations.replies failed: %w", err)
	}
	messages := summarizeMessages(apiMessages, opts.IncludeRawBlocks)
	return &ConversationMessagesResponse{
		OK:         true,
		Messages:   messages,
		Count:      len(messages),
		HasMore:    hasMore,
		NextCursor: strings.TrimSpace(nextCursor),
	}, nil
}

// GetMessage fetches the single message at ts in channelID, so the update/delete
// tools can show a caller which message they are about to rewrite or destroy.
//
// conversations.history on its own is not enough. It only returns top-level channel
// messages, so for a thread reply Slack answers with the nearest *older* top-level
// message instead — precisely the wrong message to display above a delete
// confirmation. Every candidate's ts is therefore compared against the requested one,
// and only on a mismatch (or an empty page) does this fall back to
// conversations.replies.
//
// The replies fallback follows Slack's cursor rather than reading one page.
// conversations.replies returns a thread from its beginning no matter which member's ts
// was asked about, so a reply past the first page is unreachable without paging — and
// reporting it as absent would tell a caller their ts is wrong moments before a delete
// that will in fact succeed. The walk is bounded by maxMessageSearchPages; hitting that
// bound is reported through the second result rather than as "not found", so the caller
// can say "gave up looking" instead of "no such message".
//
// A message that simply isn't there is reported as (nil, false, nil) rather than an
// error, since "no such message" is a legitimate answer the caller should see rather
// than a failure. API errors are returned as-is and left for the caller to downgrade:
// chat.update and chat.delete need no history scope, so a token that can delete but
// not read must still be able to delete.
func (w *webAPITransport) GetMessage(ctx context.Context, channelID, ts string) (message *SlackMessageSummary, searchTruncated bool, err error) {
	if err := w.requireToken(); err != nil {
		return nil, false, err
	}
	channelID, err = w.ResolveChannelID(channelID)
	if err != nil {
		return nil, false, err
	}
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return nil, false, fmt.Errorf("slack: ts is required")
	}

	history, historyErr := w.GetConversationHistory(ctx, ConversationHistoryOptions{
		ChannelID: channelID,
		Latest:    ts,
		Inclusive: true,
		Limit:     1,
	})
	if historyErr == nil {
		if found := findMessageByTS(history.Messages, ts); found != nil {
			return found, false, nil
		}
	}

	cursor := ""
	for range maxMessageSearchPages {
		replies, err := w.GetConversationReplies(ctx, ConversationRepliesOptions{
			ChannelID: channelID,
			TS:        ts,
			Cursor:    cursor,
			Inclusive: true,
			Limit:     messageSearchPageSize,
		})
		if err != nil {
			// Prefer the history error when both calls failed: history is the call that
			// covers ordinary channel messages, so its failure is the more informative one.
			if historyErr != nil {
				return nil, false, historyErr
			}
			return nil, false, err
		}
		if found := findMessageByTS(replies.Messages, ts); found != nil {
			return found, false, nil
		}
		cursor = replies.NextCursor
		if cursor == "" {
			return nil, false, nil
		}
	}
	return nil, true, nil
}

// findMessageByTS returns the message whose ts is exactly ts, or nil. It returns a
// pointer into messages, so callers must not retain it past the slice's lifetime.
func findMessageByTS(messages []SlackMessageSummary, ts string) *SlackMessageSummary {
	for i := range messages {
		if messages[i].TS == ts {
			return &messages[i]
		}
	}
	return nil
}

func summarizeMessages(messages []slackapi.Message, includeRawBlocks bool) []SlackMessageSummary {
	out := make([]SlackMessageSummary, 0, len(messages))
	for _, message := range messages {
		summary := SlackMessageSummary{
			Type:       message.Type,
			SubType:    message.SubType,
			User:       message.User,
			BotID:      message.BotID,
			Username:   message.Username,
			Text:       message.Text,
			TS:         message.Timestamp,
			ThreadTS:   message.ThreadTimestamp,
			ParentUser: message.ParentUserId,
			ReplyCount: message.ReplyCount,
			ReplyUsers: message.ReplyUsers,
		}
		if strings.TrimSpace(summary.Text) == "" {
			summary.Text = fallbackText(message.Blocks, message.Attachments)
		}
		if includeRawBlocks {
			if len(message.Blocks.BlockSet) > 0 {
				summary.Blocks = message.Blocks.BlockSet
			}
			if len(message.Attachments) > 0 {
				summary.Attachments = message.Attachments
			}
		}
		if message.Metadata.EventType != "" {
			summary.Metadata = message.Metadata
		}
		out = append(out, summary)
	}
	return out
}

// fallbackText produces a best-effort plain-text rendering of a message's
// blocks/attachments, for the (mostly bot/app) messages that leave the
// top-level Text field empty and put their content in Block Kit or
// attachments instead. This lets summarizeMessages omit the much larger raw
// blocks/attachments payload by default without losing the message content.
//
// It takes the blocks and attachments rather than a slackapi.Message because
// search.messages returns its matches as a different type carrying the same two
// fields, and a block-only message must not read as empty there either.
func fallbackText(blocks slackapi.Blocks, attachments []slackapi.Attachment) string {
	var parts []string
	if text := blocksPlainText(blocks); text != "" {
		parts = append(parts, text)
	}
	if text := attachmentsPlainText(attachments); text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

func blocksPlainText(blocks slackapi.Blocks) string {
	var lines []string
	for _, block := range blocks.BlockSet {
		switch b := block.(type) {
		case *slackapi.SectionBlock:
			if b.Text != nil && b.Text.Text != "" {
				lines = append(lines, b.Text.Text)
			}
			for _, field := range b.Fields {
				if field != nil && field.Text != "" {
					lines = append(lines, field.Text)
				}
			}
		case *slackapi.HeaderBlock:
			if b.Text != nil && b.Text.Text != "" {
				lines = append(lines, b.Text.Text)
			}
		case *slackapi.ContextBlock:
			for _, element := range b.ContextElements.Elements {
				if text, ok := element.(*slackapi.TextBlockObject); ok && text.Text != "" {
					lines = append(lines, text.Text)
				}
			}
		case *slackapi.ImageBlock:
			if b.Title != nil && b.Title.Text != "" {
				lines = append(lines, b.Title.Text)
			} else if b.AltText != "" {
				lines = append(lines, b.AltText)
			}
		case *slackapi.RichTextBlock:
			if text := richTextElementsPlainText(b.Elements); text != "" {
				lines = append(lines, text)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// richTextElementsPlainText walks the rich_text block element tree (sections,
// lists, quotes, preformatted code) and concatenates their text content.
// Non-text elements (users, channels, emoji, links without display text) are
// skipped rather than rendered, since this is a best-effort fallback, not a
// full rich_text renderer.
func richTextElementsPlainText(elements []slackapi.RichTextElement) string {
	var lines []string
	for _, element := range elements {
		switch e := element.(type) {
		case *slackapi.RichTextSection:
			if text := richTextSectionElementsPlainText(e.Elements); text != "" {
				lines = append(lines, text)
			}
		case *slackapi.RichTextQuote:
			if text := richTextSectionElementsPlainText(e.Elements); text != "" {
				lines = append(lines, text)
			}
		case *slackapi.RichTextPreformatted:
			if text := richTextSectionElementsPlainText(e.Elements); text != "" {
				lines = append(lines, text)
			}
		case *slackapi.RichTextList:
			if text := richTextElementsPlainText(e.Elements); text != "" {
				lines = append(lines, text)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func richTextSectionElementsPlainText(elements []slackapi.RichTextSectionElement) string {
	var b strings.Builder
	for _, element := range elements {
		switch e := element.(type) {
		case *slackapi.RichTextSectionTextElement:
			b.WriteString(e.Text)
		case *slackapi.RichTextSectionLinkElement:
			if e.Text != "" {
				b.WriteString(e.Text)
			} else {
				b.WriteString(e.URL)
			}
		}
	}
	return b.String()
}

// attachmentsPlainText renders the human-readable parts of legacy message
// attachments (pretext/text/fields/footer), skipping layout-only fields like
// color, image URLs, and action definitions.
func attachmentsPlainText(attachments []slackapi.Attachment) string {
	var lines []string
	for _, attachment := range attachments {
		if attachment.Pretext != "" {
			lines = append(lines, attachment.Pretext)
		}
		if attachment.Title != "" {
			lines = append(lines, attachment.Title)
		}
		if attachment.Text != "" {
			lines = append(lines, attachment.Text)
		} else if attachment.Fallback != "" {
			lines = append(lines, attachment.Fallback)
		}
		for _, field := range attachment.Fields {
			if field.Title != "" || field.Value != "" {
				lines = append(lines, strings.TrimSpace(field.Title+": "+field.Value))
			}
		}
		if attachment.Footer != "" {
			lines = append(lines, attachment.Footer)
		}
	}
	return strings.Join(lines, "\n")
}
