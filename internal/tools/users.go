package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

// ListSlackUsersInput is the input for list_slack_users.
type ListSlackUsersInput struct {
	Query          string `json:"query,omitempty" jsonschema:"name / real_name / display_name / email に対する部分一致検索クエリ（大文字小文字を区別しません）。"`
	Limit          int    `json:"limit,omitempty" jsonschema:"最大取得件数。省略時は200、最大1000です。"`
	Cursor         string `json:"cursor,omitempty" jsonschema:"続きから取得する場合のSlack pagination cursorです。"`
	TeamID         string `json:"team_id,omitempty" jsonschema:"Enterprise Gridのorg-level tokenで対象ワークスペースを指定する場合のteam idです。"`
	IncludeDeleted bool   `json:"include_deleted,omitempty" jsonschema:"trueの場合、deactivate済み(deleted)ユーザーも含めます。省略時は除外されます。"`
}

// ListSlackUsersOutput is the structured output for list_slack_users.
type ListSlackUsersOutput = client.ListUsersResponse

// LookupSlackUserByEmailInput is the input for lookup_slack_user_by_email.
type LookupSlackUserByEmailInput struct {
	Email string `json:"email" jsonschema:"検索対象のメールアドレス。"`
}

// LookupSlackUserByEmailOutput is the structured output for lookup_slack_user_by_email.
type LookupSlackUserByEmailOutput struct {
	OK   bool                     `json:"ok"`
	User *client.SlackUserSummary `json:"user,omitempty"`
}

// ResolveSlackUserInput is the input for resolve_slack_user.
type ResolveSlackUserInput struct {
	Name   string `json:"name,omitempty" jsonschema:"検索対象のユーザー名・real name・display nameのいずれか。emailが指定された場合は無視されます。"`
	Email  string `json:"email,omitempty" jsonschema:"検索対象のメールアドレス。指定された場合はnameより優先されます。"`
	TeamID string `json:"team_id,omitempty" jsonschema:"Enterprise Gridのorg-level tokenで対象ワークスペースを指定する場合のteam idです。nameでの検索時のみ利用します。"`
}

// ResolveSlackUserOutput is the structured output for resolve_slack_user. Status is
// one of "found", "ambiguous", or "not_found"; User/Mention are set only when
// status is "found", and Candidates only when status is "ambiguous".
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
