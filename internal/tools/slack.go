// Package tools registers MCP tools.
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

// SlackTools provides Slack-related MCP tools.
type SlackTools struct {
	client *client.SlackClient
}

// NewSlackTools creates SlackTools.
func NewSlackTools(c *client.SlackClient) *SlackTools {
	return &SlackTools{client: c}
}

// Register registers the Slack tools each configured transport can actually serve.
//
// Registration is gated rather than unconditional because an MCP client's model
// picks tools from the advertised list and has no way to know which ones the
// process was given credentials for. Advertising a webhook tool to a token-only
// deployment invites the model to choose it, build a payload, get a human to approve
// the preview, and only then fail — so the tool is simply not offered.
//
// For the same reason there are no separate preview_* tools: every mutating tool
// returns its preview when confirm is unset, so previewing is the default path through
// the tool a model already picked, not a second tool it has to know to reach for
// first. A distinct preview tool could only add a way to skip the gate.
func (t *SlackTools) Register(server *mcp.Server) {
	if t.client.WebhookConfigured() {
		t.registerWebhookTools(server)
	}
	if t.client.WebAPIConfigured() {
		t.registerWebAPITools(server)
	}
}

// registerWebhookTools registers the tools backed by MCP_SLACK_WEBHOOK_URL.
func (t *SlackTools) registerWebhookTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message",
		Description: "MCP_SLACK_WEBHOOK_URL の Slack Incoming Webhook にメッセージを投稿します。confirm=false（省略時）は投稿せず、送信される payload のプレビューのみを返します。",
	}, t.postSlackMessage)
}

// registerWebAPITools registers the tools backed by a Slack Web API token.
//
// Descriptions say what a tool does and how to choose between neighbours, and leave out
// which environment variable supplied the token. A tool is only advertised once its
// credentials are present, so naming them tells the model something it cannot act on and
// cannot choose differently because of — while costing context on every request. Setup
// and OAuth scopes belong in docs/tools.md, which the operator reads.
func (t *SlackTools) registerWebAPITools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message_as_user",
		Description: "Slackにメッセージを投稿します（chat.postMessage）。トークン所有者本人の名前で投稿されます。confirm=false（省略時）は投稿せず、チャンネル名・メンション先・スレッド元メッセージを解決したプレビューを返します。投稿時は channel_id と ts を返し、ts は update_slack_message / delete_slack_message にそのまま渡せます。",
	}, t.postSlackMessageAsUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_slack_message",
		Description: "投稿済みメッセージの内容を書き換えます（chat.update）。書き換えられるのは元の投稿者本人の投稿のみで、text/blocks/attachments は既存の内容を丸ごと置き換えます（部分更新はできません）。confirm=false（省略時）は更新せず、現在の本文と更新後の payload を並べたプレビューを返します。",
	}, t.updateSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_slack_message",
		Description: "メッセージを削除します（chat.delete）。削除は取り消せません。confirm=false（省略時）は削除せず、削除対象メッセージの内容をプレビューとして返します。",
	}, t.deleteSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_info",
		Description: "チャンネルIDから単一チャンネルの詳細を取得します（conversations.info）。IDが既に分かっている場合は、一覧をページングするより先にこれを使ってください。",
	}, t.getSlackChannelInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_channels",
		Description: "ワークスペース全体のチャンネル一覧を取得します（conversations.list）。トークン所有者が参加しているチャンネルだけが欲しい場合は list_joined_slack_channels のほうが速く、確実です。並び順は取得結果にローカルで適用します。",
	}, t.listSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_joined_slack_channels",
		Description: "トークン所有者が参加しているチャンネルのみを取得します（users.conversations）。絞り込みはSlack側で行われるため、list_slack_channels をローカルで絞り込むより効率的です。「自分が入っているチャンネル」を尋ねられた場合はこちらを使ってください。",
	}, t.listJoinedSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_history",
		Description: "チャンネルのメッセージ履歴を新しい順に取得します（conversations.history）。トップレベルのメッセージのみが対象です。スレッド内の返信を読むには get_slack_thread_replies を使ってください。",
	}, t.getSlackChannelHistory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_thread_replies",
		Description: "指定したスレッドの返信を取得します（conversations.replies）。ts にはスレッド親のtsを渡してください（get_slack_channel_history の結果の thread_ts が該当します）。",
	}, t.getSlackThreadReplies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_users",
		Description: "ワークスペースメンバーを一覧・検索します（users.list）。deactivate済みユーザーはデフォルトで除外されます。特定の1人を宛先として特定したい場合は、この一覧から選ぶのではなく resolve_slack_user を使ってください。",
	}, t.listSlackUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_slack_user_by_email",
		Description: "メールアドレスの完全一致で単一ユーザーを取得します（users.lookupByEmail）。名前で探す場合や、見つからないときに候補も知りたい場合は resolve_slack_user を使ってください。",
	}, t.lookupSlackUserByEmail)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve_slack_user",
		Description: "name または email から Slack ユーザーを一意に特定します。メンション先を決めるときはこれを使ってください。候補が複数ある場合は自動選択せず status=\"ambiguous\" と候補一覧を返すので、曖昧なまま投稿しないでください。status=\"found\" のときの mention をそのまま本文に埋め込めます。search_truncated=true の場合、not_found は「探した範囲にいなかった」という意味しかありません。",
	}, t.resolveSlackUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_auth_info",
		Description: "設定中のトークンがどのワークスペース・ユーザー・Botとして認証されるかを返します（auth.test）。OAuthスコープを必要としないため、他のツールがスコープ不足で失敗するときの切り分けに使えます。",
	}, t.getSlackAuthInfo)
}
