package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

// WebAPIMessage is the message input sent to Slack chat.postMessage.
type WebAPIMessage struct {
	ChannelID   string           `json:"channel,omitempty"`
	Text        string           `json:"text,omitempty"`
	Blocks      []map[string]any `json:"blocks,omitempty"`
	Attachments []map[string]any `json:"attachments,omitempty"`
	ThreadTS    string           `json:"thread_ts,omitempty"`
	IconEmoji   string           `json:"icon_emoji,omitempty"`
	UnfurlLinks *bool            `json:"unfurl_links,omitempty"`
	UnfurlMedia *bool            `json:"unfurl_media,omitempty"`
}

// PostWebAPIMessageResponse contains the relevant chat.postMessage response fields.
type PostWebAPIMessageResponse struct {
	OK        bool   `json:"ok"`
	ChannelID string `json:"channel,omitempty"`
	TS        string `json:"ts,omitempty"`
}

// PreviewWebAPIMessage builds the chat.postMessage payload without sending it.
func (w *webAPITransport) PreviewWebAPIMessage(msg WebAPIMessage) (WebAPIMessage, error) {
	if strings.TrimSpace(msg.Text) == "" {
		return WebAPIMessage{}, fmt.Errorf("slack: text is required")
	}
	msg.ChannelID = w.channelIDOrDefault(msg.ChannelID)
	if msg.ChannelID == "" {
		return WebAPIMessage{}, fmt.Errorf("slack: channel_id is required")
	}
	msg.Blocks = appendRawSourceLabelBlock(msg.Blocks, msg.Text, w.sourceLabel)
	if _, err := convertBlocks(msg.Blocks); err != nil {
		return WebAPIMessage{}, err
	}
	if _, err := convertAttachments(msg.Attachments); err != nil {
		return WebAPIMessage{}, err
	}
	return msg, nil
}

// DeleteWebAPIMessageResponse contains the relevant chat.delete response fields.
type DeleteWebAPIMessageResponse struct {
	OK        bool   `json:"ok"`
	ChannelID string `json:"channel,omitempty"`
	TS        string `json:"ts,omitempty"`
}

// PostWebAPIMessage posts a message with Slack Web API chat.postMessage.
func (w *webAPITransport) PostWebAPIMessage(ctx context.Context, msg WebAPIMessage) (*PostWebAPIMessageResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(msg.Text) == "" {
		return nil, fmt.Errorf("slack: text is required")
	}
	msg.ChannelID = w.channelIDOrDefault(msg.ChannelID)
	if msg.ChannelID == "" {
		return nil, fmt.Errorf("slack: channel_id is required")
	}

	options, err := buildPostMessageOptions(msg, w.sourceLabel)
	if err != nil {
		return nil, err
	}
	channelID, ts, err := w.slackAPIClient.PostMessageContext(ctx, msg.ChannelID, options...)
	if err != nil {
		return nil, fmt.Errorf("slack: chat.postMessage failed: %w", err)
	}
	return &PostWebAPIMessageResponse{
		OK:        true,
		ChannelID: channelID,
		TS:        ts,
	}, nil
}

// UpdateWebAPIMessage is the message content sent to Slack Web API chat.update.
type UpdateWebAPIMessage struct {
	ChannelID   string           `json:"channel,omitempty"`
	TS          string           `json:"ts"`
	Text        string           `json:"text,omitempty"`
	Blocks      []map[string]any `json:"blocks,omitempty"`
	Attachments []map[string]any `json:"attachments,omitempty"`
}

// UpdateWebAPIMessageResponse contains the relevant chat.update response fields.
type UpdateWebAPIMessageResponse struct {
	OK        bool   `json:"ok"`
	ChannelID string `json:"channel,omitempty"`
	TS        string `json:"ts,omitempty"`
	Text      string `json:"text,omitempty"`
}

// UpdateWebAPIMessage replaces a message's content with Slack Web API chat.update.
// Only the original poster (the same bot, for a bot token, or the same user, for a
// user token) can update a message; Slack rejects the request otherwise. As with
// PostWebAPIMessage, blocks/attachments fully replace the previous content rather
// than merging with it.
func (w *webAPITransport) UpdateWebAPIMessage(ctx context.Context, msg UpdateWebAPIMessage) (*UpdateWebAPIMessageResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	msg.ChannelID = w.channelIDOrDefault(msg.ChannelID)
	if msg.ChannelID == "" {
		return nil, fmt.Errorf("slack: channel_id is required")
	}
	ts := strings.TrimSpace(msg.TS)
	if ts == "" {
		return nil, fmt.Errorf("slack: ts is required")
	}
	if strings.TrimSpace(msg.Text) == "" && len(msg.Blocks) == 0 && len(msg.Attachments) == 0 {
		return nil, fmt.Errorf("slack: text, blocks, or attachments is required")
	}

	options, err := buildContentOptions(msg.Text, msg.Blocks, msg.Attachments, w.sourceLabel)
	if err != nil {
		return nil, err
	}

	channelID, respTS, text, err := w.slackAPIClient.UpdateMessageContext(ctx, msg.ChannelID, ts, options...)
	if err != nil {
		return nil, fmt.Errorf("slack: chat.update failed: %w", err)
	}
	return &UpdateWebAPIMessageResponse{
		OK:        true,
		ChannelID: channelID,
		TS:        respTS,
		Text:      text,
	}, nil
}

