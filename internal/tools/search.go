package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/client"
)

// SearchSlackMessagesInput is the input for search_slack_messages.
type SearchSlackMessagesInput struct {
	Query            string `json:"query" jsonschema:"The search query. Slack's search syntax works as is: in:#channel / from:@user / before:2026-01-31 / after:2026-01-01 / during:January / has:link / \"exact phrase\". Multiple terms are ANDed; narrowing with modifiers is more precise than adding search terms."`
	Limit            int    `json:"limit,omitempty" jsonschema:"The maximum number of items per page. Defaults to 20, at most 100."`
	Page             int    `json:"page,omitempty" jsonschema:"The page number to fetch. 1-based; defaults to 1, at most 100. To continue, pass the next_page of the previous response."`
	Sort             string `json:"sort,omitempty" jsonschema:"The sort key: score (relevance) or timestamp (posting time). Defaults to score."`
	SortDirection    string `json:"sort_dir,omitempty" jsonschema:"The sort direction: asc or desc. Defaults to desc."`
	TeamID           string `json:"team_id,omitempty" jsonschema:"The team id naming the target workspace when using an Enterprise Grid org-level token."`
	IncludeRawBlocks bool   `json:"include_raw_blocks,omitempty" jsonschema:"When true, the raw Block Kit blocks and attachments are returned too. By default only the text summary is returned."`
}

// SearchSlackMessagesOutput is the structured output for search_slack_messages.
type SearchSlackMessagesOutput = client.SearchMessagesResponse

func (t *SlackTools) searchSlackMessages(ctx context.Context, _ *mcp.CallToolRequest, in SearchSlackMessagesInput) (*mcp.CallToolResult, SearchSlackMessagesOutput, error) {
	out, err := t.client.SearchMessages(ctx, client.SearchMessagesOptions{
		Query:            in.Query,
		Limit:            in.Limit,
		Page:             in.Page,
		Sort:             in.Sort,
		SortDirection:    in.SortDirection,
		TeamID:           in.TeamID,
		IncludeRawBlocks: in.IncludeRawBlocks,
	})
	if err != nil {
		return nil, SearchSlackMessagesOutput{}, err
	}

	return nil, *out, nil
}
