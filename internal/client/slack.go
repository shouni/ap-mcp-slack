// Package client provides outbound service clients.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
	slackapi "github.com/slack-go/slack"
)

const (
	requestTimeout = 10 * time.Second

	defaultChannelListLimit = 200
	maxChannelListLimit     = 1000
	channelListPageSize     = 200

	defaultUserListLimit = 200
	maxUserListLimit     = 1000
	userListPageSize     = 200

	// resolveUserSearchCap bounds how many workspace members ResolveUser scans when
	// resolving by name, so a single tool call can't loop unboundedly against a very
	// large workspace. Callers that need to search further should page through
	// ListUsers with query directly instead.
	resolveUserSearchCap = 5000
)

// Status values returned by resolve_slack_user.
const (
	ResolveUserStatusFound     = "found"
	ResolveUserStatusAmbiguous = "ambiguous"
	ResolveUserStatusNotFound  = "not_found"
)

// Sort values accepted by list_slack_channels' sort option.
const (
	ChannelSortNone        = "none"
	ChannelSortNameAsc     = "name_asc"
	ChannelSortNameDesc    = "name_desc"
	ChannelSortCreatedAsc  = "created_asc"
	ChannelSortCreatedDesc = "created_desc"
)

// SlackClient posts and deletes Slack messages through incoming webhooks and Web API.
// It composes a webhook transport (Message/PostMessage) and a token-authenticated Web
// API transport (WebAPIMessage/PostWebAPIMessage/DeleteWebAPIMessage/ListChannels).
// The two share no state, so they're kept as separate embedded types rather than one
// struct with fields that are only meaningful to one side or the other.
type SlackClient struct {
	webhookTransport
	webAPITransport
}

// SlackClientConfig configures SlackClient.
type SlackClientConfig struct {
	WebhookURL       string
	Token            string
	DefaultChannelID string
	APIBaseURL       string
	// SourceLabel, if set, is appended as a Block Kit context footer on every Web
	// API ("post as user") message. Incoming Webhook posts don't need it: Slack
	// already renders those under the app's own name/icon with an "APP" badge, so
	// they're already distinguishable from a message the user typed themselves. A
	// user-token chat.postMessage call has no such marker.
	SourceLabel string
}

// NewSlackClient creates a SlackClient.
func NewSlackClient(webhookURL string) *SlackClient {
	return NewSlackClientWithConfig(SlackClientConfig{WebhookURL: webhookURL})
}

// NewSlackClientWithConfig creates a SlackClient with explicit configuration.
func NewSlackClientWithConfig(cfg SlackClientConfig) *SlackClient {
	return &SlackClient{
		webhookTransport: newWebhookTransport(cfg),
		webAPITransport:  newWebAPITransport(cfg),
	}
}

// ----------------------------------------------------------------------
// Webhook transport
// ----------------------------------------------------------------------

// webhookTransport posts messages through Slack Incoming Webhooks.
//
// Response bodies are capped by go-http-kit itself at httpkit.MaxResponseBodySize
// (25MB, unconditional, not caller-configurable in v1.6.0) rather than the tighter
// 64KB this package enforced manually before adopting go-http-kit. A malicious or
// misbehaving webhook endpoint can't force unbounded memory growth, only up to that
// fixed ceiling; a real Slack incoming webhook only ever returns a few bytes.
type webhookTransport struct {
	webhookURL    string
	httpKitClient *httpkit.Client
}

func newWebhookTransport(cfg SlackClientConfig) webhookTransport {
	// Webhook posts are non-idempotent (they create a new Slack message), so retries
	// are disabled to avoid duplicate posts on transient errors. SSRF/DNS-rebinding
	// validation always stays on here; tests that need a loopback httptest server
	// build a webhookTransport literal directly rather than going through this
	// production constructor, so there's no config flag that could flip it off.
	return webhookTransport{
		webhookURL:    strings.TrimSpace(cfg.WebhookURL),
		httpKitClient: httpkit.New(requestTimeout, httpkit.WithNoRetry()),
	}
}

