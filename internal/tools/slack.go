// Package tools registers MCP tools.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

// SlackTools provides Slack-related MCP tools.
type SlackTools struct {
	client *client.SlackClient
}

// NewSlackTools creates SlackTools.
func NewSlackTools(c *client.SlackClient) *SlackTools {
	return &SlackTools{client: c}
}

// Register registers Slack tools on the MCP server.
func (t *SlackTools) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message",
		Description: "MCP_SLACK_WEBHOOK_URL の Slack Incoming Webhook にメッセージを投稿します。",
	}, t.postSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message_as_user",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API chat.postMessage でメッセージを投稿します。成功時に channel_id と ts を返します。",
	}, t.postSlackMessageAsUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_slack_message",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API chat.delete でメッセージを削除します。",
	}, t.deleteSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_channels",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API conversations.list でチャンネル一覧を取得します。並び順は取得した結果にローカルで適用します。",
	}, t.listSlackChannels)
}

// PostSlackMessageInput is the input for post_slack_message.
type PostSlackMessageInput struct {
	Text        string           `json:"text" jsonschema:"Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。"`
	Blocks      []map[string]any `json:"blocks,omitempty" jsonschema:"任意のSlack Block Kit blocks配列。指定する場合もアクセシビリティ用にtextを含めてください。"`
	Attachments []map[string]any `json:"attachments,omitempty" jsonschema:"任意のSlack attachments配列。"`
	ThreadTS    string           `json:"thread_ts,omitempty" jsonschema:"スレッド返信にする場合の親メッセージts。Webhook側で利用可能な場合のみ有効です。"`
	IconEmoji   string           `json:"icon_emoji,omitempty" jsonschema:"投稿者アイコンとして使うSlack絵文字名。例: :robot_face:"`
	UnfurlLinks *bool            `json:"unfurl_links,omitempty" jsonschema:"リンク展開を制御します。"`
	UnfurlMedia *bool            `json:"unfurl_media,omitempty" jsonschema:"メディア展開を制御します。"`
}

// PostSlackMessageOutput is the structured output for post_slack_message.
type PostSlackMessageOutput struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body,omitempty"`
}

