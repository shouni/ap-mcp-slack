package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	slackapi "github.com/slack-go/slack"
)

const (
	defaultChannelListLimit = 200
	maxChannelListLimit     = 1000
	channelListPageSize     = 200
	defaultMessageListLimit = 100
	maxMessageListLimit     = 1000
)

// Sort values accepted by list_slack_channels' sort option.
const (
	ChannelSortNone        = "none"
	ChannelSortNameAsc     = "name_asc"
	ChannelSortNameDesc    = "name_desc"
	ChannelSortCreatedAsc  = "created_asc"
	ChannelSortCreatedDesc = "created_desc"
)

// webAPITransport posts, deletes, and lists messages/channels through the
// token-authenticated Slack Web API.
type webAPITransport struct {
	token            string
	defaultChannelID string
	sourceLabel      string
	slackAPIClient   *slackapi.Client
}

func newWebAPITransport(cfg SlackClientConfig) webAPITransport {
	httpClient := &http.Client{Timeout: requestTimeout}
	slackOptions := []slackapi.Option{slackapi.OptionHTTPClient(httpClient)}
	if apiBaseURL := normalizeSlackAPIBaseURL(cfg.APIBaseURL); apiBaseURL != "" {
		slackOptions = append(slackOptions, slackapi.OptionAPIURL(apiBaseURL))
	}
	token := strings.TrimSpace(cfg.Token)

	return webAPITransport{
		token:            token,
		defaultChannelID: strings.TrimSpace(cfg.DefaultChannelID),
		sourceLabel:      strings.TrimSpace(cfg.SourceLabel),
		slackAPIClient:   slackapi.New(token, slackOptions...),
	}
}

// requireToken reports an error if no Web API token was configured. All Web API
// operations (post-as-user, delete, list) need one, so they share this check.
func (w *webAPITransport) requireToken() error {
	if w.token == "" {
		return fmt.Errorf("slack: token is required")
	}
	return nil
}

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

// ListChannelsOptions configures Slack conversations.list requests.
type ListChannelsOptions struct {
	Types           []string `json:"types,omitempty"`
	ExcludeArchived bool     `json:"exclude_archived,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	Cursor          string   `json:"cursor,omitempty"`
	TeamID          string   `json:"team_id,omitempty"`
	Sort            string   `json:"sort,omitempty"`
}

// SlackChannelSummary contains the channel fields returned by list_slack_channels.
type SlackChannelSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	NameNormalized string `json:"name_normalized,omitempty"`
	User           string `json:"user,omitempty"`
	Created        int64  `json:"created,omitempty"`
	NumMembers     int    `json:"num_members,omitempty"`
	IsChannel      bool   `json:"is_channel,omitempty"`
	IsGroup        bool   `json:"is_group,omitempty"`
	IsIM           bool   `json:"is_im,omitempty"`
	IsMPIM         bool   `json:"is_mpim,omitempty"`
	IsPrivate      bool   `json:"is_private,omitempty"`
	IsArchived     bool   `json:"is_archived,omitempty"`
	IsGeneral      bool   `json:"is_general,omitempty"`
	IsMember       bool   `json:"is_member,omitempty"`
	IsShared       bool   `json:"is_shared,omitempty"`
	IsExtShared    bool   `json:"is_ext_shared,omitempty"`
	IsOrgShared    bool   `json:"is_org_shared,omitempty"`
}

// ListChannelsResponse contains the relevant conversations.list response fields.
type ListChannelsResponse struct {
	OK         bool                  `json:"ok"`
	Channels   []SlackChannelSummary `json:"channels"`
	Names      []string              `json:"names"`
	Count      int                   `json:"count"`
	NextCursor string                `json:"next_cursor,omitempty"`
	Sort       string                `json:"sort"`
}

// ListJoinedChannelsOptions configures Slack users.conversations requests.
type ListJoinedChannelsOptions struct {
	Types           []string `json:"types,omitempty"`
	ExcludeArchived bool     `json:"exclude_archived,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	Cursor          string   `json:"cursor,omitempty"`
	TeamID          string   `json:"team_id,omitempty"`
	Sort            string   `json:"sort,omitempty"`
}

// ConversationHistoryOptions configures Slack conversations.history requests.
type ConversationHistoryOptions struct {
	ChannelID          string `json:"channel_id,omitempty"`
	Limit              int    `json:"limit,omitempty"`
	Cursor             string `json:"cursor,omitempty"`
	Oldest             string `json:"oldest,omitempty"`
	Latest             string `json:"latest,omitempty"`
	Inclusive          bool   `json:"inclusive,omitempty"`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty"`
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
}

