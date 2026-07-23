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
	// Metadata is only populated when the caller sets IncludeAllMetadata.
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
	channelID := w.channelIDOrDefault(opts.ChannelID)
	if channelID == "" {
		return nil, fmt.Errorf("slack: channel_id is required")
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
	channelID := w.channelIDOrDefault(opts.ChannelID)
	if channelID == "" {
		return nil, fmt.Errorf("slack: channel_id is required")
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
			summary.Text = fallbackMessageText(message)
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

// fallbackMessageText produces a best-effort plain-text rendering of a
// message's blocks/attachments, for the (mostly bot/app) messages that leave
// the top-level Text field empty and put their content in Block Kit or
// attachments instead. This lets summarizeMessages omit the much larger raw
// blocks/attachments payload by default without losing the message content.
func fallbackMessageText(message slackapi.Message) string {
	var parts []string
	if text := blocksPlainText(message.Blocks); text != "" {
		parts = append(parts, text)
	}
	if text := attachmentsPlainText(message.Attachments); text != "" {
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
