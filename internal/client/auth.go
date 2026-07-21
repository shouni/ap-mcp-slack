package client

import (
	"context"
	"fmt"
)

// AuthInfoResponse contains the relevant auth.test response fields.
type AuthInfoResponse struct {
	OK           bool   `json:"ok"`
	URL          string `json:"url,omitempty"`
	Team         string `json:"team,omitempty"`
	TeamID       string `json:"team_id,omitempty"`
	User         string `json:"user,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	BotID        string `json:"bot_id,omitempty"`
	EnterpriseID string `json:"enterprise_id,omitempty"`
}

// GetAuthInfo identifies the configured Web API token through auth.test. Unlike
// every other Web API operation here, auth.test requires no OAuth scope, so it
// doubles as a lightweight way to confirm the configured token is valid and see
// which workspace/user/bot it resolves to, without needing any tool-specific scope.
func (w *webAPITransport) GetAuthInfo(ctx context.Context) (*AuthInfoResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}

	resp, err := w.slackAPIClient.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack: auth.test failed: %w", err)
	}

	return &AuthInfoResponse{
		OK:           true,
		URL:          resp.URL,
		Team:         resp.Team,
		TeamID:       resp.TeamID,
		User:         resp.User,
		UserID:       resp.UserID,
		BotID:        resp.BotID,
		EnterpriseID: resp.EnterpriseID,
	}, nil
}
