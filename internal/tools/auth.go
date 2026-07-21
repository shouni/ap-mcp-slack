package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

// GetSlackAuthInfoInput is the input for get_slack_auth_info. It takes no fields.
type GetSlackAuthInfoInput struct{}

// GetSlackAuthInfoOutput is the structured output for get_slack_auth_info.
type GetSlackAuthInfoOutput = client.AuthInfoResponse

func (t *SlackTools) getSlackAuthInfo(ctx context.Context, _ *mcp.CallToolRequest, _ GetSlackAuthInfoInput) (*mcp.CallToolResult, GetSlackAuthInfoOutput, error) {
	out, err := t.client.GetAuthInfo(ctx)
	if err != nil {
		return nil, GetSlackAuthInfoOutput{}, err
	}

	return nil, *out, nil
}