// DeleteWebAPIMessage deletes a message with Slack Web API chat.delete.
func (w *webAPITransport) DeleteWebAPIMessage(ctx context.Context, channelID string, ts string) (*DeleteWebAPIMessageResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	channelID = w.channelIDOrDefault(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("slack: channel_id is required")
	}
	if strings.TrimSpace(ts) == "" {
		return nil, fmt.Errorf("slack: ts is required")
	}

	respChannelID, respTS, err := w.slackAPIClient.DeleteMessageContext(ctx, channelID, strings.TrimSpace(ts))
	if err != nil {
		return nil, fmt.Errorf("slack: chat.delete failed: %w", err)
	}
	return &DeleteWebAPIMessageResponse{
		OK:        true,
		ChannelID: respChannelID,
		TS:        respTS,
	}, nil
}

func buildPostMessageOptions(msg WebAPIMessage, sourceLabel string) ([]slackapi.MsgOption, error) {
	options, err := buildContentOptions(msg.Text, msg.Blocks, msg.Attachments, sourceLabel)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(msg.ThreadTS) != "" {
		options = append(options, slackapi.MsgOptionTS(strings.TrimSpace(msg.ThreadTS)))
	}
	if strings.TrimSpace(msg.IconEmoji) != "" {
		options = append(options, slackapi.MsgOptionIconEmoji(strings.TrimSpace(msg.IconEmoji)))
	}
	if msg.UnfurlLinks != nil {
		if *msg.UnfurlLinks {
			options = append(options, slackapi.MsgOptionEnableLinkUnfurl())
		} else {
			options = append(options, slackapi.MsgOptionDisableLinkUnfurl())
		}
	}
	if msg.UnfurlMedia != nil && !*msg.UnfurlMedia {
		options = append(options, slackapi.MsgOptionDisableMediaUnfurl())
	}

	return options, nil
}

// buildContentOptions builds the text/blocks/attachments options shared by
// chat.postMessage and chat.update, appending the source-label footer block.
func buildContentOptions(text string, rawBlocks, rawAttachments []map[string]any, sourceLabel string) ([]slackapi.MsgOption, error) {
	options := []slackapi.MsgOption{slackapi.MsgOptionText(text, false)}

	blocks, err := convertBlocks(rawBlocks)
	if err != nil {
		return nil, err
	}
	blocks = appendSourceLabelBlock(blocks, text, sourceLabel)
	if len(blocks) > 0 {
		options = append(options, slackapi.MsgOptionBlocks(blocks...))
	}

	attachments, err := convertAttachments(rawAttachments)
	if err != nil {
		return nil, err
	}
	if len(attachments) > 0 {
		options = append(options, slackapi.MsgOptionAttachments(attachments...))
	}

	return options, nil
}

// appendSourceLabelBlock appends a context block naming the message's source (e.g.
// "ap-mcp-slack (MCP) 経由"). This exists because a user-token chat.postMessage call
// posts under the human user's own name and avatar with no "APP" badge, so without
// this footer there is no way to tell an MCP-originated post apart from one the user
// typed by hand. If the caller supplied no blocks, msg.Text is first turned into a
// section block so the visible body isn't replaced by just the footer: Slack renders
// blocks (when present) in place of text, using text only as the fallback/notification
// string.
func appendSourceLabelBlock(blocks []slackapi.Block, text, sourceLabel string) []slackapi.Block {
	sourceLabel = strings.TrimSpace(sourceLabel)
	if sourceLabel == "" {
		return blocks
	}

	if len(blocks) == 0 && strings.TrimSpace(text) != "" {
		blocks = append(blocks, slackapi.NewSectionBlock(
			slackapi.NewTextBlockObject(slackapi.MarkdownType, text, false, false), nil, nil,
		))
	}

	return append(blocks, slackapi.NewContextBlock("",
		slackapi.NewTextBlockObject(slackapi.MarkdownType, sourceLabel, false, false),
	))
}

func convertBlocks(rawBlocks []map[string]any) ([]slackapi.Block, error) {
	if len(rawBlocks) == 0 {
		return nil, nil
	}

	blocks := make([]slackapi.Block, 0, len(rawBlocks))
	for _, rawBlock := range rawBlocks {
		data, err := json.Marshal(rawBlock)
		if err != nil {
			return nil, fmt.Errorf("slack: failed to encode block: %w", err)
		}
		block, err := slackapi.BlockFromJSON(string(data))
		if err != nil {
			return nil, fmt.Errorf("slack: failed to decode block: %w", err)
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func convertAttachments(rawAttachments []map[string]any) ([]slackapi.Attachment, error) {
	if len(rawAttachments) == 0 {
		return nil, nil
	}

	data, err := json.Marshal(rawAttachments)
	if err != nil {
		return nil, fmt.Errorf("slack: failed to encode attachments: %w", err)
	}

	var attachments []slackapi.Attachment
	if err := json.Unmarshal(data, &attachments); err != nil {
		return nil, fmt.Errorf("slack: failed to decode attachments: %w", err)
	}
	return attachments, nil
}
