package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

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
type ListSlackChannelsOutput = client.ListChannelsResponse

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
type ListJoinedSlackChannelsOutput = client.ListChannelsResponse

// GetSlackChannelInfoInput is the input for get_slack_channel_info.
type GetSlackChannelInfoInput struct {
	ChannelID         string `json:"channel_id,omitempty" jsonschema:"取得対象チャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	IncludeNumMembers bool   `json:"include_num_members,omitempty" jsonschema:"trueの場合、num_membersを含めて取得します。"`
	IncludeLocale     bool   `json:"include_locale,omitempty" jsonschema:"trueの場合、ロケール情報を含めて取得します。"`
}

// GetSlackChannelInfoOutput is the structured output for get_slack_channel_info.
type GetSlackChannelInfoOutput = client.GetChannelInfoResponse

// GetSlackChannelHistoryInput is the input for get_slack_channel_history.
type GetSlackChannelHistoryInput struct {
	ChannelID          string `json:"channel_id,omitempty" jsonschema:"取得対象チャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	Limit              int    `json:"limit,omitempty" jsonschema:"最大取得件数。省略時は100、最大1000です。"`
	Cursor             string `json:"cursor,omitempty" jsonschema:"続きから取得する場合のSlack pagination cursorです。"`
	Oldest             string `json:"oldest,omitempty" jsonschema:"このUnix timestampより後のメッセージのみ取得します。例: 1700000000.000100"`
	Latest             string `json:"latest,omitempty" jsonschema:"このUnix timestampより前のメッセージのみ取得します。例: 1700000000.000100"`
	Inclusive          bool   `json:"inclusive,omitempty" jsonschema:"oldest/latest と同じtimestampのメッセージも含めます。"`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty" jsonschema:"trueの場合、Slackのメッセージメタデータも取得対象にします。"`
	IncludeRawBlocks   bool   `json:"include_raw_blocks,omitempty" jsonschema:"trueの場合、Block Kit blocksとattachmentsの生データも取得対象にします。省略時はテキストとして要約されたものだけを返します。"`
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
	IncludeRawBlocks   bool   `json:"include_raw_blocks,omitempty" jsonschema:"trueの場合、Block Kit blocksとattachmentsの生データも取得対象にします。省略時はテキストとして要約されたものだけを返します。"`
}

// GetSlackMessagesOutput is the structured output for history/replies tools.
type GetSlackMessagesOutput = client.ConversationMessagesResponse

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

	return nil, *out, nil
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

	return nil, *out, nil
}

func (t *SlackTools) getSlackChannelInfo(ctx context.Context, _ *mcp.CallToolRequest, in GetSlackChannelInfoInput) (*mcp.CallToolResult, GetSlackChannelInfoOutput, error) {
	out, err := t.client.GetChannelInfo(ctx, client.GetChannelInfoOptions{
		ChannelID:         in.ChannelID,
		IncludeNumMembers: in.IncludeNumMembers,
		IncludeLocale:     in.IncludeLocale,
	})
	if err != nil {
		return nil, GetSlackChannelInfoOutput{}, err
	}

	return nil, *out, nil
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
		IncludeRawBlocks:   in.IncludeRawBlocks,
	})
	if err != nil {
		return nil, GetSlackMessagesOutput{}, err
	}

	return nil, *out, nil
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
		IncludeRawBlocks:   in.IncludeRawBlocks,
	})
	if err != nil {
		return nil, GetSlackMessagesOutput{}, err
	}

	return nil, *out, nil
}
