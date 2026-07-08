// Package client provides outbound service clients.
package client

import (
	"context"
	"encoding/json"
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
)

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

// ListChannels lists Slack channel-like conversations through conversations.list.
func (w *webAPITransport) ListChannels(ctx context.Context, opts ListChannelsOptions) (*ListChannelsResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}

	limit, err := normalizeChannelListLimit(opts.Limit)
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
			if len(channels) >= limit {
				break
			}
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

func normalizeChannelListLimit(limit int) (int, error) {
	if limit == 0 {
		return defaultChannelListLimit, nil
	}
	if limit < 0 {
		return 0, fmt.Errorf("slack: limit must be greater than 0")
	}
	if limit > maxChannelListLimit {
		return 0, fmt.Errorf("slack: limit must be %d or less", maxChannelListLimit)
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
		for _, part := range strings.Split(rawType, ",") {
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

func buildPostMessageOptions(msg WebAPIMessage) ([]slackapi.MsgOption, error) {
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
