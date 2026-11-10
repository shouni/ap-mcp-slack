package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/client"
)

// ListSlackChannelsInput is the input for list_slack_channels and
// list_joined_slack_channels, which take the same parameters and differ only in
// whether Slack scopes the listing to the token owner's memberships.
type ListSlackChannelsInput struct {
	Types           []string `json:"types,omitempty" jsonschema:"The conversation types to fetch. Accepts public_channel, private_channel, mpim, im. Defaults to the Slack API default, public_channel."`
	ExcludeArchived bool     `json:"exclude_archived,omitempty" jsonschema:"When true, archived channels are excluded."`
	Limit           int      `json:"limit,omitempty" jsonschema:"The maximum number of items. Defaults to 200, at most 1000. Because of the remainder of what Slack returns in one page, the number returned can slightly exceed it."`
	Cursor          string   `json:"cursor,omitempty" jsonschema:"The Slack pagination cursor to continue from."`
	TeamID          string   `json:"team_id,omitempty" jsonschema:"The team id naming the target workspace when using an Enterprise Grid org-level token."`
	Sort            string   `json:"sort,omitempty" jsonschema:"The order applied to the fetched results before returning them. Accepts none, name_asc, name_desc, created_asc, created_desc. Defaults to name_asc."`
}

// toOptions converts the shared listing input to the client's request options.
func (in ListSlackChannelsInput) toOptions() client.ListChannelsOptions {
	return client.ListChannelsOptions{
		Types:           in.Types,
		ExcludeArchived: in.ExcludeArchived,
		Limit:           in.Limit,
		Cursor:          in.Cursor,
		TeamID:          in.TeamID,
		Sort:            in.Sort,
	}
}

// ListSlackChannelsOutput is the structured output for list_slack_channels and
// list_joined_slack_channels.
type ListSlackChannelsOutput = client.ListChannelsResponse

// GetSlackChannelInfoInput is the input for get_slack_channel_info.
type GetSlackChannelInfoInput struct {
	ChannelID         string `json:"channel_id,omitempty" jsonschema:"The channel ID to fetch. Defaults to MCP_SLACK_CHANNEL_ID."`
	IncludeNumMembers bool   `json:"include_num_members,omitempty" jsonschema:"When true, num_members is included."`
	IncludeLocale     bool   `json:"include_locale,omitempty" jsonschema:"When true, locale information is included."`
}

// GetSlackChannelInfoOutput is the structured output for get_slack_channel_info.
type GetSlackChannelInfoOutput = client.GetChannelInfoResponse

// GetSlackChannelHistoryInput is the input for get_slack_channel_history.
type GetSlackChannelHistoryInput struct {
	ChannelID          string `json:"channel_id,omitempty" jsonschema:"The channel ID to fetch. Defaults to MCP_SLACK_CHANNEL_ID."`
	Limit              int    `json:"limit,omitempty" jsonschema:"The maximum number of items. Defaults to 100, at most 1000."`
	Cursor             string `json:"cursor,omitempty" jsonschema:"The Slack pagination cursor to continue from."`
	Oldest             string `json:"oldest,omitempty" jsonschema:"Fetch only messages after this Unix timestamp. Example: 1700000000.000100"`
	Latest             string `json:"latest,omitempty" jsonschema:"Fetch only messages before this Unix timestamp. Example: 1700000000.000100"`
	Inclusive          bool   `json:"inclusive,omitempty" jsonschema:"Also include messages with the same timestamp as oldest/latest."`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty" jsonschema:"When true, Slack message metadata is fetched too."`
	IncludeRawBlocks   bool   `json:"include_raw_blocks,omitempty" jsonschema:"When true, the raw Block Kit blocks and attachments are fetched too. By default only the text summary is returned."`
}

// GetSlackThreadRepliesInput is the input for get_slack_thread_replies.
type GetSlackThreadRepliesInput struct {
	ChannelID          string `json:"channel_id,omitempty" jsonschema:"The channel ID to fetch. Defaults to MCP_SLACK_CHANNEL_ID."`
	TS                 string `json:"ts" jsonschema:"The parent message ts. Give the ts of the thread parent, not of a reply."`
	Limit              int    `json:"limit,omitempty" jsonschema:"The maximum number of items. Defaults to 100, at most 1000."`
	Cursor             string `json:"cursor,omitempty" jsonschema:"The Slack pagination cursor to continue from."`
	Oldest             string `json:"oldest,omitempty" jsonschema:"Fetch only replies after this Unix timestamp. Example: 1700000000.000100"`
	Latest             string `json:"latest,omitempty" jsonschema:"Fetch only replies before this Unix timestamp. Example: 1700000000.000100"`
	Inclusive          bool   `json:"inclusive,omitempty" jsonschema:"Also include replies with the same timestamp as oldest/latest."`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty" jsonschema:"When true, Slack message metadata is fetched too."`
	IncludeRawBlocks   bool   `json:"include_raw_blocks,omitempty" jsonschema:"When true, the raw Block Kit blocks and attachments are fetched too. By default only the text summary is returned."`
}

// GetSlackMessagesOutput is the structured output for history/replies tools.
type GetSlackMessagesOutput = client.ConversationMessagesResponse

func (t *SlackTools) listSlackChannels(ctx context.Context, _ *mcp.CallToolRequest, in ListSlackChannelsInput) (*mcp.CallToolResult, ListSlackChannelsOutput, error) {
	out, err := t.client.ListChannels(ctx, in.toOptions())
	if err != nil {
		return nil, ListSlackChannelsOutput{}, err
	}

	return nil, *out, nil
}

func (t *SlackTools) listJoinedSlackChannels(ctx context.Context, _ *mcp.CallToolRequest, in ListSlackChannelsInput) (*mcp.CallToolResult, ListSlackChannelsOutput, error) {
	out, err := t.client.ListJoinedChannels(ctx, in.toOptions())
	if err != nil {
		return nil, ListSlackChannelsOutput{}, err
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