// Message is the JSON payload sent to Slack Incoming Webhooks.
type Message struct {
	Text        string           `json:"text,omitempty"`
	Blocks      []map[string]any `json:"blocks,omitempty"`
	Attachments []map[string]any `json:"attachments,omitempty"`
	ThreadTS    string           `json:"thread_ts,omitempty"`
	IconEmoji   string           `json:"icon_emoji,omitempty"`
	UnfurlLinks *bool            `json:"unfurl_links,omitempty"`
	UnfurlMedia *bool            `json:"unfurl_media,omitempty"`
}

// PostMessageResponse contains the relevant Slack webhook response details.
type PostMessageResponse struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

// PostMessage posts a message to Slack.
func (w *webhookTransport) PostMessage(ctx context.Context, msg Message) (*PostMessageResponse, error) {
	if w.webhookURL == "" {
		return nil, fmt.Errorf("slack: webhook URL is required")
	}
	if strings.TrimSpace(msg.Text) == "" {
		return nil, fmt.Errorf("slack: text is required")
	}

	responseBody, err := w.httpKitClient.PostJSONAndFetchBytes(ctx, w.webhookURL, msg)
	if err != nil {
		return nil, fmt.Errorf("slack: post webhook: %w", err)
	}

	// go-http-kit's PostJSONAndFetchBytes abstracts away the exact 2xx status code
	// (it only surfaces non-2xx as an error), and Slack's incoming webhooks are
	// documented to respond 200 on every accepted post, so that's what we report here.
	return &PostMessageResponse{
		StatusCode: http.StatusOK,
		Body:       strings.TrimSpace(string(responseBody)),
	}, nil
}

// ----------------------------------------------------------------------
// Web API transport
// ----------------------------------------------------------------------

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

