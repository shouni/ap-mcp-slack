package client

import (
	"fmt"
	"net/http"
	"strings"

	slackapi "github.com/slack-go/slack"
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

// WebAPIConfigured reports whether a Web API token was configured. Tool registration
// uses it to leave the Web API tools out entirely when they could only ever fail.
func (w *webAPITransport) WebAPIConfigured() bool {
	return w.token != ""
}

// requireToken reports an error if no Web API token was configured. All Web API
// operations (post-as-user, delete, list) need one, so they share this check.
func (w *webAPITransport) requireToken() error {
	if w.token == "" {
		return fmt.Errorf("slack: token is required")
	}
	return nil
}

func (w *webAPITransport) channelIDOrDefault(channelID string) string {
	channelID = strings.TrimSpace(channelID)
	if channelID != "" {
		return channelID
	}
	return w.defaultChannelID
}

// ResolveChannelID applies MCP_SLACK_CHANNEL_ID to an omitted channelID and reports an
// error when neither yields a destination. Every channel-scoped operation starts with
// it, so they agree on what "no channel" means.
//
// It is exported so the tool layer can settle "is there a destination at all" without
// calling Slack. That question has a definite answer offline, and answering it from a
// failed lookup instead conflates it with "the destination exists but this token cannot
// read it" — which for update/delete must stay a note on the preview rather than a
// refusal to act.
func (w *webAPITransport) ResolveChannelID(channelID string) (string, error) {
	if resolved := w.channelIDOrDefault(channelID); resolved != "" {
		return resolved, nil
	}
	return "", fmt.Errorf("slack: channel_id is required")
}

func normalizeSlackAPIBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.TrimRight(raw, "/") + "/"
}
