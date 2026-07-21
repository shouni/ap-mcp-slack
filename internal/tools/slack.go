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

// Register registers Slack tools on the MCP server.
func (t *SlackTools) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "preview_slack_message",
		Description: "post_slack_message で送信される Slack Incoming Webhook payload を、Slackへ投稿せずに確認します。",
	}, t.previewSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message",
		Description: "MCP_SLACK_WEBHOOK_URL の Slack Incoming Webhook にメッセージを投稿します。",
	}, t.postSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "preview_slack_message_as_user",
		Description: "post_slack_message_as_user で送信される Slack Web API chat.postMessage payload を、Slackへ投稿せずに確認します。",
	}, t.previewSlackMessageAsUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message_as_user",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API chat.postMessage でメッセージを投稿します。成功時に channel_id と ts を返します。",
	}, t.postSlackMessageAsUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_slack_message",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API chat.update で投稿済みメッセージの内容を更新します。更新できるのは元の投稿者本人（同じユーザー/ボット）の投稿のみです。",
	}, t.updateSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_slack_message",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API chat.delete でメッセージを削除します。",
	}, t.deleteSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_info",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API conversations.info で単一チャンネルの詳細情報を取得します。",
	}, t.getSlackChannelInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_channels",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API conversations.list でワークスペース全体のチャンネル一覧を取得します。並び順は取得した結果にローカルで適用します。自分（トークン所有者）が参加しているチャンネルだけが欲しい場合は list_joined_slack_channels を使ってください。",
	}, t.listSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_joined_slack_channels",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API users.conversations でトークン所有者が参加しているチャンネル一覧のみを取得します（サーバー側でメンバーシップに絞り込まれます）。MCP_SLACK_USER_TOKEN（ユーザートークン）を設定している場合はそのユーザー本人が参加しているチャンネル、ボットトークンのみの場合はそのボットが参加しているチャンネルが対象です。",
	}, t.listJoinedSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_history",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API conversations.history でチャンネルのメッセージ履歴を取得します。public channel は channels:history、private channel は groups:history スコープが必要です。",
	}, t.getSlackChannelHistory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_thread_replies",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API conversations.replies で指定メッセージのスレッド返信を取得します。public channel は channels:history、private channel は groups:history スコープが必要です。",
	}, t.getSlackThreadReplies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_users",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API users.list でワークスペースメンバー一覧を取得します。deleted（deactivate済み）ユーザーはデフォルトで除外されます。要 users:read スコープ。",
	}, t.listSlackUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_slack_user_by_email",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API users.lookupByEmail でメールアドレスから単一ユーザーを検索します。要 users:read.email スコープ。",
	}, t.lookupSlackUserByEmail)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve_slack_user",
		Description: "name または email から Slack ユーザーを一意に解決します。email が指定された場合は users.lookupByEmail を優先し、無ければ users.list から完全一致→部分一致の順で検索します。候補が複数ある場合は自動選択せず候補一覧を返します（曖昧なまま送信しないでください）。戻り値の mention はそのまま <@U...> 形式でメッセージに埋め込めます。要 users:read（および email 指定時は users:read.email）スコープ。",
	}, t.resolveSlackUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_auth_info",
		Description: "MCP_SLACK_USER_TOKEN または MCP_SLACK_TOKEN を使い、Slack Web API auth.test で現在設定されているトークンの認証情報（team, user, bot_id など）を確認します。OAuthスコープは不要です。",
	}, t.getSlackAuthInfo)
}
