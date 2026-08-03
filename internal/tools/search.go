package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/client"
)

// SearchSlackMessagesInput is the input for search_slack_messages.
type SearchSlackMessagesInput struct {
	Query            string `json:"query" jsonschema:"検索クエリ。Slackの検索構文がそのまま使えます: in:#channel / from:@user / before:2026-01-31 / after:2026-01-01 / during:January / has:link / \"完全一致フレーズ\"。複数語はAND、絞り込みは検索語より修飾子で行うほうが精度が上がります。"`
	Limit            int    `json:"limit,omitempty" jsonschema:"1ページあたりの最大取得件数。省略時は20、最大100です。"`
	Page             int    `json:"page,omitempty" jsonschema:"取得するページ番号。1始まりで、省略時は1、最大100です。続きは前回の応答の next_page を渡してください。"`
	Sort             string `json:"sort,omitempty" jsonschema:"並び替えの基準。score（関連度）または timestamp（投稿日時）。省略時は score です。"`
	SortDirection    string `json:"sort_dir,omitempty" jsonschema:"並び順。asc または desc。省略時は desc です。"`
	TeamID           string `json:"team_id,omitempty" jsonschema:"Enterprise Gridのorg-level tokenで対象ワークスペースを指定する場合のteam idです。"`
	IncludeRawBlocks bool   `json:"include_raw_blocks,omitempty" jsonschema:"trueの場合、Block Kit blocksとattachmentsの生データも返します。省略時はテキストとして要約されたものだけを返します。"`
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
