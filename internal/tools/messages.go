package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

// MessageContent holds the fields shared by post_slack_message and
// post_slack_message_as_user (everything except thread_ts, whose semantics differ
// slightly between the webhook and Web API paths, and channel_id, which only the
// Web API path accepts).
type MessageContent struct {
	Text        string           `json:"text" jsonschema:"Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。"`
	Blocks      []map[string]any `json:"blocks,omitempty" jsonschema:"任意のSlack Block Kit blocks配列。指定する場合もアクセシビリティ用にtextを含めてください。"`
	Attachments []map[string]any `json:"attachments,omitempty" jsonschema:"任意のSlack attachments配列。"`
	IconEmoji   string           `json:"icon_emoji,omitempty" jsonschema:"投稿者アイコンとして使うSlack絵文字名。例: :robot_face:"`
	UnfurlLinks *bool            `json:"unfurl_links,omitempty" jsonschema:"リンク展開を制御します。"`
	UnfurlMedia *bool            `json:"unfurl_media,omitempty" jsonschema:"メディア展開を制御します。"`
}

// toMessage builds the Incoming Webhook payload, which carries threadTS as a
// separate top-level field.
func (m MessageContent) toMessage(threadTS string) client.Message {
	return client.Message{
		Text:        m.Text,
		Blocks:      m.Blocks,
		Attachments: m.Attachments,
		ThreadTS:    threadTS,
		IconEmoji:   m.IconEmoji,
		UnfurlLinks: m.UnfurlLinks,
		UnfurlMedia: m.UnfurlMedia,
	}
}

// toWebAPIMessage builds the chat.postMessage payload, which additionally carries
// the destination channelID.
func (m MessageContent) toWebAPIMessage(channelID, threadTS string) client.WebAPIMessage {
	return client.WebAPIMessage{
		ChannelID:   channelID,
		Text:        m.Text,
		Blocks:      m.Blocks,
		Attachments: m.Attachments,
		ThreadTS:    threadTS,
		IconEmoji:   m.IconEmoji,
		UnfurlLinks: m.UnfurlLinks,
		UnfurlMedia: m.UnfurlMedia,
	}
}

// PostSlackMessageInput is the input for post_slack_message.
type PostSlackMessageInput struct {
	MessageContent
	ThreadTS string `json:"thread_ts,omitempty" jsonschema:"スレッド返信にする場合の親メッセージts。Webhook側で利用可能な場合のみ有効です。"`
}

// PostSlackMessageOutput is the structured output for post_slack_message.
type PostSlackMessageOutput struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body,omitempty"`
}

// PreviewSlackMessageOutput is the structured output for preview_slack_message.
type PreviewSlackMessageOutput struct {
	OK        bool           `json:"ok"`
	Transport string         `json:"transport"`
	Payload   client.Message `json:"payload"`
}

// PostSlackMessageAsUserInput is the input for post_slack_message_as_user.
type PostSlackMessageAsUserInput struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"投稿先チャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	MessageContent
	ThreadTS string `json:"thread_ts,omitempty" jsonschema:"スレッド返信にする場合の親メッセージts。"`
}

// PostSlackMessageAsUserOutput is the structured output for post_slack_message_as_user.
type PostSlackMessageAsUserOutput = client.PostWebAPIMessageResponse

// PreviewSlackMessageAsUserOutput is the structured output for preview_slack_message_as_user.
type PreviewSlackMessageAsUserOutput struct {
	OK        bool                 `json:"ok"`
	Transport string               `json:"transport"`
	ChannelID string               `json:"channel_id"`
	Payload   client.WebAPIMessage `json:"payload"`
}