// ListChannels lists Slack channel-like conversations through conversations.list.
func (w *webAPITransport) ListChannels(ctx context.Context, opts ListChannelsOptions) (*ListChannelsResponse, error) {
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

	cursor := strings.TrimSpace(opts.Cursor)
	teamID := strings.TrimSpace(opts.TeamID)
	channels := make([]SlackChannelSummary, 0, limit)
	seenCursors := map[string]struct{}{}

	for len(channels) < limit {
		requestLimit := min(channelListPageSize, limit-len(channels))
		apiChannels, nextCursor, err := w.slackAPIClient.GetConversationsContext(ctx, &slackapi.GetConversationsParameters{
			Cursor:          cursor,
			ExcludeArchived: opts.ExcludeArchived,
			Limit:           requestLimit,
			Types:           types,
			TeamID:          teamID,
		})
		if err != nil {
			return nil, fmt.Errorf("slack: conversations.list failed: %w", err)
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
			return nil, fmt.Errorf("slack: conversations.list returned duplicate cursor %q", nextCursor)
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

// ListJoinedChannels lists the conversations the token owner (the calling user, for a
// user token, or the bot, for a bot token) is a member of, through users.conversations.
// Unlike ListChannels/conversations.list, this is scoped server-side to the caller's
// own memberships rather than the whole workspace, so every returned channel already
// has IsMember set.
func (w *webAPITransport) ListJoinedChannels(ctx context.Context, opts ListJoinedChannelsOptions) (*ListChannelsResponse, error) {
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

	cursor := strings.TrimSpace(opts.Cursor)
	teamID := strings.TrimSpace(opts.TeamID)
	channels := make([]SlackChannelSummary, 0, limit)
	seenCursors := map[string]struct{}{}

	for len(channels) < limit {
		requestLimit := min(channelListPageSize, limit-len(channels))
		apiChannels, nextCursor, err := w.slackAPIClient.GetConversationsForUserContext(ctx, &slackapi.GetConversationsForUserParameters{
			Cursor:          cursor,
			ExcludeArchived: opts.ExcludeArchived,
			Limit:           requestLimit,
			Types:           types,
			TeamID:          teamID,
		})
		if err != nil {
			return nil, fmt.Errorf("slack: users.conversations failed: %w", err)
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
			return nil, fmt.Errorf("slack: users.conversations returned duplicate cursor %q", nextCursor)
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

// ----------------------------------------------------------------------
// Users
// ----------------------------------------------------------------------

// ListUsersOptions configures Slack users.list requests.
type ListUsersOptions struct {
	Limit          int    `json:"limit,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
	TeamID         string `json:"team_id,omitempty"`
	IncludeDeleted bool   `json:"include_deleted,omitempty"`
	Query          string `json:"query,omitempty"`
}

// SlackUserSummary contains the user fields returned by the user-lookup tools.
type SlackUserSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	RealName    string `json:"real_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Deleted     bool   `json:"deleted,omitempty"`
	IsBot       bool   `json:"is_bot,omitempty"`
}

// ListUsersResponse contains the relevant users.list response fields.
type ListUsersResponse struct {
	OK         bool               `json:"ok"`
	Users      []SlackUserSummary `json:"users"`
	Count      int                `json:"count"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

// ResolveUserResponse is the result of resolving a Slack user by name or email.
type ResolveUserResponse struct {
	OK         bool               `json:"ok"`
	Status     string             `json:"status"`
	User       *SlackUserSummary  `json:"user,omitempty"`
	Mention    string             `json:"mention,omitempty"`
	Candidates []SlackUserSummary `json:"candidates,omitempty"`
}

// ListUsers lists Slack workspace members through users.list. Deleted (deactivated)
// users are excluded by default: callers use this to find people to message, not to
// audit historical accounts.
func (w *webAPITransport) ListUsers(ctx context.Context, opts ListUsersOptions) (*ListUsersResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}

	limit, err := normalizeListLimit(opts.Limit, defaultUserListLimit, maxUserListLimit)
	if err != nil {
		return nil, err
	}

	query := strings.ToLower(strings.TrimSpace(opts.Query))
	pagination := w.slackAPIClient.GetUsersPaginated(
		slackapi.GetUsersOptionCursor(strings.TrimSpace(opts.Cursor)),
		slackapi.GetUsersOptionLimit(userListPageSize),
		slackapi.GetUsersOptionTeamID(strings.TrimSpace(opts.TeamID)),
	)

	users := make([]SlackUserSummary, 0, limit)
	seenCursors := map[string]struct{}{}
	nextCursor := ""

	for len(users) < limit {
		pagination, err = pagination.Next(ctx)
		if err != nil {
			return nil, fmt.Errorf("slack: users.list failed: %w", err)
		}

		for _, apiUser := range pagination.Users {
			if len(users) >= limit {
				break
			}
			summary := summarizeUser(apiUser)
			if !opts.IncludeDeleted && summary.Deleted {
				continue
			}
			if query != "" && !userMatchesQuery(summary, query) {
				continue
			}
			users = append(users, summary)
		}

		nextCursor = strings.TrimSpace(pagination.Cursor)
		if nextCursor == "" {
			break
		}
		if _, ok := seenCursors[nextCursor]; ok {
			return nil, fmt.Errorf("slack: users.list returned duplicate cursor %q", nextCursor)
		}
		seenCursors[nextCursor] = struct{}{}
	}

	return &ListUsersResponse{
		OK:         true,
		Users:      users,
		Count:      len(users),
		NextCursor: nextCursor,
	}, nil
}

// LookupUserByEmail resolves a single Slack user by exact email address through
// users.lookupByEmail.
func (w *webAPITransport) LookupUserByEmail(ctx context.Context, email string) (*SlackUserSummary, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, fmt.Errorf("slack: email is required")
	}

	apiUser, err := w.slackAPIClient.GetUserByEmailContext(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("slack: users.lookupByEmail failed: %w", err)
	}

	summary := summarizeUser(*apiUser)
	return &summary, nil
}

// ResolveUser resolves a Slack user by email (preferred, exact match via
// users.lookupByEmail) or by user/real/display name (via users.list). Name
// resolution prefers an exact case-insensitive match; if none exists it falls back
// to substring matches. A single match is returned as "found"; zero or multiple
// matches are reported as "not_found"/"ambiguous" rather than guessing, since
// callers use this to avoid mis-sending messages to the wrong person.
func (w *webAPITransport) ResolveUser(ctx context.Context, name, email, teamID string) (*ResolveUserResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" && email == "" {
		return nil, fmt.Errorf("slack: name or email is required")
	}

	if email != "" {
		user, err := w.LookupUserByEmail(ctx, email)
		if err != nil {
			if isUserNotFoundError(err) {
				return &ResolveUserResponse{OK: true, Status: ResolveUserStatusNotFound}, nil
			}
			return nil, err
		}
		return resolvedUserResponse(*user), nil
	}

	users, err := w.collectActiveUsers(ctx, teamID)
	if err != nil {
		return nil, err
	}

	lowerName := strings.ToLower(name)
	var candidates []SlackUserSummary
	for _, user := range users {
		if strings.ToLower(user.Name) == lowerName ||
			strings.ToLower(user.RealName) == lowerName ||
			strings.ToLower(user.DisplayName) == lowerName {
			candidates = append(candidates, user)
		}
	}
	if len(candidates) == 0 {
		for _, user := range users {
			if userMatchesQuery(user, lowerName) {
				candidates = append(candidates, user)
			}
		}
	}

	switch len(candidates) {
	case 0:
		return &ResolveUserResponse{OK: true, Status: ResolveUserStatusNotFound}, nil
	case 1:
		return resolvedUserResponse(candidates[0]), nil
	default:
		return &ResolveUserResponse{OK: true, Status: ResolveUserStatusAmbiguous, Candidates: candidates}, nil
	}
}

// collectActiveUsers pages through users.list, excluding deleted users, up to
// resolveUserSearchCap members.
func (w *webAPITransport) collectActiveUsers(ctx context.Context, teamID string) ([]SlackUserSummary, error) {
	pagination := w.slackAPIClient.GetUsersPaginated(
		slackapi.GetUsersOptionLimit(userListPageSize),
		slackapi.GetUsersOptionTeamID(strings.TrimSpace(teamID)),
	)

	users := make([]SlackUserSummary, 0, userListPageSize)
	seenCursors := map[string]struct{}{}

	for len(users) < resolveUserSearchCap {
		var err error
		pagination, err = pagination.Next(ctx)
		if err != nil {
			return nil, fmt.Errorf("slack: users.list failed: %w", err)
		}

		for _, apiUser := range pagination.Users {
			if apiUser.Deleted {
				continue
			}
			users = append(users, summarizeUser(apiUser))
		}

		nextCursor := strings.TrimSpace(pagination.Cursor)
		if nextCursor == "" {
			break
		}
		if _, ok := seenCursors[nextCursor]; ok {
			return nil, fmt.Errorf("slack: users.list returned duplicate cursor %q", nextCursor)
		}
		seenCursors[nextCursor] = struct{}{}
	}

	return users, nil
}

func summarizeUser(user slackapi.User) SlackUserSummary {
	return SlackUserSummary{
		ID:          user.ID,
		Name:        user.Name,
		RealName:    user.RealName,
		DisplayName: user.Profile.DisplayName,
		Email:       user.Profile.Email,
		Deleted:     user.Deleted,
		IsBot:       user.IsBot,
	}
}

// userMatchesQuery reports whether query (already lowercased) is a substring of
// user's name, real name, display name, or email.
func userMatchesQuery(user SlackUserSummary, query string) bool {
	for _, field := range []string{user.Name, user.RealName, user.DisplayName, user.Email} {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func resolvedUserResponse(user SlackUserSummary) *ResolveUserResponse {
	return &ResolveUserResponse{
		OK:      true,
		Status:  ResolveUserStatusFound,
		User:    &user,
		Mention: mentionString(user.ID),
	}
}

func mentionString(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}

// isUserNotFoundError reports whether err is Slack's users_not_found error, as
// returned by users.lookupByEmail when no user has the given email address.
func isUserNotFoundError(err error) bool {
	slackErr, ok := errors.AsType[slackapi.SlackErrorResponse](err)
	return ok && slackErr.Err == "users_not_found"
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

func normalizeListLimit(limit, defaultLimit, maxLimit int) (int, error) {
	if limit == 0 {
		return defaultLimit, nil
	}
	if limit < 0 {
		return 0, fmt.Errorf("slack: limit must be greater than 0")
	}
	if limit > maxLimit {
		return 0, fmt.Errorf("slack: limit must be %d or less", maxLimit)
	}
	return limit, nil
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
