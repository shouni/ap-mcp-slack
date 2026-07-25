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

// PreviewWebAPIMessage resolves and validates the chat.postMessage payload without
// sending it: it fills in the default channel, appends the source-label footer, and
// rejects blocks/attachments Slack could not parse.
//
// PostWebAPIMessage sends exactly what this returns, so the payload a human approves
// in a preview is the payload that reaches Slack.
func (w *webAPITransport) PreviewWebAPIMessage(msg WebAPIMessage) (WebAPIMessage, error) {
	if strings.TrimSpace(msg.Text) == "" {
		return WebAPIMessage{}, fmt.Errorf("slack: text is required")
	}
	resolvedChannelID, err := w.ResolveChannelID(msg.ChannelID)
	if err != nil {
		return WebAPIMessage{}, err
	}
	msg.ChannelID = resolvedChannelID
	msg.Blocks = appendSourceLabelBlock(msg.Blocks, msg.Text, w.sourceLabel)
	if _, err := buildContentOptions(msg.Text, msg.Blocks, msg.Attachments); err != nil {
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

// PostWebAPIMessage posts a message with Slack Web API chat.postMessage. It resolves
// the payload through PreviewWebAPIMessage so the message sent is byte-for-byte the one
// a preview of the same input would have shown.
func (w *webAPITransport) PostWebAPIMessage(ctx context.Context, msg WebAPIMessage) (*PostWebAPIMessageResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	msg, err := w.PreviewWebAPIMessage(msg)
	if err != nil {
		return nil, err
	}

	options, err := buildPostMessageOptions(msg)
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

// PreviewUpdateWebAPIMessage resolves and validates the chat.update payload without
// sending it, the same way PreviewWebAPIMessage does for chat.postMessage: it fills in
// the default channel, appends the source-label footer, and rejects unparseable
// blocks/attachments.
//
// An update overwrites content irreversibly, so the caller is shown the resolved
// replacement — footer and all — rather than just the raw text it passed in.
func (w *webAPITransport) PreviewUpdateWebAPIMessage(msg UpdateWebAPIMessage) (UpdateWebAPIMessage, error) {
	resolvedChannelID, err := w.ResolveChannelID(msg.ChannelID)
	if err != nil {
		return UpdateWebAPIMessage{}, err
	}
	msg.ChannelID = resolvedChannelID
	msg.TS = strings.TrimSpace(msg.TS)
	if msg.TS == "" {
		return UpdateWebAPIMessage{}, fmt.Errorf("slack: ts is required")
	}
	if strings.TrimSpace(msg.Text) == "" && len(msg.Blocks) == 0 && len(msg.Attachments) == 0 {
		return UpdateWebAPIMessage{}, fmt.Errorf("slack: text, blocks, or attachments is required")
	}
	msg.Blocks = appendSourceLabelBlock(msg.Blocks, msg.Text, w.sourceLabel)
	if _, err := buildContentOptions(msg.Text, msg.Blocks, msg.Attachments); err != nil {
		return UpdateWebAPIMessage{}, err
	}
	return msg, nil
}

// UpdateWebAPIMessage replaces a message's content with Slack Web API chat.update.
// Only the original poster (the same bot, for a bot token, or the same user, for a
// user token) can update a message; Slack rejects the request otherwise. As with
// PostWebAPIMessage, blocks/attachments fully replace the previous content rather
// than merging with it, and the payload is resolved through
// PreviewUpdateWebAPIMessage so it matches what a preview would have shown.
func (w *webAPITransport) UpdateWebAPIMessage(ctx context.Context, msg UpdateWebAPIMessage) (*UpdateWebAPIMessageResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	msg, err := w.PreviewUpdateWebAPIMessage(msg)
	if err != nil {
		return nil, err
	}

	options, err := buildContentOptions(msg.Text, msg.Blocks, msg.Attachments)
	if err != nil {
		return nil, err
	}

	channelID, respTS, text, err := w.slackAPIClient.UpdateMessageContext(ctx, msg.ChannelID, msg.TS, options...)
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
	channelID, err := w.ResolveChannelID(channelID)
	if err != nil {
		return nil, err
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

func buildPostMessageOptions(msg WebAPIMessage) ([]slackapi.MsgOption, error) {
	options, err := buildContentOptions(msg.Text, msg.Blocks, msg.Attachments)
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
// chat.postMessage and chat.update. The source-label footer is expected to be already
// present in rawBlocks: the Preview* methods apply it, and both send paths route
// through them, so it is applied in exactly one place.
//
// Doubling as the payload validator is deliberate — the Preview* methods call this and
// discard the options, which guarantees a preview rejects precisely the payloads the
// corresponding send would have rejected.
func buildContentOptions(text string, rawBlocks, rawAttachments []map[string]any) ([]slackapi.MsgOption, error) {
	options := []slackapi.MsgOption{slackapi.MsgOptionText(text, false)}

	blocks, err := convertBlocks(rawBlocks)
	if err != nil {
		return nil, err
	}
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
