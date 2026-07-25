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
	//
	// Derived from collectPages' own page bound so the two cannot disagree: stating it
	// as a literal would let whichever bound happened to be tighter silently override
	// the other, and ResolveUser's SearchTruncated would then describe the wrong one.
	resolveUserSearchCap = maxListPages * userListPageSize
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
	// SearchTruncated reports that the name search stopped at resolveUserSearchCap
	// members without scanning the whole workspace, so a "not_found" or "ambiguous"
	// answer describes only the part that was searched. Without this a caller cannot
	// tell "this person is not in the workspace" from "we gave up looking".
	SearchTruncated bool `json:"search_truncated,omitempty"`
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
	users, nextCursor, err := collectPages(ctx, "users.list", limit, userListPageSize, strings.TrimSpace(opts.Cursor),
		w.fetchUserPage(strings.TrimSpace(opts.TeamID)),
		func(summary SlackUserSummary) bool {
			if !opts.IncludeDeleted && summary.Deleted {
				return false
			}
			return query == "" || userMatchesQuery(summary, query)
		})
	if err != nil {
		return nil, err
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

	users, truncated, err := w.collectActiveUsers(ctx, teamID)
	if err != nil {
		return nil, err
	}

	var candidates []SlackUserSummary
	for _, user := range users {
		if userNameEquals(user, name) {
			candidates = append(candidates, user)
		}
	}
	if len(candidates) == 0 {
		lowerName := strings.ToLower(name)
		for _, user := range users {
			if userMatchesQuery(user, lowerName) {
				candidates = append(candidates, user)
			}
		}
	}

	switch len(candidates) {
	case 0:
		return &ResolveUserResponse{OK: true, Status: ResolveUserStatusNotFound, SearchTruncated: truncated}, nil
	case 1:
		// An exact single hit is trustworthy even from a truncated scan, but a single
		// substring hit is not: an unscanned member could match just as well, which
		// would have made this ambiguous rather than found.
		resolved := resolvedUserResponse(candidates[0])
		if truncated && !userNameEquals(candidates[0], name) {
			resolved.SearchTruncated = true
		}
		return resolved, nil
	default:
		return &ResolveUserResponse{OK: true, Status: ResolveUserStatusAmbiguous, Candidates: candidates, SearchTruncated: truncated}, nil
	}
}

// userNameEquals reports whether name matches any of user's name fields, ignoring case.
//
// EqualFold rather than lowercasing both sides: this runs once per scanned member (up to
// resolveUserSearchCap of them), and ToLower allocates a new string for every field that
// isn't already lower-case, which is most real names. EqualFold compares in place.
func userNameEquals(user SlackUserSummary, name string) bool {
	return strings.EqualFold(user.Name, name) ||
		strings.EqualFold(user.RealName, name) ||
		strings.EqualFold(user.DisplayName, name)
}

// collectActiveUsers pages through users.list, excluding deleted users, up to
// resolveUserSearchCap members. The second result reports whether the cap was reached
// with more pages still outstanding, i.e. whether the workspace was scanned in full.
func (w *webAPITransport) collectActiveUsers(ctx context.Context, teamID string) ([]SlackUserSummary, bool, error) {
	users, nextCursor, err := collectPages(ctx, "users.list", resolveUserSearchCap, userListPageSize, "",
		w.fetchUserPage(strings.TrimSpace(teamID)),
		func(summary SlackUserSummary) bool {
			return !summary.Deleted
		})
	if err != nil {
		return nil, false, err
	}
	return users, nextCursor != "", nil
}

// fetchUserPage returns a pageFetcher over users.list for teamID. A fresh
// UserPagination is built per page because slack-go fixes the per-request limit at
// construction time, and collectPages narrows that limit to the outstanding shortfall
// as it goes; the cursor is the only state that has to carry across pages.
func (w *webAPITransport) fetchUserPage(teamID string) pageFetcher[SlackUserSummary] {
	return func(ctx context.Context, cursor string, requestLimit int) ([]SlackUserSummary, string, error) {
		page, err := w.slackAPIClient.GetUsersPaginated(
			slackapi.GetUsersOptionCursor(cursor),
			slackapi.GetUsersOptionLimit(requestLimit),
			slackapi.GetUsersOptionTeamID(teamID),
		).Next(ctx)
		if err != nil {
			return nil, "", err
		}

		users := make([]SlackUserSummary, 0, len(page.Users))
		for _, apiUser := range page.Users {
			users = append(users, summarizeUser(apiUser))
		}
		return users, page.Cursor, nil
	}
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

// ResolvedMention is a Slack user resolved by ID, for showing a human-readable
// mention target (real/display name) in message previews instead of a bare <@ID> tag.
type ResolvedMention struct {
	SlackUserSummary
	Mention string `json:"mention"`
}

// ResolveMentions looks up each of userIDs via users.info. It is used by
// post_slack_message_as_user's preview to show who a message's explicit mentions field
// will actually notify, since the raw <@ID> tags embedded in the payload aren't
// human-readable on their own.
func (w *webAPITransport) ResolveMentions(ctx context.Context, userIDs []string) ([]ResolvedMention, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}

	mentions := make([]ResolvedMention, 0, len(userIDs))
	for _, id := range userIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		apiUser, err := w.slackAPIClient.GetUserInfoContext(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("slack: users.info failed for %q: %w", id, err)
		}
		mentions = append(mentions, ResolvedMention{
			SlackUserSummary: summarizeUser(*apiUser),
			Mention:          mentionString(id),
		})
	}
	return mentions, nil
}

// isUserNotFoundError reports whether err is Slack's users_not_found error, as
// returned by users.lookupByEmail when no user has the given email address.
func isUserNotFoundError(err error) bool {
	slackErr, ok := errors.AsType[slackapi.SlackErrorResponse](err)
	return ok && slackErr.Err == "users_not_found"
}
