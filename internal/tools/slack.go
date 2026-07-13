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
		Name:        "preview_slack_message",
		Description: "post_slack_message で送信される Slack Incoming Webhook payload を、Slackへ投稿せずに確認します。",
	}, t.previewSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message",
		Description: "MCP_SLACK_WEBHOOK_URL の Slack Incoming Webhook にメッセージを投稿します。",
	}, t.postSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "preview_slack_message_as_user",
		Description: "post_slack_message_as_user で送信される Slack Web API chat.postMessage payload を、Slackへ投稿せずに確認します。",
	}, t.previewSlackMessageAsUser)

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
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API conversations.list でワークスペース全体のチャンネル一覧を取得します。並び順は取得した結果にローカルで適用します。自分（トークン所有者）が参加しているチャンネルだけが欲しい場合は list_joined_slack_channels を使ってください。",
	}, t.listSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_joined_slack_channels",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API users.conversations でトークン所有者が参加しているチャンネル一覧のみを取得します（サーバー側でメンバーシップに絞り込まれます）。MCP_SLACK_USER_TOKEN（ユーザートークン）を設定している場合はそのユーザー本人が参加しているチャンネル、ボットトークンのみの場合はそのボットが参加しているチャンネルが対象です。",
	}, t.listJoinedSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_history",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API conversations.history でチャンネルのメッセージ履歴を取得します。public channel は channels:history、private channel は groups:history スコープが必要です。",
	}, t.getSlackChannelHistory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_thread_replies",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API conversations.replies で指定メッセージのスレッド返信を取得します。public channel は channels:history、private channel は groups:history スコープが必要です。",
	}, t.getSlackThreadReplies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_users",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API users.list でワークスペースメンバー一覧を取得します。deleted（deactivate済み）ユーザーはデフォルトで除外されます。要 users:read スコープ。",
	}, t.listSlackUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_slack_user_by_email",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API users.lookupByEmail でメールアドレスから単一ユーザーを検索します。要 users:read.email スコープ。",
	}, t.lookupSlackUserByEmail)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve_slack_user",
		Description: "name または email から Slack ユーザーを一意に解決します。email が指定された場合は users.lookupByEmail を優先し、無ければ users.list から完全一致→部分一致の順で検索します。候補が複数ある場合は自動選択せず候補一覧を返します（曖昧なまま送信しないでください）。戻り値の mention はそのまま <@U...> 形式でメッセージに埋め込めます。要 users:read（および email 指定時は users:read.email）スコープ。",
	}, t.resolveSlackUser)
}

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
type PostSlackMessageAsUserOutput struct {
	OK        bool   `json:"ok"`
	ChannelID string `json:"channel_id,omitempty"`
	TS        string `json:"ts,omitempty"`
}

