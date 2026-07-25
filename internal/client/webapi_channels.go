package client

import (
	"context"
	"fmt"
	"sort"
	"strings"

	slackapi "github.com/slack-go/slack"
)

const (
	defaultChannelListLimit = 200
	maxChannelListLimit     = 1000
	channelListPageSize     = 200
)

// Sort values accepted by list_slack_channels' sort option.
const (
	ChannelSortNone        = "none"
	ChannelSortNameAsc     = "name_asc"
	ChannelSortNameDesc    = "name_desc"
	ChannelSortCreatedAsc  = "created_asc"
	ChannelSortCreatedDesc = "created_desc"
)

// ListChannelsOptions configures Slack conversations.list and users.conversations
// requests, which take the same set of parameters.
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
	Count      int                   `json:"count"`
	NextCursor string                `json:"next_cursor,omitempty"`
	Sort       string                `json:"sort"`
}

// GetChannelInfoOptions configures Slack conversations.info requests.
type GetChannelInfoOptions struct {
	ChannelID         string `json:"channel_id,omitempty"`
	IncludeNumMembers bool   `json:"include_num_members,omitempty"`
	IncludeLocale     bool   `json:"include_locale,omitempty"`
}

// GetChannelInfoResponse contains the relevant conversations.info response fields.
type GetChannelInfoResponse struct {
	OK      bool                `json:"ok"`
	Channel SlackChannelSummary `json:"channel"`
}

// GetChannelInfo fetches a single channel's details through conversations.info. This
// complements ListChannels/ListJoinedChannels for callers that already have a
// channel ID and want its details without paging through the whole workspace.
func (w *webAPITransport) GetChannelInfo(ctx context.Context, opts GetChannelInfoOptions) (*GetChannelInfoResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	channelID, err := w.ResolveChannelID(opts.ChannelID)
	if err != nil {
		return nil, err
	}

	channel, err := w.slackAPIClient.GetConversationInfoContext(ctx, &slackapi.GetConversationInfoInput{
		ChannelID:         channelID,
		IncludeLocale:     opts.IncludeLocale,
		IncludeNumMembers: opts.IncludeNumMembers,
	})
	if err != nil {
		return nil, fmt.Errorf("slack: conversations.info failed: %w", err)
	}

	return &GetChannelInfoResponse{
		OK:      true,
		Channel: summarizeChannel(*channel),
	}, nil
}

// ListChannels lists Slack channel-like conversations through conversations.list.
func (w *webAPITransport) ListChannels(ctx context.Context, opts ListChannelsOptions) (*ListChannelsResponse, error) {
	return w.listChannels(ctx, "conversations.list", opts,
		func(ctx context.Context, params channelPageParams) ([]slackapi.Channel, string, error) {
			return w.slackAPIClient.GetConversationsContext(ctx, &slackapi.GetConversationsParameters{
				Cursor:          params.Cursor,
				ExcludeArchived: params.ExcludeArchived,
				Limit:           params.Limit,
				Types:           params.Types,
				TeamID:          params.TeamID,
			})
		})
}

// ListJoinedChannels lists the conversations the token owner (the calling user, for a
// user token, or the bot, for a bot token) is a member of, through users.conversations.
// Unlike ListChannels/conversations.list, this is scoped server-side to the caller's
// own memberships rather than the whole workspace, so every returned channel already
// has IsMember set.
func (w *webAPITransport) ListJoinedChannels(ctx context.Context, opts ListChannelsOptions) (*ListChannelsResponse, error) {
	return w.listChannels(ctx, "users.conversations", opts,
		func(ctx context.Context, params channelPageParams) ([]slackapi.Channel, string, error) {
			return w.slackAPIClient.GetConversationsForUserContext(ctx, &slackapi.GetConversationsForUserParameters{
				Cursor:          params.Cursor,
				ExcludeArchived: params.ExcludeArchived,
				Limit:           params.Limit,
				Types:           params.Types,
				TeamID:          params.TeamID,
			})
		})
}

// channelPageParams holds the normalized per-page request parameters shared by
// conversations.list and users.conversations.
type channelPageParams struct {
	Cursor          string
	Limit           int
	Types           []string
	TeamID          string
	ExcludeArchived bool
}

// listChannels validates opts, pages through fetch, then sorts and summarizes the
// result. conversations.list and users.conversations take identical parameters and
// return identical shapes, so they differ only in the fetch closure.
func (w *webAPITransport) listChannels(ctx context.Context, apiMethod string, opts ListChannelsOptions, fetch func(context.Context, channelPageParams) ([]slackapi.Channel, string, error)) (*ListChannelsResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}

	limit, err := normalizeListLimit(opts.Limit, defaultChannelListLimit, maxChannelListLimit)
	if err != nil {
		return nil, err
	}
	types, err := normalizeChannelTypes(opts.Types)
	if err != nil {
		return nil, err
	}
	sortBy, err := normalizeChannelSort(opts.Sort)
	if err != nil {
		return nil, err
	}
	teamID := strings.TrimSpace(opts.TeamID)

	channels, nextCursor, err := collectPages(ctx, apiMethod, limit, channelListPageSize, strings.TrimSpace(opts.Cursor),
		func(ctx context.Context, cursor string, requestLimit int) ([]SlackChannelSummary, string, error) {
			apiChannels, pageCursor, err := fetch(ctx, channelPageParams{
				Cursor:          cursor,
				Limit:           requestLimit,
				Types:           types,
				TeamID:          teamID,
				ExcludeArchived: opts.ExcludeArchived,
			})
			if err != nil {
				return nil, "", err
			}
			return summarizeChannels(apiChannels), pageCursor, nil
		}, nil)
	if err != nil {
		return nil, err
	}

	sortChannels(channels, sortBy)

	return &ListChannelsResponse{
		OK:         true,
		Channels:   channels,
		Count:      len(channels),
		NextCursor: nextCursor,
		Sort:       sortBy,
	}, nil
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

func summarizeChannels(channels []slackapi.Channel) []SlackChannelSummary {
	out := make([]SlackChannelSummary, 0, len(channels))
	for _, channel := range channels {
		out = append(out, summarizeChannel(channel))
	}
	return out
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
