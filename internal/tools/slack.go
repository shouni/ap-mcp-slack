// Package tools registers MCP tools.
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/client"
)

// ServerInstructions is advertised to the MCP client once, at initialize time, and
// typically lands in the model's system context. It carries only what holds across
// the whole tool surface — the confirm gate's two-step flow, the default channel,
// and how to continue paginated results — so the per-tool descriptions can stay
// about choosing between tools. Like those descriptions, it names no environment
// variables: configuration is the operator's side of the contract, not the model's.
const ServerInstructions = `Slackへ書き込むツール（投稿・更新・削除）は、confirm を省略すると実行されず、実際に送信・変更される内容のプレビューだけを返します。まず confirm なしで呼び出し、プレビューを利用者に確認してもらってから、同じ入力に confirm=true を付けて再実行してください。プレビューの payload は確定時に送信されるものと完全に一致します。channel_id を省略した場合はサーバーに設定された既定チャンネルが使われ、未設定ならエラーになります。一覧・履歴系ツールは next_cursor（横断検索は next_page）が返ったときだけ続きがあり、retry_after_seconds が付いている場合はその秒数待ってから続きを取得してください。`

// SlackTools provides Slack-related MCP tools.
type SlackTools struct {
	client *client.SlackClient
}

// NewSlackTools creates SlackTools.
func NewSlackTools(c *client.SlackClient) *SlackTools {
	return &SlackTools{client: c}
}

// The annotation constructors below cover the three behaviour classes the tools fall
// into, so every registration declares which class it is in rather than repeating
// hint fields. OpenWorldHint is false throughout: every tool talks to the one Slack
// workspace the server was configured for, never to an open set of external systems.
// Each call returns a fresh value so no two tools alias the same hint storage.

// readOnlyAnnotations marks a tool that never writes to Slack. Clients use
// ReadOnlyHint to skip confirmation UI, so it must only ever go on tools where that
// is safe by construction.
func readOnlyAnnotations() *mcp.ToolAnnotations {
	closedWorld := false
	return &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld}
}

// additiveWriteAnnotations marks a tool that creates new content but never destroys
// existing content (the post tools). Not idempotent: each confirmed call posts
// another message.
func additiveWriteAnnotations() *mcp.ToolAnnotations {
	additive := false
	closedWorld := false
	return &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &closedWorld}
}