// SlackMessageSummary contains the message fields returned by history/replies tools.
type SlackMessageSummary struct {
	Type       string   `json:"type,omitempty"`
	SubType    string   `json:"subtype,omitempty"`
	User       string   `json:"user,omitempty"`
	BotID      string   `json:"bot_id,omitempty"`
	Username   string   `json:"username,omitempty"`
	Text       string   `json:"text,omitempty"`
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

// channelPageFetcher fetches one page of channels at cursor, requesting at most
// requestLimit items, returning the page and Slack's cursor for the next one.
type channelPageFetcher func(ctx context.Context, cursor string, requestLimit int) (channels []slackapi.Channel, nextCursor string, err error)

// ListChannels lists Slack channel-like conversations through conversations.list.
func (w *webAPITransport) ListChannels(ctx context.Context, opts ListChannelsOptions) (*ListChannelsResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}

	limit, types, sortBy, err := normalizeChannelListParams(opts.Limit, opts.Types, opts.Sort)
	if err != nil {
		return nil, err
	}
	teamID := strings.TrimSpace(opts.TeamID)

	return w.collectChannelPages(ctx, "conversations.list", limit, strings.TrimSpace(opts.Cursor), sortBy,
		func(ctx context.Context, cursor string, requestLimit int) ([]slackapi.Channel, string, error) {
			return w.slackAPIClient.GetConversationsContext(ctx, &slackapi.GetConversationsParameters{
				Cursor:          cursor,
				ExcludeArchived: opts.ExcludeArchived,
				Limit:           requestLimit,
				Types:           types,
				TeamID:          teamID,
			})
		})
}

// ListJoinedChannels lists the conversations the token owner (the calling user, for a
// user token, or the bot, for a bot token) is a member of, through users.conversations.
// Unlike ListChannels/conversations.list, this is scoped server-side to the caller's
// own memberships rather than the whole workspace, so every returned channel already
// has IsMember set.
func (w *webAPITransport) ListJoinedChannels(ctx context.Context, opts ListJoinedChannelsOptions) (*ListChannelsResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}

	limit, types, sortBy, err := normalizeChannelListParams(opts.Limit, opts.Types, opts.Sort)
	if err != nil {
		return nil, err
	}
	teamID := strings.TrimSpace(opts.TeamID)

	return w.collectChannelPages(ctx, "users.conversations", limit, strings.TrimSpace(opts.Cursor), sortBy,
		func(ctx context.Context, cursor string, requestLimit int) ([]slackapi.Channel, string, error) {
			return w.slackAPIClient.GetConversationsForUserContext(ctx, &slackapi.GetConversationsForUserParameters{
				Cursor:          cursor,
				ExcludeArchived: opts.ExcludeArchived,
				Limit:           requestLimit,
				Types:           types,
				TeamID:          teamID,
			})
		})
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
	messages := summarizeMessages(resp.Messages)
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
	messages := summarizeMessages(apiMessages)
	return &ConversationMessagesResponse{
		OK:         true,
		Messages:   messages,
		Count:      len(messages),
		HasMore:    hasMore,
		NextCursor: strings.TrimSpace(nextCursor),
	}, nil
}

// normalizeChannelListParams validates and applies defaults to the limit/types/sort
// options shared by ListChannels and ListJoinedChannels.
func normalizeChannelListParams(rawLimit int, rawTypes []string, rawSort string) (int, []string, string, error) {
	limit, err := normalizeListLimit(rawLimit, defaultChannelListLimit, maxChannelListLimit)
	if err != nil {
		return 0, nil, "", err
	}
	types, err := normalizeChannelTypes(rawTypes)
	if err != nil {
		return 0, nil, "", err
	}
	sortBy, err := normalizeChannelSort(rawSort)
	if err != nil {
		return 0, nil, "", err
	}
	return limit, types, sortBy, nil
}