// UpdateSlackMessageInput is the input for update_slack_message.
type UpdateSlackMessageInput struct {
	ChannelID   string           `json:"channel_id,omitempty" jsonschema:"更新対象のチャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	TS          string           `json:"ts" jsonschema:"更新対象メッセージのts。post_slack_message_as_user の戻り値を利用できます。"`
	Text        string           `json:"text,omitempty" jsonschema:"更新後の本文。blocksを指定しない場合は必須です。"`
	Blocks      []map[string]any `json:"blocks,omitempty" jsonschema:"更新後のSlack Block Kit blocks配列。指定すると既存のblocksを置き換えます。"`
	Attachments []map[string]any `json:"attachments,omitempty" jsonschema:"更新後のSlack attachments配列。指定すると既存のattachmentsを置き換えます。"`
}

// UpdateSlackMessageOutput is the structured output for update_slack_message.
type UpdateSlackMessageOutput = client.UpdateWebAPIMessageResponse

// DeleteSlackMessageInput is the input for delete_slack_message.
type DeleteSlackMessageInput struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"削除対象のチャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	TS        string `json:"ts" jsonschema:"削除対象メッセージのts。post_slack_message_as_user の戻り値を利用できます。"`
}

// DeleteSlackMessageOutput is the structured output for delete_slack_message.
type DeleteSlackMessageOutput = client.DeleteWebAPIMessageResponse

func (t *SlackTools) previewSlackMessage(_ context.Context, _ *mcp.CallToolRequest, in PostSlackMessageInput) (*mcp.CallToolResult, PreviewSlackMessageOutput, error) {
	payload, err := t.client.PreviewMessage(in.toMessage(in.ThreadTS))
	if err != nil {
		return nil, PreviewSlackMessageOutput{}, err
	}

	return nil, PreviewSlackMessageOutput{
		OK:        true,
		Transport: "webhook",
		Payload:   payload,
	}, nil
}

func (t *SlackTools) postSlackMessage(ctx context.Context, _ *mcp.CallToolRequest, in PostSlackMessageInput) (*mcp.CallToolResult, PostSlackMessageOutput, error) {
	out, err := t.client.PostMessage(ctx, in.toMessage(in.ThreadTS))
	if err != nil {
		return nil, PostSlackMessageOutput{}, err
	}

	return nil, PostSlackMessageOutput{
		OK:         true,
		StatusCode: out.StatusCode,
		Body:       out.Body,
	}, nil
}

func (t *SlackTools) previewSlackMessageAsUser(_ context.Context, _ *mcp.CallToolRequest, in PostSlackMessageAsUserInput) (*mcp.CallToolResult, PreviewSlackMessageAsUserOutput, error) {
	payload, err := t.client.PreviewWebAPIMessage(in.toWebAPIMessage(in.ChannelID, in.ThreadTS))
	if err != nil {
		return nil, PreviewSlackMessageAsUserOutput{}, err
	}

	return nil, PreviewSlackMessageAsUserOutput{
		OK:        true,
		Transport: "web_api",
		ChannelID: payload.ChannelID,
		Payload:   payload,
	}, nil
}

func (t *SlackTools) postSlackMessageAsUser(ctx context.Context, _ *mcp.CallToolRequest, in PostSlackMessageAsUserInput) (*mcp.CallToolResult, PostSlackMessageAsUserOutput, error) {
	out, err := t.client.PostWebAPIMessage(ctx, in.toWebAPIMessage(in.ChannelID, in.ThreadTS))
	if err != nil {
		return nil, PostSlackMessageAsUserOutput{}, err
	}

	return nil, *out, nil
}

func (t *SlackTools) updateSlackMessage(ctx context.Context, _ *mcp.CallToolRequest, in UpdateSlackMessageInput) (*mcp.CallToolResult, UpdateSlackMessageOutput, error) {
	out, err := t.client.UpdateWebAPIMessage(ctx, client.UpdateWebAPIMessage{
		ChannelID:   in.ChannelID,
		TS:          in.TS,
		Text:        in.Text,
		Blocks:      in.Blocks,
		Attachments: in.Attachments,
	})
	if err != nil {
		return nil, UpdateSlackMessageOutput{}, err
	}

	return nil, *out, nil
}

func (t *SlackTools) deleteSlackMessage(ctx context.Context, _ *mcp.CallToolRequest, in DeleteSlackMessageInput) (*mcp.CallToolResult, DeleteSlackMessageOutput, error) {
	out, err := t.client.DeleteWebAPIMessage(ctx, in.ChannelID, in.TS)
	if err != nil {
		return nil, DeleteSlackMessageOutput{}, err
	}

	return nil, *out, nil
}