// PreviewSlackMessageAsUserOutput is the structured output for preview_slack_message_as_user.
type PreviewSlackMessageAsUserOutput struct {
	OK        bool                 `json:"ok"`
	Transport string               `json:"transport"`
	ChannelID string               `json:"channel_id"`
	Payload   client.WebAPIMessage `json:"payload"`
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

// ListJoinedSlackChannelsInput is the input for list_joined_slack_channels.
type ListJoinedSlackChannelsInput struct {
	Types           []string `json:"types,omitempty" jsonschema:"取得する会話種別。public_channel, private_channel, mpim, im を指定できます。省略時はSlack APIのデフォルト public_channel です。"`
	ExcludeArchived bool     `json:"exclude_archived,omitempty" jsonschema:"trueの場合、アーカイブ済みチャンネルを除外します。"`
	Limit           int      `json:"limit,omitempty" jsonschema:"最大取得件数。省略時は200、最大1000です。"`
	Cursor          string   `json:"cursor,omitempty" jsonschema:"続きから取得する場合のSlack pagination cursorです。"`
	TeamID          string   `json:"team_id,omitempty" jsonschema:"Enterprise Gridのorg-level tokenで対象ワークスペースを指定する場合のteam idです。"`
	Sort            string   `json:"sort,omitempty" jsonschema:"取得した結果に適用する返却前の並び順。none, name_asc, name_desc, created_asc, created_desc を指定できます。省略時は name_asc です。"`
}

// ListJoinedSlackChannelsOutput is the structured output for list_joined_slack_channels.
type ListJoinedSlackChannelsOutput struct {
	OK         bool                         `json:"ok"`
	Channels   []client.SlackChannelSummary `json:"channels"`
	Names      []string                     `json:"names"`
	Count      int                          `json:"count"`
	NextCursor string                       `json:"next_cursor,omitempty"`
	Sort       string                       `json:"sort"`
}

// GetSlackChannelHistoryInput is the input for get_slack_channel_history.
type GetSlackChannelHistoryInput struct {
	ChannelID          string `json:"channel_id,omitempty" jsonschema:"取得対象チャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	Limit              int    `json:"limit,omitempty" jsonschema:"最大取得件数。省略時は100、最大1000です。"`
	Cursor             string `json:"cursor,omitempty" jsonschema:"続きから取得する場合のSlack pagination cursorです。"`
	Oldest             string `json:"oldest,omitempty" jsonschema:"このUnix timestampより後のメッセージのみ取得します。例: 1700000000.000100"`
	Latest             string `json:"latest,omitempty" jsonschema:"このUnix timestampより前のメッセージのみ取得します。例: 1700000000.000100"`
	Inclusive          bool   `json:"inclusive,omitempty" jsonschema:"oldest/latest と同じtimestampのメッセージも含めます。"`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty" jsonschema:"trueの場合、Slackのメッセージメタデータも取得対象にします。"`
}

// GetSlackThreadRepliesInput is the input for get_slack_thread_replies.
type GetSlackThreadRepliesInput struct {
	ChannelID          string `json:"channel_id,omitempty" jsonschema:"取得対象チャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	TS                 string `json:"ts" jsonschema:"親メッセージのts。返信メッセージのtsではなくスレッド親のtsを指定してください。"`
	Limit              int    `json:"limit,omitempty" jsonschema:"最大取得件数。省略時は100、最大1000です。"`
	Cursor             string `json:"cursor,omitempty" jsonschema:"続きから取得する場合のSlack pagination cursorです。"`
	Oldest             string `json:"oldest,omitempty" jsonschema:"このUnix timestampより後の返信のみ取得します。例: 1700000000.000100"`
	Latest             string `json:"latest,omitempty" jsonschema:"このUnix timestampより前の返信のみ取得します。例: 1700000000.000100"`
	Inclusive          bool   `json:"inclusive,omitempty" jsonschema:"oldest/latest と同じtimestampの返信も含めます。"`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty" jsonschema:"trueの場合、Slackのメッセージメタデータも取得対象にします。"`
}

// GetSlackMessagesOutput is the structured output for history/replies tools.
type GetSlackMessagesOutput struct {
	OK         bool                         `json:"ok"`
	Messages   []client.SlackMessageSummary `json:"messages"`
	Count      int                          `json:"count"`
	HasMore    bool                         `json:"has_more"`
	NextCursor string                       `json:"next_cursor,omitempty"`
}

// ListSlackUsersInput is the input for list_slack_users.
type ListSlackUsersInput struct {
	Query          string `json:"query,omitempty" jsonschema:"name / real_name / display_name / email に対する部分一致検索クエリ（大文字小文字を区別しません）。"`
	Limit          int    `json:"limit,omitempty" jsonschema:"最大取得件数。省略時は200、最大1000です。"`
	Cursor         string `json:"cursor,omitempty" jsonschema:"続きから取得する場合のSlack pagination cursorです。"`
	TeamID         string `json:"team_id,omitempty" jsonschema:"Enterprise Gridのorg-level tokenで対象ワークスペースを指定する場合のteam idです。"`
	IncludeDeleted bool   `json:"include_deleted,omitempty" jsonschema:"trueの場合、deactivate済み(deleted)ユーザーも含めます。省略時は除外されます。"`
}

// ListSlackUsersOutput is the structured output for list_slack_users.
type ListSlackUsersOutput struct {
	OK         bool                      `json:"ok"`
	Users      []client.SlackUserSummary `json:"users"`
	Count      int                       `json:"count"`
	NextCursor string                    `json:"next_cursor,omitempty"`
}

// LookupSlackUserByEmailInput is the input for lookup_slack_user_by_email.
type LookupSlackUserByEmailInput struct {
	Email string `json:"email" jsonschema:"検索対象のメールアドレス。"`
}

// LookupSlackUserByEmailOutput is the structured output for lookup_slack_user_by_email.
type LookupSlackUserByEmailOutput struct {
	OK   bool                     `json:"ok"`
	User *client.SlackUserSummary `json:"user,omitempty"`
}

// ResolveSlackUserInput is the input for resolve_slack_user.
type ResolveSlackUserInput struct {
	Name   string `json:"name,omitempty" jsonschema:"検索対象のユーザー名・real name・display nameのいずれか。emailが指定された場合は無視されます。"`
	Email  string `json:"email,omitempty" jsonschema:"検索対象のメールアドレス。指定された場合はnameより優先されます。"`
	TeamID string `json:"team_id,omitempty" jsonschema:"Enterprise Gridのorg-level tokenで対象ワークスペースを指定する場合のteam idです。nameでの検索時のみ利用します。"`
}

// ResolveSlackUserOutput is the structured output for resolve_slack_user. Status is
// one of "found", "ambiguous", or "not_found"; User/Mention are set only when
// status is "found", and Candidates only when status is "ambiguous".
type ResolveSlackUserOutput struct {
	OK         bool                      `json:"ok"`
	Status     string                    `json:"status"`
	User       *client.SlackUserSummary  `json:"user,omitempty"`
	Mention    string                    `json:"mention,omitempty"`
	Candidates []client.SlackUserSummary `json:"candidates,omitempty"`
}

func (t *SlackTools) previewSlackMessage(_ context.Context, _ *mcp.CallToolRequest, in PostSlackMessageInput) (*mcp.CallToolResult, PreviewSlackMessageOutput, error) {
	payload, err := t.client.PreviewMessage(client.Message{
		Text:        in.Text,
		Blocks:      in.Blocks,
		Attachments: in.Attachments,
		ThreadTS:    in.ThreadTS,
		IconEmoji:   in.IconEmoji,
		UnfurlLinks: in.UnfurlLinks,
		UnfurlMedia: in.UnfurlMedia,
	})
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

func (t *SlackTools) previewSlackMessageAsUser(_ context.Context, _ *mcp.CallToolRequest, in PostSlackMessageAsUserInput) (*mcp.CallToolResult, PreviewSlackMessageAsUserOutput, error) {
	payload, err := t.client.PreviewWebAPIMessage(client.WebAPIMessage{
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

func (t *SlackTools) listJoinedSlackChannels(ctx context.Context, _ *mcp.CallToolRequest, in ListJoinedSlackChannelsInput) (*mcp.CallToolResult, ListJoinedSlackChannelsOutput, error) {
	out, err := t.client.ListJoinedChannels(ctx, client.ListJoinedChannelsOptions{
		Types:           in.Types,
		ExcludeArchived: in.ExcludeArchived,
		Limit:           in.Limit,
		Cursor:          in.Cursor,
		TeamID:          in.TeamID,
		Sort:            in.Sort,
	})
	if err != nil {
		return nil, ListJoinedSlackChannelsOutput{}, err
	}

	return nil, ListJoinedSlackChannelsOutput{
		OK:         out.OK,
		Channels:   out.Channels,
		Names:      out.Names,
		Count:      out.Count,
		NextCursor: out.NextCursor,
		Sort:       out.Sort,
	}, nil
}

func (t *SlackTools) getSlackChannelHistory(ctx context.Context, _ *mcp.CallToolRequest, in GetSlackChannelHistoryInput) (*mcp.CallToolResult, GetSlackMessagesOutput, error) {
	out, err := t.client.GetConversationHistory(ctx, client.ConversationHistoryOptions{
		ChannelID:          in.ChannelID,
		Limit:              in.Limit,
		Cursor:             in.Cursor,
		Oldest:             in.Oldest,
		Latest:             in.Latest,
		Inclusive:          in.Inclusive,
		IncludeAllMetadata: in.IncludeAllMetadata,
	})
	if err != nil {
		return nil, GetSlackMessagesOutput{}, err
	}

	return nil, GetSlackMessagesOutput{
		OK:         out.OK,
		Messages:   out.Messages,
		Count:      out.Count,
		HasMore:    out.HasMore,
		NextCursor: out.NextCursor,
	}, nil
}

func (t *SlackTools) getSlackThreadReplies(ctx context.Context, _ *mcp.CallToolRequest, in GetSlackThreadRepliesInput) (*mcp.CallToolResult, GetSlackMessagesOutput, error) {
	out, err := t.client.GetConversationReplies(ctx, client.ConversationRepliesOptions{
		ChannelID:          in.ChannelID,
		TS:                 in.TS,
		Limit:              in.Limit,
		Cursor:             in.Cursor,
		Oldest:             in.Oldest,
		Latest:             in.Latest,
		Inclusive:          in.Inclusive,
		IncludeAllMetadata: in.IncludeAllMetadata,
	})
	if err != nil {
		return nil, GetSlackMessagesOutput{}, err
	}

	return nil, GetSlackMessagesOutput{
		OK:         out.OK,
		Messages:   out.Messages,
		Count:      out.Count,
		HasMore:    out.HasMore,
		NextCursor: out.NextCursor,
	}, nil
}

func (t *SlackTools) listSlackUsers(ctx context.Context, _ *mcp.CallToolRequest, in ListSlackUsersInput) (*mcp.CallToolResult, ListSlackUsersOutput, error) {
	out, err := t.client.ListUsers(ctx, client.ListUsersOptions{
		Limit:          in.Limit,
		Cursor:         in.Cursor,
		TeamID:         in.TeamID,
		IncludeDeleted: in.IncludeDeleted,
		Query:          in.Query,
	})
	if err != nil {
		return nil, ListSlackUsersOutput{}, err
	}

	return nil, ListSlackUsersOutput{
		OK:         out.OK,
		Users:      out.Users,
		Count:      out.Count,
		NextCursor: out.NextCursor,
	}, nil
}

func (t *SlackTools) lookupSlackUserByEmail(ctx context.Context, _ *mcp.CallToolRequest, in LookupSlackUserByEmailInput) (*mcp.CallToolResult, LookupSlackUserByEmailOutput, error) {
	user, err := t.client.LookupUserByEmail(ctx, in.Email)
	if err != nil {
		return nil, LookupSlackUserByEmailOutput{}, err
	}

	return nil, LookupSlackUserByEmailOutput{OK: true, User: user}, nil
}

func (t *SlackTools) resolveSlackUser(ctx context.Context, _ *mcp.CallToolRequest, in ResolveSlackUserInput) (*mcp.CallToolResult, ResolveSlackUserOutput, error) {
	out, err := t.client.ResolveUser(ctx, in.Name, in.Email, in.TeamID)
	if err != nil {
		return nil, ResolveSlackUserOutput{}, err
	}

	return nil, ResolveSlackUserOutput{
		OK:         out.OK,
		Status:     out.Status,
		User:       out.User,
		Mention:    out.Mention,
		Candidates: out.Candidates,
	}, nil
}