// destructiveWriteAnnotations marks a tool that irreversibly replaces or removes
// existing content (update/delete). Idempotent: repeating the same confirmed call
// leaves Slack exactly as the first call did — a repeated update rewrites the same
// content, a repeated delete finds the message already gone.
func destructiveWriteAnnotations() *mcp.ToolAnnotations {
	destructive := true
	closedWorld := false
	return &mcp.ToolAnnotations{DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &closedWorld}
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
		Title:       "Slack投稿（Webhook）",
		Description: "MCP_SLACK_WEBHOOK_URL の Slack Incoming Webhook にメッセージを投稿します。confirm=false（省略時）は投稿せず、送信される payload のプレビューのみを返します。",
		Annotations: additiveWriteAnnotations(),
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
		Title:       "Slack投稿（本人名義）",
		Description: "Slackにメッセージを投稿します（chat.postMessage）。トークン所有者本人の名前で投稿されます。confirm=false（省略時）は投稿せず、チャンネル名・メンション先・スレッド元メッセージを解決したプレビューを返します。投稿時は channel_id と ts を返し、ts は update_slack_message / delete_slack_message にそのまま渡せます。",
		Annotations: additiveWriteAnnotations(),
	}, t.postSlackMessageAsUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_slack_message",
		Title:       "Slackメッセージ更新",
		Description: "投稿済みメッセージの内容を書き換えます（chat.update）。書き換えられるのは元の投稿者本人の投稿のみで、text/blocks/attachments は既存の内容を丸ごと置き換えます（部分更新はできません）。confirm=false（省略時）は更新せず、現在の本文と更新後の payload を並べたプレビューを返します。",
		Annotations: destructiveWriteAnnotations(),
	}, t.updateSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_slack_message",
		Title:       "Slackメッセージ削除",
		Description: "メッセージを削除します（chat.delete）。削除は取り消せません。confirm=false（省略時）は削除せず、削除対象メッセージの内容をプレビューとして返します。",
		Annotations: destructiveWriteAnnotations(),
	}, t.deleteSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_info",
		Title:       "チャンネル詳細取得",
		Description: "チャンネルIDから単一チャンネルの詳細を取得します（conversations.info）。IDが既に分かっている場合は、一覧をページングするより先にこれを使ってください。",
		Annotations: readOnlyAnnotations(),
	}, t.getSlackChannelInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_channels",
		Title:       "チャンネル一覧",
		Description: "ワークスペース全体のチャンネル一覧を取得します（conversations.list）。トークン所有者が参加しているチャンネルだけが欲しい場合は list_joined_slack_channels のほうが速く、確実です。並び順は取得結果にローカルで適用します。",
		Annotations: readOnlyAnnotations(),
	}, t.listSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_joined_slack_channels",
		Title:       "参加チャンネル一覧",
		Description: "トークン所有者が参加しているチャンネルのみを取得します（users.conversations）。絞り込みはSlack側で行われるため、list_slack_channels をローカルで絞り込むより効率的です。「自分が入っているチャンネル」を尋ねられた場合はこちらを使ってください。",
		Annotations: readOnlyAnnotations(),
	}, t.listJoinedSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_history",
		Title:       "チャンネル履歴取得",
		Description: "チャンネルのメッセージ履歴を新しい順に取得します（conversations.history）。トップレベルのメッセージのみが対象です。スレッド内の返信を読むには get_slack_thread_replies を使ってください。",
		Annotations: readOnlyAnnotations(),
	}, t.getSlackChannelHistory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_thread_replies",
		Title:       "スレッド返信取得",
		Description: "指定したスレッドの返信を取得します（conversations.replies）。ts にはスレッド親のtsを渡してください（get_slack_channel_history の結果の thread_ts が該当します）。",
		Annotations: readOnlyAnnotations(),
	}, t.getSlackThreadReplies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_slack_messages",
		Title:       "メッセージ横断検索",
		Description: "ワークスペース全体をSlackの検索インデックスで横断全文検索します（search.messages）。チャンネルを指定せずに「どこかで話題に出たはず」を探す場合はこれを使ってください。チャンネルが分かっていて時系列に読みたい場合は get_slack_channel_history のほうが適しています。query にはSlackの検索構文（in: / from: / before: / after: / has:）がそのまま使えます。結果は関連度順で1ページずつ返り、続きは next_page を page に渡します。",
		Annotations: readOnlyAnnotations(),
	}, t.searchSlackMessages)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_users",
		Title:       "ユーザー一覧・検索",
		Description: "ワークスペースメンバーを一覧・検索します（users.list）。deactivate済みユーザーはデフォルトで除外されます。特定の1人を宛先として特定したい場合は、この一覧から選ぶのではなく resolve_slack_user を使ってください。",
		Annotations: readOnlyAnnotations(),
	}, t.listSlackUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_slack_user_by_email",
		Title:       "メールでユーザー特定",
		Description: "メールアドレスの完全一致で単一ユーザーを取得します（users.lookupByEmail）。名前で探す場合や、見つからないときに候補も知りたい場合は resolve_slack_user を使ってください。",
		Annotations: readOnlyAnnotations(),
	}, t.lookupSlackUserByEmail)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve_slack_user",
		Title:       "ユーザー解決",
		Description: "name または email から Slack ユーザーを一意に特定します。メンション先を決めるときはこれを使ってください。候補が複数ある場合は自動選択せず status=\"ambiguous\" と候補一覧を返すので、曖昧なまま投稿しないでください。status=\"found\" のときの mention をそのまま本文に埋め込めます。search_truncated=true の場合、not_found は「探した範囲にいなかった」という意味しかありません。",
		Annotations: readOnlyAnnotations(),
	}, t.resolveSlackUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_auth_info",
		Title:       "認証情報確認",
		Description: "設定中のトークンがどのワークスペース・ユーザー・Botとして認証されるかを返します（auth.test）。OAuthスコープを必要としないため、他のツールがスコープ不足で失敗するときの切り分けに使えます。",
		Annotations: readOnlyAnnotations(),
	}, t.getSlackAuthInfo)
}
