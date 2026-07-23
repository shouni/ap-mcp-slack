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

func normalizeSlackAPIBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.TrimRight(raw, "/") + "/"
}
