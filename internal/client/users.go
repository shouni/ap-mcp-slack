package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

const (
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
