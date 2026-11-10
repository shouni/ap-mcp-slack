package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/client"
)

// ListSlackUsersInput is the input for list_slack_users.
type ListSlackUsersInput struct {
	Query          string `json:"query,omitempty" jsonschema:"A case-insensitive substring search query over name / real_name / display_name / email."`
	Limit          int    `json:"limit,omitempty" jsonschema:"The maximum number of items. Defaults to 200, at most 1000. Because of the remainder of what Slack returns in one page, the number returned can slightly exceed it."`
	Cursor         string `json:"cursor,omitempty" jsonschema:"The Slack pagination cursor to continue from."`
	TeamID         string `json:"team_id,omitempty" jsonschema:"The team id naming the target workspace when using an Enterprise Grid org-level token."`
	IncludeDeleted bool   `json:"include_deleted,omitempty" jsonschema:"When true, deactivated (deleted) users are included too. Excluded by default."`
}

// ListSlackUsersOutput is the structured output for list_slack_users.
type ListSlackUsersOutput = client.ListUsersResponse

// LookupSlackUserByEmailInput is the input for lookup_slack_user_by_email.
type LookupSlackUserByEmailInput struct {
	Email string `json:"email" jsonschema:"The email address to look up."`
}

// LookupSlackUserByEmailOutput is the structured output for lookup_slack_user_by_email.
type LookupSlackUserByEmailOutput struct {
	OK   bool                     `json:"ok"`
	User *client.SlackUserSummary `json:"user,omitempty"`
}

// ResolveSlackUserInput is the input for resolve_slack_user.
type ResolveSlackUserInput struct {
	Name   string `json:"name,omitempty" jsonschema:"The user name, real name or display name to look up. Ignored when email is given."`
	Email  string `json:"email,omitempty" jsonschema:"The email address to look up. Takes precedence over name when given."`
	TeamID string `json:"team_id,omitempty" jsonschema:"The team id naming the target workspace when using an Enterprise Grid org-level token. Used only when searching by name."`
}

// ResolveSlackUserOutput is the structured output for resolve_slack_user. Status is
// one of "found", "ambiguous", or "not_found"; User/Mention are set only when
// status is "found", and Candidates only when status is "ambiguous".
// search_truncated warns that the name search did not cover the whole workspace, so a
// "not_found" is "not among those searched" rather than "not in this workspace".
type ResolveSlackUserOutput = client.ResolveUserResponse

func (t *SlackTools) listSlackUsers(ctx context.Context, _ *mcp.CallToolRequest, in ListSlackUsersInput) (*mcp.CallToolResult, ListSlackUsersOutput, error) {
	out, err := t.client.ListUsers(ctx, client.ListUsersOptions{
		Limit:          in.Limit,
		Cursor:         in.Cursor,
		TeamID:         in.TeamID,
		IncludeDeleted: in.IncludeDeleted,
		Query:          in.Query,
	})
	if err != nil {
		return nil, ListSlackUsersOutput{}, err
	}

	return nil, *out, nil
}

func (t *SlackTools) lookupSlackUserByEmail(ctx context.Context, _ *mcp.CallToolRequest, in LookupSlackUserByEmailInput) (*mcp.CallToolResult, LookupSlackUserByEmailOutput, error) {
	user, err := t.client.LookupUserByEmail(ctx, in.Email)
	if err != nil {
		return nil, LookupSlackUserByEmailOutput{}, err
	}

	return nil, LookupSlackUserByEmailOutput{OK: true, User: user}, nil
}

func (t *SlackTools) resolveSlackUser(ctx context.Context, _ *mcp.CallToolRequest, in ResolveSlackUserInput) (*mcp.CallToolResult, ResolveSlackUserOutput, error) {
	out, err := t.client.ResolveUser(ctx, in.Name, in.Email, in.TeamID)
	if err != nil {
		return nil, ResolveSlackUserOutput{}, err
	}

	return nil, *out, nil
}