// PostSlackMessageAsUserInput is the input for post_slack_message_as_user.
type PostSlackMessageAsUserInput struct {
	ChannelID   string           `json:"channel_id,omitempty" jsonschema:"投稿先チャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	Text        string           `json:"text" jsonschema:"Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。"`
	Blocks      []map[string]any `json:"blocks,omitempty" jsonschema:"任意のSlack Block Kit blocks配列。指定する場合もアクセシビリティ用にtextを含めてください。"`
	Attachments []map[string]any `json:"attachments,omitempty" jsonschema:"任意のSlack attachments配列。"`
	ThreadTS    string           `json:"thread_ts,omitempty" jsonschema:"スレッド返信にする場合の親メッセージts。"`
	IconEmoji   string           `json:"icon_emoji,omitempty" jsonschema:"投稿者アイコンとして使うSlack絵文字名。例: :robot_face:"`
	UnfurlLinks *bool            `json:"unfurl_links,omitempty" jsonschema:"リンク展開を制御します。"`
	UnfurlMedia *bool            `json:"unfurl_media,omitempty" jsonschema:"メディア展開を制御します。"`
}

// PostSlackMessageAsUserOutput is the structured output for post_slack_message_as_user.
type PostSlackMessageAsUserOutput struct {
	OK        bool   `json:"ok"`
	ChannelID string `json:"channel_id,omitempty"`
	TS        string `json:"ts,omitempty"`
}

// DeleteSlackMessageInput is the input for delete_slack_message.
type DeleteSlackMessageInput struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"削除対象のチャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	TS        string `json:"ts" jsonschema:"削除対象メッセージのts。post_slack_message_as_user の戻り値を利用できます。"`
}

// DeleteSlackMessageOutput is the structured output for delete_slack_message.
type DeleteSlackMessageOutput struct {
	OK        bool   `json:"ok"`
	ChannelID string `json:"channel_id,omitempty"`
	TS        string `json:"ts,omitempty"`
}

// ListSlackChannelsInput is the input for list_slack_channels.
type ListSlackChannelsInput struct {
	Types           []string `json:"types,omitempty" jsonschema:"取得する会話種別。public_channel, private_channel, mpim, im を指定できます。省略時はSlack APIのデフォルト public_channel です。"`
	ExcludeArchived bool     `json:"exclude_archived,omitempty" jsonschema:"trueの場合、アーカイブ済みチャンネルを除外します。"`
	Limit           int      `json:"limit,omitempty" jsonschema:"最大取得件数。省略時は200、最大1000です。"`
	Cursor          string   `json:"cursor,omitempty" jsonschema:"続きから取得する場合のSlack pagination cursorです。"`
	TeamID          string   `json:"team_id,omitempty" jsonschema:"Enterprise Gridのorg-level tokenで対象ワークスペースを指定する場合のteam idです。"`
	Sort            string   `json:"sort,omitempty" jsonschema:"取得した結果に適用する返却前の並び順。none, name_asc, name_desc, created_asc, created_desc を指定できます。省略時は name_asc です。"`
}

// ListSlackChannelsOutput is the structured output for list_slack_channels.
type ListSlackChannelsOutput struct {
	OK         bool                         `json:"ok"`
	Channels   []client.SlackChannelSummary `json:"channels"`
	Names      []string                     `json:"names"`
	Count      int                          `json:"count"`
	NextCursor string                       `json:"next_cursor,omitempty"`
	Sort       string                       `json:"sort"`
}

func (t *SlackTools) postSlackMessage(ctx context.Context, _ *mcp.CallToolRequest, in PostSlackMessageInput) (*mcp.CallToolResult, PostSlackMessageOutput, error) {
	out, err := t.client.PostMessage(ctx, client.Message{
		Text:        in.Text,
		Blocks:      in.Blocks,
		Attachments: in.Attachments,
		ThreadTS:    in.ThreadTS,
		IconEmoji:   in.IconEmoji,
		UnfurlLinks: in.UnfurlLinks,
		UnfurlMedia: in.UnfurlMedia,
	})
	if err != nil {
		return nil, PostSlackMessageOutput{}, err
	}

	return nil, PostSlackMessageOutput{
		OK:         true,
		StatusCode: out.StatusCode,
		Body:       out.Body,
	}, nil
}

func (t *SlackTools) postSlackMessageAsUser(ctx context.Context, _ *mcp.CallToolRequest, in PostSlackMessageAsUserInput) (*mcp.CallToolResult, PostSlackMessageAsUserOutput, error) {
	out, err := t.client.PostWebAPIMessage(ctx, client.WebAPIMessage{
		ChannelID:   in.ChannelID,
		Text:        in.Text,
		Blocks:      in.Blocks,
		Attachments: in.Attachments,
		ThreadTS:    in.ThreadTS,
		IconEmoji:   in.IconEmoji,
		UnfurlLinks: in.UnfurlLinks,
		UnfurlMedia: in.UnfurlMedia,
	})
	if err != nil {
		return nil, PostSlackMessageAsUserOutput{}, err
	}

	return nil, PostSlackMessageAsUserOutput{
		OK:        out.OK,
		ChannelID: out.ChannelID,
		TS:        out.TS,
	}, nil
}

func (t *SlackTools) deleteSlackMessage(ctx context.Context, _ *mcp.CallToolRequest, in DeleteSlackMessageInput) (*mcp.CallToolResult, DeleteSlackMessageOutput, error) {
	out, err := t.client.DeleteWebAPIMessage(ctx, in.ChannelID, in.TS)
	if err != nil {
		return nil, DeleteSlackMessageOutput{}, err
	}

	return nil, DeleteSlackMessageOutput{
		OK:        out.OK,
		ChannelID: out.ChannelID,
		TS:        out.TS,
	}, nil
}

func (t *SlackTools) listSlackChannels(ctx context.Context, _ *mcp.CallToolRequest, in ListSlackChannelsInput) (*mcp.CallToolResult, ListSlackChannelsOutput, error) {
	out, err := t.client.ListChannels(ctx, client.ListChannelsOptions{
		Types:           in.Types,
		ExcludeArchived: in.ExcludeArchived,
		Limit:           in.Limit,
		Cursor:          in.Cursor,
		TeamID:          in.TeamID,
		Sort:            in.Sort,
	})
	if err != nil {
		return nil, ListSlackChannelsOutput{}, err
	}

	return nil, ListSlackChannelsOutput{
		OK:         out.OK,
		Channels:   out.Channels,
		Names:      out.Names,
		Count:      out.Count,
		NextCursor: out.NextCursor,
		Sort:       out.Sort,
	}, nil
}
