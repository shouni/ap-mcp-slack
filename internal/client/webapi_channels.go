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
	channelID := w.channelIDOrDefault(opts.ChannelID)
	if channelID == "" {
		return nil, fmt.Errorf("slack: channel_id is required")
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