// collectChannelPages pages through fetch, starting at cursor, until limit channels
// have been collected or Slack has no more pages, then sorts and summarizes them.
// Slack's pagination guide notes a page may return more items than requested, so a
// page's items are all kept even if that pushes the total past limit: dropping the
// overshoot would permanently lose it, since the next call resumes after nextCursor
// regardless of what this method chose to do with the current page.
func (w *webAPITransport) collectChannelPages(ctx context.Context, apiMethod string, limit int, cursor string, sortBy string, fetch channelPageFetcher) (*ListChannelsResponse, error) {
	channels := make([]SlackChannelSummary, 0, limit)
	seenCursors := map[string]struct{}{}

	for len(channels) < limit {
		requestLimit := min(channelListPageSize, limit-len(channels))
		apiChannels, nextCursor, err := fetch(ctx, cursor, requestLimit)
		if err != nil {
			return nil, fmt.Errorf("slack: %s failed: %w", apiMethod, err)
		}

		for _, channel := range apiChannels {
			channels = append(channels, summarizeChannel(channel))
		}

		nextCursor = strings.TrimSpace(nextCursor)
		if nextCursor == "" {
			cursor = ""
			break
		}
		if _, ok := seenCursors[nextCursor]; ok {
			return nil, fmt.Errorf("slack: %s returned duplicate cursor %q", apiMethod, nextCursor)
		}
		seenCursors[nextCursor] = struct{}{}
		cursor = nextCursor
	}

	sortChannels(channels, sortBy)
	names := channelNames(channels)

	return &ListChannelsResponse{
		OK:         true,
		Channels:   channels,
		Names:      names,
		Count:      len(channels),
		NextCursor: cursor,
		Sort:       sortBy,
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

func (w *webAPITransport) channelIDOrDefault(channelID string) string {
	channelID = strings.TrimSpace(channelID)
	if channelID != "" {
		return channelID
	}
	return w.defaultChannelID
}

func normalizeSlackAPIBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.TrimRight(raw, "/") + "/"
}

func normalizeChannelTypes(types []string) ([]string, error) {
	if len(types) == 0 {
		return nil, nil
	}

	validTypes := map[string]struct{}{
		"public_channel":  {},
		"private_channel": {},
		"mpim":            {},
		"im":              {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(types))
	for _, rawType := range types {
		for part := range strings.SplitSeq(rawType, ",") {
			channelType := strings.ToLower(strings.TrimSpace(part))
			if channelType == "" {
				continue
			}
			if _, ok := validTypes[channelType]; !ok {
				return nil, fmt.Errorf("slack: unsupported channel type %q", channelType)
			}
			if _, ok := seen[channelType]; ok {
				continue
			}
			seen[channelType] = struct{}{}
			out = append(out, channelType)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func normalizeChannelSort(raw string) (string, error) {
	sortBy := strings.ToLower(strings.TrimSpace(raw))
	if sortBy == "" {
		return ChannelSortNameAsc, nil
	}
	switch sortBy {
	case ChannelSortNone, ChannelSortNameAsc, ChannelSortNameDesc, ChannelSortCreatedAsc, ChannelSortCreatedDesc:
		return sortBy, nil
	default:
		return "", fmt.Errorf("slack: unsupported sort %q", raw)
	}
}

func summarizeChannel(channel slackapi.Channel) SlackChannelSummary {
	return SlackChannelSummary{
		ID:             channel.ID,
		Name:           channel.Name,
		NameNormalized: channel.NameNormalized,
		User:           channel.User,
		Created:        int64(channel.Created),
		NumMembers:     channel.NumMembers,
		IsChannel:      channel.IsChannel,
		IsGroup:        channel.IsGroup,
		IsIM:           channel.IsIM,
		IsMPIM:         channel.IsMpIM,
		IsPrivate:      channel.IsPrivate,
		IsArchived:     channel.IsArchived,
		IsGeneral:      channel.IsGeneral,
		IsMember:       channel.IsMember,
		IsShared:       channel.IsShared,
		IsExtShared:    channel.IsExtShared,
		IsOrgShared:    channel.IsOrgShared,
	}
}

func summarizeMessages(messages []slackapi.Message) []SlackMessageSummary {
	out := make([]SlackMessageSummary, 0, len(messages))
	for _, message := range messages {
		out = append(out, SlackMessageSummary{
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
		})
	}
	return out
}

func sortChannels(channels []SlackChannelSummary, sortBy string) {
	switch sortBy {
	case ChannelSortNone:
		return
	case ChannelSortNameDesc:
		sort.SliceStable(channels, func(i, j int) bool {
			return compareChannelName(channels[i], channels[j]) > 0
		})
	case ChannelSortCreatedAsc:
		sort.SliceStable(channels, func(i, j int) bool {
			if channels[i].Created == channels[j].Created {
				return channels[i].ID < channels[j].ID
			}
			return channels[i].Created < channels[j].Created
		})
	case ChannelSortCreatedDesc:
		sort.SliceStable(channels, func(i, j int) bool {
			if channels[i].Created == channels[j].Created {
				return channels[i].ID < channels[j].ID
			}
			return channels[i].Created > channels[j].Created
		})
	default:
		sort.SliceStable(channels, func(i, j int) bool {
			return compareChannelName(channels[i], channels[j]) < 0
		})
	}
}

func compareChannelName(left SlackChannelSummary, right SlackChannelSummary) int {
	leftName := channelNameKey(left)
	rightName := channelNameKey(right)
	if leftName < rightName {
		return -1
	}
	if leftName > rightName {
		return 1
	}
	if left.ID < right.ID {
		return -1
	}
	if left.ID > right.ID {
		return 1
	}
	return 0
}

func channelNameKey(channel SlackChannelSummary) string {
	for _, value := range []string{channel.Name, channel.NameNormalized, channel.User, channel.ID} {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			return value
		}
	}
	return ""
}

func channelNames(channels []SlackChannelSummary) []string {
	names := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel.Name != "" {
			names = append(names, channel.Name)
		}
	}
	return names
}

func buildPostMessageOptions(msg WebAPIMessage, sourceLabel string) ([]slackapi.MsgOption, error) {
	options := []slackapi.MsgOption{
		slackapi.MsgOptionText(msg.Text, false),
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

	blocks, err := convertBlocks(msg.Blocks)
	if err != nil {
		return nil, err
	}
	blocks = appendSourceLabelBlock(blocks, msg.Text, sourceLabel)
	if len(blocks) > 0 {
		options = append(options, slackapi.MsgOptionBlocks(blocks...))
	}

	attachments, err := convertAttachments(msg.Attachments)
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
